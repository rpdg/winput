package screen

import (
	"errors"
	"fmt"
	"image"
	"math"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/rpdg/winput/window"
)

// GDI constants.
const (
	SRCCOPY        = 0x00CC0020
	DIB_RGB_COLORS = 0
	BI_RGB         = 0

	// GetSystemMetrics constants.
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79
)

// dpiAwarenessPerMonitor is DPI_AWARENESS_PER_MONITOR_AWARE.
// GetAwarenessFromDpiAwarenessContext returns this for both per-monitor V1 and V2.
const dpiAwarenessPerMonitor = 2

// defaultCaptureMemoryMB is the peak budget used when MaxMemoryMB is not positive.
// Peak counts the GDI DIB and the Go image (2 × width × height × 4).
const defaultCaptureMemoryMB = 1000

const mib = 1024 * 1024

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

// CaptureOptions configures one capture.
//
// MaxMemoryMB is the peak byte budget of that capture: the GDI DIB plus the
// Go image, 2 × width × height × 4. The unit is mebibytes (1024 × 1024).
// Zero and negative values select the default, 1000. The default pixel count
// matches the previous single-buffer default of 500. An explicit positive
// value allows half as many pixels as the same number did then.
//
// A capture locks the calling goroutine to one OS thread from the DPI check
// through GDI release. GdiFlush reports the calling thread's whole batch, so
// a flush failure fails the capture even when this BitBlt was not the only
// call in that batch. The implementation discards any batch already queued on
// the thread before it starts, and does not change the thread's DPI awareness.
//
// Session 0, a service process, a locked workstation, the secure desktop
// (UAC), and a minimized RDP session are not supported. Capture may return an
// error, or it may return an all-black image and a nil error. Black images
// are not detected.
//
// If the pixels were copied and a later GDI release fails, the image is
// returned together with the cleanup error. The image is usable.
type CaptureOptions struct {
	// Deprecated: PreserveAlpha has no effect. BitBlt does not provide a
	// meaningful alpha channel, and image.RGBA is alpha-premultiplied.
	// Every captured pixel is written with alpha 255.
	PreserveAlpha bool

	// MaxMemoryMB is the peak memory limit in mebibytes (DIB plus Go image).
	// Zero means 1000.
	MaxMemoryMB int
}

var defaultOptions = CaptureOptions{
	PreserveAlpha: false,
	MaxMemoryMB:   defaultCaptureMemoryMB,
}

// CaptureVirtualDesktop captures the entire virtual desktop (all monitors).
//
// The returned image has origin (0, 0) at the top-left of the virtual desktop.
// Pass that image to ImageToVirtual. A CaptureRegion image does not use the
// same origin.
//
// The calling thread must be per-monitor DPI aware (V1 or V2). Call
// winput.EnablePerMonitorDPI at process startup. If this thread was later
// switched to another DPI awareness context, capture returns an error until
// that context is restored. When the thread DPI APIs are missing, the check
// falls back to the process. See CaptureOptions for the environments that are
// not supported, the peak memory budget, and the cleanup-error return.
func CaptureVirtualDesktop() (*image.RGBA, error) {
	return CaptureVirtualDesktopWithOptions(defaultOptions)
}

// CaptureVirtualDesktopWithOptions captures the virtual desktop with opts.
// See CaptureVirtualDesktop and CaptureOptions.
func CaptureVirtualDesktopWithOptions(opts CaptureOptions) (*image.RGBA, error) {
	return captureRect(true, 0, 0, 0, 0, opts)
}

// CaptureRegion captures the intersection of the requested rectangle with the
// virtual desktop. It does not capture the whole desktop first.
//
// x and y are virtual-desktop coordinates and may be negative. w and h are the
// requested pixel size; either value <= 0 returns an error before any DPI or
// GDI call. A request that extends past the virtual desktop returns the
// intersection. The image Rect starts at (0, 0), and that pixel is the
// top-left of the intersection, not of the virtual desktop. Do not pass the
// result to ImageToVirtual.
//
// CaptureRegion uses the default CaptureOptions. See CaptureOptions for DPI,
// unsupported environments, peak memory, and cleanup errors.
func CaptureRegion(x, y, w, h int32) (*image.RGBA, error) {
	return CaptureRegionWithOptions(x, y, w, h, defaultOptions)
}

// CaptureRegionWithOptions captures the intersection of the requested
// rectangle with the virtual desktop using opts. See CaptureRegion.
func CaptureRegionWithOptions(x, y, w, h int32, opts CaptureOptions) (*image.RGBA, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid region size: %dx%d", w, h)
	}
	return captureRect(false, int64(x), int64(y), int64(w), int64(h), opts)
}

// apiError formats a failed Win32 call. A non-zero syscall.Errno is wrapped
// so errors.Is can match it. A zero errno, or any other error, is not wrapped:
// LazyProc.Call surfaces GetLastError even when the call did not set it.
func apiError(name string, e error) error {
	if errno, ok := e.(syscall.Errno); ok && errno != 0 {
		return fmt.Errorf("%s failed: %d: %w", name, uint32(errno), errno)
	}
	return fmt.Errorf("%s failed", name)
}

func captureLimitMB(limitMB int) int {
	if limitMB <= 0 {
		return defaultCaptureMemoryMB
	}
	return limitMB
}

func captureLimitBytes(limitMB int) (int64, error) {
	mb := int64(captureLimitMB(limitMB))
	if mb > math.MaxInt64/mib {
		return 0, fmt.Errorf("invalid capture memory limit: %d MB", limitMB)
	}
	return mb * mib, nil
}

func capturePeakBytes(w, h int64) (int64, error) {
	if w <= 0 || h <= 0 {
		return 0, fmt.Errorf("invalid screen dimensions: %dx%d", w, h)
	}
	if w > math.MaxInt64/h {
		return 0, fmt.Errorf("resolution too large: %dx%d", w, h)
	}
	wh := w * h
	if wh > math.MaxInt64/8 {
		return 0, fmt.Errorf("resolution too large: %dx%d", w, h)
	}
	peak := wh * 8
	if peak > int64(math.MaxInt) {
		return 0, fmt.Errorf("resolution too large: %dx%d", w, h)
	}
	return peak, nil
}

func ceilMiB(n int64) int64 {
	return (n + mib - 1) / mib
}

// captureExceeds reports a memory error when the peak for w×h is above
// limitBytes. shownMB is the limit printed in the error text.
func captureExceeds(w, h, limitBytes, shownMB int64) error {
	peak, err := capturePeakBytes(w, h)
	if err != nil {
		return err
	}
	if peak > limitBytes {
		return fmt.Errorf("resolution too large: %dx%d requires %d MB peak (limit: %d MB)",
			w, h, ceilMiB(peak), shownMB)
	}
	return nil
}

func checkCaptureMemory(w, h int64, limitMB int) error {
	limit, err := captureLimitBytes(limitMB)
	if err != nil {
		return err
	}
	return captureExceeds(w, h, limit, int64(captureLimitMB(limitMB)))
}

func addInt64(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, false
	}
	if b < 0 && a < math.MinInt64-b {
		return 0, false
	}
	return a + b, true
}

// intersectRect returns the intersection of [x, x+w) × [y, y+h) with the
// virtual desktop [vx, vx+vw) × [vy, vy+vh). Arithmetic is int64 and does not
// wrap. A negative intersection origin is preserved.
func intersectRect(x, y, w, h, vx, vy, vw, vh int64) (ix, iy, iw, ih int64, err error) {
	if w <= 0 || h <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("invalid region size: %dx%d", w, h)
	}
	if vw <= 0 || vh <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("invalid screen dimensions: %dx%d", vw, vh)
	}
	x2, ok := addInt64(x, w)
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("requested region is outside virtual desktop")
	}
	y2, ok := addInt64(y, h)
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("requested region is outside virtual desktop")
	}
	vx2, ok := addInt64(vx, vw)
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("invalid screen dimensions: %dx%d", vw, vh)
	}
	vy2, ok := addInt64(vy, vh)
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("invalid screen dimensions: %dx%d", vw, vh)
	}
	ix = max(x, vx)
	iy = max(y, vy)
	ix2 := min(x2, vx2)
	iy2 := min(y2, vy2)
	if ix >= ix2 || iy >= iy2 {
		return 0, 0, 0, 0, fmt.Errorf("requested region is outside virtual desktop")
	}
	return ix, iy, ix2 - ix, iy2 - iy, nil
}

func metricI32(v uintptr) int64 {
	return int64(int32(v))
}

func virtualScreen() (x, y, w, h int64) {
	vx, _, _ := window.ProcGetSystemMetrics.Call(SM_XVIRTUALSCREEN)
	vy, _, _ := window.ProcGetSystemMetrics.Call(SM_YVIRTUALSCREEN)
	vw, _, _ := window.ProcGetSystemMetrics.Call(SM_CXVIRTUALSCREEN)
	vh, _, _ := window.ProcGetSystemMetrics.Call(SM_CYVIRTUALSCREEN)
	return metricI32(vx), metricI32(vy), metricI32(vw), metricI32(vh)
}

func fitI32(v int64) (int32, bool) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, false
	}
	return int32(v), true
}

// callingThreadPerMonitor reports whether the calling thread is per-monitor
// aware. known is false when either thread API is missing; the caller then
// uses the process check. When known is true, aware is the thread result and
// the process check must not override it.
func callingThreadPerMonitor() (aware, known bool) {
	if window.ProcGetThreadDpiAwarenessContext.Find() != nil {
		return false, false
	}
	if window.ProcGetAwarenessFromDpiAwarenessContext.Find() != nil {
		return false, false
	}
	ctx, _, _ := window.ProcGetThreadDpiAwarenessContext.Call()
	aw, _, _ := window.ProcGetAwarenessFromDpiAwarenessContext.Call(ctx)
	return int32(aw) == dpiAwarenessPerMonitor, true
}

func ensureCaptureDPI() error {
	const processText = "process is not Per-Monitor DPI Aware; call winput.EnablePerMonitorDPI() first"
	aware, known := callingThreadPerMonitor()
	if !known {
		if !window.IsPerMonitorDPIAware() {
			return fmt.Errorf("%s", processText)
		}
		return nil
	}
	if aware {
		return nil
	}
	// GetDpiAwarenessContextForProcess and GetProcessDpiAwareness follow the
	// calling thread after SetThreadDpiAwarenessContext. Measured on this
	// machine: both returned the thread's system-aware context (0x6011,
	// awareness 1) while another OS thread still reported per-monitor.
	// The process default is read from a thread that has not been switched.
	if processPerMonitorOtherThread() {
		return fmt.Errorf("calling thread is not Per-Monitor DPI Aware although the process is; restore this thread's DPI awareness context")
	}
	return fmt.Errorf("%s", processText)
}

func processPerMonitorOtherThread() bool {
	ch := make(chan bool, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		ch <- window.IsPerMonitorDPIAware()
	}()
	return <-ch
}

// captureRect locks the OS thread, reads the virtual screen once, and blits
// either that whole screen (full) or its intersection with (x, y, w, h).
func captureRect(full bool, x, y, w, h int64, opts CaptureOptions) (img *image.RGBA, err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Drop a batch left on this thread by earlier GDI calls. GdiFlush reports
	// every call still queued, so a stale failure would otherwise fail us.
	window.ProcGdiFlush.Call()

	var (
		hScreenDC uintptr
		hMemDC    uintptr
		hBitmap   uintptr
		oldObj    uintptr
		selected  bool
		opErr     error
	)
	// Registered after Unlock, so release runs while the goroutine is still
	// pinned to the thread that created the DCs.
	defer func() {
		var clean error
		if selected {
			r, _, e := window.ProcSelectObject.Call(hMemDC, oldObj)
			if r == 0 {
				clean = errors.Join(clean, apiError("SelectObject", e))
			}
		}
		if hBitmap != 0 {
			r, _, e := window.ProcDeleteObject.Call(hBitmap)
			if r == 0 {
				clean = errors.Join(clean, apiError("DeleteObject", e))
			}
		}
		if hMemDC != 0 {
			r, _, e := window.ProcDeleteDC.Call(hMemDC)
			if r == 0 {
				clean = errors.Join(clean, apiError("DeleteDC", e))
			}
		}
		if hScreenDC != 0 {
			r, _, e := window.ProcReleaseDC.Call(0, hScreenDC)
			if r == 0 {
				clean = errors.Join(clean, apiError("ReleaseDC", e))
			}
		}
		if opErr != nil {
			img = nil
			err = errors.Join(opErr, clean)
			return
		}
		if clean != nil {
			err = errors.Join(err, clean)
		}
	}()

	if err = ensureCaptureDPI(); err != nil {
		return nil, err
	}

	vx, vy, vw, vh := virtualScreen()
	var ix, iy, iw, ih int64
	if full {
		if vw <= 0 || vh <= 0 {
			return nil, fmt.Errorf("invalid screen dimensions: %dx%d", vw, vh)
		}
		ix, iy, iw, ih = vx, vy, vw, vh
	} else {
		ix, iy, iw, ih, err = intersectRect(x, y, w, h, vx, vy, vw, vh)
		if err != nil {
			return nil, err
		}
	}
	if err = checkCaptureMemory(iw, ih, opts.MaxMemoryMB); err != nil {
		return nil, err
	}
	ix32, okX := fitI32(ix)
	iy32, okY := fitI32(iy)
	iw32, okW := fitI32(iw)
	ih32, okH := fitI32(ih)
	if !okX || !okY || !okW || !okH || iw32 <= 0 || ih32 <= 0 {
		return nil, fmt.Errorf("resolution too large: %dx%d", iw, ih)
	}

	var callErr error
	hScreenDC, _, callErr = window.ProcGetDC.Call(0)
	if hScreenDC == 0 {
		opErr = apiError("GetDC", callErr)
		return
	}
	hMemDC, _, callErr = window.ProcCreateCompatibleDC.Call(hScreenDC)
	if hMemDC == 0 {
		opErr = apiError("CreateCompatibleDC", callErr)
		return
	}

	// Top-down 32-bit BGRA. The converter reads this layout directly.
	bmi := BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth:       iw32,
		BiHeight:      -ih32,
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: BI_RGB,
	}
	var ppvBits unsafe.Pointer
	hBitmap, _, callErr = window.ProcCreateDIBSection.Call(
		hMemDC,
		uintptr(unsafe.Pointer(&bmi)),
		DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&ppvBits)),
		0, 0,
	)
	if hBitmap == 0 || ppvBits == nil {
		opErr = apiError("CreateDIBSection", callErr)
		return
	}

	oldObj, _, callErr = window.ProcSelectObject.Call(hMemDC, hBitmap)
	if oldObj == 0 {
		opErr = apiError("SelectObject", callErr)
		return
	}
	selected = true

	// Source coordinates are signed 32-bit virtual-screen positions. Converting
	// int32 to uintptr keeps the two's-complement low 32 bits, including negatives.
	ret, _, callErr := window.ProcBitBlt.Call(
		hMemDC,
		0, 0, uintptr(iw32), uintptr(ih32),
		hScreenDC,
		uintptr(ix32), uintptr(iy32),
		SRCCOPY,
	)
	if ret == 0 {
		opErr = apiError("BitBlt", callErr)
		return
	}
	ret, _, callErr = window.ProcGdiFlush.Call()
	if ret == 0 {
		opErr = apiError("GdiFlush", callErr)
		return
	}

	converted, convErr := convertToRGBA(ppvBits, int(iw32), int(ih32), opts.PreserveAlpha)
	if convErr != nil {
		opErr = convErr
		return
	}
	img = converted
	return
}

func convertToRGBA(ppvBits unsafe.Pointer, width, height int, preserveAlpha bool) (*image.RGBA, error) {
	if ppvBits == nil || width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid pixel buffer pointer")
	}
	n64 := int64(width) * int64(height) * 4
	if n64 <= 0 || n64 > int64(math.MaxInt) {
		return nil, fmt.Errorf("resolution too large: %dx%d", width, height)
	}
	total := int(n64)
	srcBytes := unsafe.Slice((*byte)(ppvBits), total)
	dstBytes := make([]byte, total)
	if total > 1024*1024 {
		convertBGRAtoRGBAParallel(srcBytes, dstBytes, preserveAlpha)
	} else {
		convertBGRAtoRGBASerial(srcBytes, dstBytes, preserveAlpha)
	}
	return &image.RGBA{
		Pix:    dstBytes,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}, nil
}

func convertBGRAtoRGBASerial(src, dst []byte, preserveAlpha bool) {
	// preserveAlpha is retained so callers can prove both values write 255.
	_ = preserveAlpha
	if len(src) == 0 {
		return
	}
	_ = dst[len(src)-1]
	for i := 0; i < len(src); i += 4 {
		dst[i] = src[i+2]
		dst[i+1] = src[i+1]
		dst[i+2] = src[i]
		dst[i+3] = 255
	}
}

func convertBGRAtoRGBAParallel(src, dst []byte, preserveAlpha bool) {
	n := runtime.GOMAXPROCS(0)
	if n < 2 {
		convertBGRAtoRGBASerial(src, dst, preserveAlpha)
		return
	}
	chunk := (len(src) / n) &^ 3
	if chunk == 0 {
		convertBGRAtoRGBASerial(src, dst, preserveAlpha)
		return
	}
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		start := i * chunk
		end := start + chunk
		if i == n-1 {
			end = len(src)
		}
		go func(s, e int) {
			defer wg.Done()
			convertBGRAtoRGBASerial(src[s:e], dst[s:e], preserveAlpha)
		}(start, end)
	}
	wg.Wait()
}
