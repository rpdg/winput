package screen

import (
	"fmt"
	"image"
	"runtime"
	"sync"
	"unsafe"

	"github.com/rpdg/winput/window"
)

// GDI Constants
const (
	SRCCOPY        = 0x00CC0020
	DIB_RGB_COLORS = 0
	BI_RGB         = 0
	
	// GetSystemMetrics constants
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79
)

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

// CaptureOptions defines configuration for screen capture.
type CaptureOptions struct {
	PreserveAlpha bool
	MaxMemoryMB   int // Max memory usage in MB, 0 means default limit (500MB)
}

var defaultOptions = CaptureOptions{
	PreserveAlpha: false,
	MaxMemoryMB:   500,
}

// CaptureVirtualDesktop captures the entire virtual desktop (all monitors).
// It returns an *image.RGBA ready for OpenCV or other processing.
// It requires the process to be Per-Monitor DPI Aware.
func CaptureVirtualDesktop() (*image.RGBA, error) {
	return CaptureVirtualDesktopWithOptions(defaultOptions)
}

// CaptureVirtualDesktopWithOptions captures the virtual desktop with custom options.
func CaptureVirtualDesktopWithOptions(opts CaptureOptions) (*image.RGBA, error) {
	// 1. DPI Awareness Check
	if !window.IsPerMonitorDPIAware() {
		return nil, fmt.Errorf("process is not Per-Monitor DPI Aware; call winput.EnablePerMonitorDPI() first")
	}

	// 2. Get Virtual Desktop Bounds
	x, _, _ := window.ProcGetSystemMetrics.Call(SM_XVIRTUALSCREEN)
	y, _, _ := window.ProcGetSystemMetrics.Call(SM_YVIRTUALSCREEN)
	w, _, _ := window.ProcGetSystemMetrics.Call(SM_CXVIRTUALSCREEN)
	h, _, _ := window.ProcGetSystemMetrics.Call(SM_CYVIRTUALSCREEN)

	width := int32(w)
	height := int32(h)

	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid screen dimensions: %dx%d", width, height)
	}

	// Memory check
	limitMB := opts.MaxMemoryMB
	if limitMB <= 0 {
		limitMB = 500
	}
	totalBytes := int64(width) * int64(height) * 4
	if totalBytes > int64(limitMB)*1024*1024 {
		return nil, fmt.Errorf("resolution too large: %dx%d requires %d MB (limit: %d MB)",
			width, height, totalBytes/(1024*1024), limitMB)
	}

	// 3. Create DCs
	hScreenDC, _, _ := window.ProcGetDC.Call(0)
	if hScreenDC == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer window.ProcReleaseDC.Call(0, hScreenDC)

	hMemDC, _, _ := window.ProcCreateCompatibleDC.Call(hScreenDC)
	if hMemDC == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer window.ProcDeleteDC.Call(hMemDC)

	// 4. Create DIB Section
	bmi := BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth:       width,
		BiHeight:      -height, // Negative for Top-Down
		BiPlanes:      1,
		BiBitCount:    32, // BGRA
		BiCompression: BI_RGB,
	}

	var ppvBits unsafe.Pointer // Pointer to the raw pixel data
	hBitmap, _, _ := window.ProcCreateDIBSection.Call(
		hMemDC,
		uintptr(unsafe.Pointer(&bmi)),
		DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&ppvBits)),
		0, 0,
	)
	if hBitmap == 0 || ppvBits == nil {
		return nil, fmt.Errorf("CreateDIBSection failed")
	}

	// 5. Select Bitmap into MemDC
	oldObj, _, _ := window.ProcSelectObject.Call(hMemDC, hBitmap)
	if oldObj == 0 {
		window.ProcDeleteObject.Call(hBitmap)
		return nil, fmt.Errorf("SelectObject failed")
	}

	// 6. BitBlt: Copy Screen -> Memory -> DIB
	ret, _, _ := window.ProcBitBlt.Call(
		hMemDC,
		0, 0, uintptr(width), uintptr(height),
		hScreenDC,
		uintptr(int32(x)), uintptr(int32(y)), // Source coords
		SRCCOPY,
	)

	var img *image.RGBA
	var err error

	if ret != 0 {
		// 7. Convert to Go Image (Copy before destroying DIB)
		img, err = convertToRGBA(ppvBits, int(width), int(height), opts.PreserveAlpha)
	} else {
		err = fmt.Errorf("BitBlt failed")
	}

	// 8. Cleanup Resources
	window.ProcSelectObject.Call(hMemDC, oldObj) // Restore old object
	window.ProcDeleteObject.Call(hBitmap)        // Delete DIB

	return img, err
}

func convertToRGBA(ppvBits unsafe.Pointer, width, height int, preserveAlpha bool) (*image.RGBA, error) {
	if ppvBits == nil {
		return nil, fmt.Errorf("invalid pixel buffer pointer")
	}

	totalBytes := width * height * 4
	
	// Create slice backed by C memory (Go 1.17+)
	// This is safe because we copy immediately.
	srcBytes := unsafe.Slice((*byte)(ppvBits), totalBytes)

	// Allocate new Go managed memory
	dstBytes := make([]byte, totalBytes)

	// Use parallel conversion for large images (> 1MB)
	if totalBytes > 1024*1024 {
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
	// Simple serial loop
	// Bounds check elimination hint: len(src) == len(dst)
	_ = dst[len(src)-1]
	
	for i := 0; i < len(src); i += 4 {
		b := src[i]
		g := src[i+1]
		r := src[i+2]
		a := src[i+3]

		dst[i] = r
		dst[i+1] = g
		dst[i+2] = b
		if preserveAlpha {
			dst[i+3] = a
		} else {
			dst[i+3] = 255
		}
	}
}

func convertBGRAtoRGBAParallel(src, dst []byte, preserveAlpha bool) {
	numCPU := runtime.NumCPU()
	if numCPU < 2 {
		convertBGRAtoRGBASerial(src, dst, preserveAlpha)
		return
	}

	// Ensure chunk size aligns with 4 bytes (pixel boundary)
	// totalBytes is guaranteed to be multiple of 4
	chunkSize := (len(src) / numCPU) &^ 3 // Round down to multiple of 4
	
	var wg sync.WaitGroup
	wg.Add(numCPU)

	for i := 0; i < numCPU; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if i == numCPU-1 {
			end = len(src) // Last chunk takes the rest
		}

		go func(s, e int) {
			defer wg.Done()
			convertBGRAtoRGBASerial(src[s:e], dst[s:e], preserveAlpha)
		}(start, end)
	}
	wg.Wait()
}