package screen

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/rpdg/winput/window"
)

func TestAPIError(t *testing.T) {
	errno := syscall.Errno(5)
	err := apiError("GetDC", errno)
	if !errors.Is(err, syscall.Errno(5)) {
		t.Fatalf("errors.Is: %v", err)
	}
	want := fmt.Sprintf("GetDC failed: %d: %s", uint32(errno), errno.Error())
	if err.Error() != want {
		t.Fatalf("text %q, want %q", err.Error(), want)
	}

	err = apiError("BitBlt", syscall.Errno(0))
	if err.Error() != "BitBlt failed" {
		t.Fatalf("zero errno text %q", err.Error())
	}
	var got syscall.Errno
	if errors.As(err, &got) {
		t.Fatal("zero errno was wrapped")
	}

	err = apiError("GdiFlush", errors.New("other"))
	if err.Error() != "GdiFlush failed" {
		t.Fatalf("plain error text %q", err.Error())
	}
	if errors.As(err, &got) {
		t.Fatal("plain error was wrapped")
	}

	err = apiError("CreateDIBSection", nil)
	if err.Error() != "CreateDIBSection failed" {
		t.Fatalf("nil error text %q", err.Error())
	}
}

func TestIntersectRect(t *testing.T) {
	tests := []struct {
		name           string
		x, y, w, h     int64
		vx, vy, vw, vh int64
		ix, iy, iw, ih int64
		wantErr        string
	}{
		{
			name: "inside",
			x:    10, y: 20, w: 30, h: 40,
			vx: 0, vy: 0, vw: 100, vh: 200,
			ix: 10, iy: 20, iw: 30, ih: 40,
		},
		{
			name: "cross top-left",
			x:    -10, y: -20, w: 30, h: 40,
			vx: 0, vy: 0, vw: 100, vh: 100,
			ix: 0, iy: 0, iw: 20, ih: 20,
		},
		{
			name: "cross bottom-right",
			x:    90, y: 80, w: 30, h: 40,
			vx: 0, vy: 0, vw: 100, vh: 100,
			ix: 90, iy: 80, iw: 10, ih: 20,
		},
		{
			name: "negative monitor inside",
			x:    -100, y: 10, w: 50, h: 60,
			vx: -1920, vy: 0, vw: 3840, vh: 1080,
			ix: -100, iy: 10, iw: 50, ih: 60,
		},
		{
			name: "negative intersection origin",
			x:    -1800, y: -50, w: 100, h: 80,
			vx: -1920, vy: -100, vw: 3840, vh: 1200,
			ix: -1800, iy: -50, iw: 100, ih: 80,
		},
		{
			name: "outside",
			x:    1000, y: 1000, w: 10, h: 10,
			vx: 0, vy: 0, vw: 100, vh: 100,
			wantErr: "outside virtual desktop",
		},
		{
			name: "non-positive size",
			x:    0, y: 0, w: 0, h: 10,
			vx: 0, vy: 0, vw: 100, vh: 100,
			wantErr: "invalid region size",
		},
		{
			name: "MaxInt32 origin outside does not wrap",
			x:    math.MaxInt32, y: 0, w: 10, h: 10,
			vx: 0, vy: 0, vw: 100, vh: 100,
			wantErr: "outside virtual desktop",
		},
		{
			name: "MinInt32 inside virtual screen",
			x:    math.MinInt32, y: 0, w: 20, h: 10,
			vx: math.MinInt32, vy: 0, vw: 100, vh: 50,
			ix: math.MinInt32, iy: 0, iw: 20, ih: 10,
		},
		{
			name: "x+w past MaxInt32 stays in int64",
			x:    math.MaxInt32 - 5, y: 0, w: 20, h: 10,
			vx: math.MaxInt32 - 50, vy: 0, vw: 100, vh: 20,
			ix: math.MaxInt32 - 5, iy: 0, iw: 20, ih: 10,
		},
		{
			name: "MinInt32 request clipped to a negative monitor",
			x:    math.MinInt32, y: 0, w: math.MaxInt32, h: 10,
			vx: -100, vy: 0, vw: 50, vh: 10,
			ix: -100, iy: 0, iw: 50, ih: 10,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix, iy, iw, ih, err := intersectRect(tt.x, tt.y, tt.w, tt.h, tt.vx, tt.vy, tt.vw, tt.vh)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ix != tt.ix || iy != tt.iy || iw != tt.iw || ih != tt.ih {
				t.Fatalf("got (%d,%d) %dx%d, want (%d,%d) %dx%d", ix, iy, iw, ih, tt.ix, tt.iy, tt.iw, tt.ih)
			}
		})
	}
}

func TestCaptureRegionRejectsNonPositiveSize(t *testing.T) {
	_, err := CaptureRegion(0, 0, 0, 10)
	if err == nil || !strings.Contains(err.Error(), "invalid region size") {
		t.Fatalf("CaptureRegion: %v", err)
	}
	_, err = CaptureRegionWithOptions(1, 1, 8, -1, defaultOptions)
	if err == nil || !strings.Contains(err.Error(), "invalid region size") {
		t.Fatalf("CaptureRegionWithOptions: %v", err)
	}
}

func TestCaptureMemory(t *testing.T) {
	if defaultOptions.MaxMemoryMB != defaultCaptureMemoryMB || defaultCaptureMemoryMB != 1000 {
		t.Fatalf("default MaxMemoryMB = %d", defaultOptions.MaxMemoryMB)
	}
	for _, mb := range []int{0, -1, -100} {
		got, err := captureLimitBytes(mb)
		if err != nil || got != 1000*mib {
			t.Fatalf("limit %d: %d %v", mb, got, err)
		}
	}

	peak, err := capturePeakBytes(256, 512)
	if err != nil {
		t.Fatal(err)
	}
	if peak != int64(mib) {
		t.Fatalf("peak %d, want 1 MiB", peak)
	}
	if err := captureExceeds(256, 512, peak, 1); err != nil {
		t.Fatal(err)
	}
	if err := captureExceeds(256, 512, peak-1, 1); err == nil {
		t.Fatal("peak+1 byte was accepted")
	}

	// 11520×2160 is three 4K panels. One buffer is under 100 MiB; the peak is not.
	const panelsW, panelsH = int64(11520), int64(2160)
	err = checkCaptureMemory(panelsW, panelsH, 100)
	want := "resolution too large: 11520x2160 requires 190 MB peak (limit: 100 MB)"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v", err)
	}

	// Exact default budget: 8 × 1024 × 128000 = 1000 MiB.
	if err := checkCaptureMemory(1024, 128000, 0); err != nil {
		t.Fatal(err)
	}
	if err := checkCaptureMemory(1024, 128000, 999); err == nil {
		t.Fatal("999MB accepted a 1000MB peak")
	}

	if _, err := capturePeakBytes(math.MaxInt64, 2); err == nil {
		t.Fatal("MaxInt64×2 was accepted")
	}
	if _, err := capturePeakBytes(1<<32, 1<<32); err == nil {
		t.Fatal("2^32 × 2^32 was accepted")
	}
	tooBigMB := int64(math.MaxInt64/mib) + 1
	if tooBigMB <= int64(math.MaxInt) {
		if _, err := captureLimitBytes(int(tooBigMB)); err == nil {
			t.Fatal("overflowing limit was accepted")
		}
	}

	// 20000×20000 peaks at 3_200_000_000, above MaxInt32 and inside int64.
	widePeak := int64(20000) * int64(20000) * 8
	_, err = capturePeakBytes(20000, 20000)
	if widePeak > int64(math.MaxInt) {
		if err == nil {
			t.Fatal("peak above MaxInt was accepted")
		}
	} else if err != nil {
		t.Fatal(err)
	}
}

func TestConvertBGRA(t *testing.T) {
	prev := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(prev)

	convertBGRAtoRGBASerial(nil, nil, true)
	convertBGRAtoRGBASerial(nil, nil, false)
	empty := []byte{}
	convertBGRAtoRGBASerial(empty, empty, true)

	sizes := []int{
		4,
		1024*1024 + 4,
		1920 * 1080 * 4,
		10004, // last parallel chunk is not an even split at GOMAXPROCS 4
	}
	for _, n := range sizes {
		src := make([]byte, n)
		for i := 0; i < n; i += 4 {
			src[i] = byte(i)
			src[i+1] = byte(i + 1)
			src[i+2] = byte(i + 2)
			src[i+3] = 0
		}
		for _, preserve := range []bool{false, true} {
			serial := make([]byte, n)
			parallel := make([]byte, n)
			convertBGRAtoRGBASerial(src, serial, preserve)
			convertBGRAtoRGBAParallel(src, parallel, preserve)
			if string(serial) != string(parallel) {
				t.Fatalf("len %d preserve %v: serial and parallel differ", n, preserve)
			}
			for i := 3; i < n; i += 4 {
				if serial[i] != 255 {
					t.Fatalf("len %d preserve %v: alpha %d at %d", n, preserve, serial[i], i)
				}
			}
			if serial[0] != src[2] || serial[1] != src[1] || serial[2] != src[0] {
				t.Fatalf("len %d: BGRA swap mismatch", n)
			}
		}
	}

	src := []byte{1, 2, 3, 0, 4, 5, 6, 9}
	img, err := convertToRGBA(unsafe.Pointer(&src[0]), 2, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if img.Rect != image.Rect(0, 0, 2, 1) {
		t.Fatalf("rect %v", img.Rect)
	}
	if img.Pix[3] != 255 || img.Pix[7] != 255 || img.Pix[0] != 3 {
		t.Fatalf("pix %v", img.Pix)
	}
}

func TestCaptureDPIAwareness(t *testing.T) {
	switch os.Getenv("WINPUT_CAPTURE_DPI_CASE") {
	case "default":
		childDefaultDPI()
	case "thread":
		childThreadDPI()
	}
	for _, mode := range []string{"default", "thread"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestCaptureDPIAwareness$")
		cmd.Env = append(os.Environ(), "WINPUT_CAPTURE_DPI_CASE="+mode)
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), mode+"_OK") {
			t.Fatalf("child %s: %v\n%s", mode, err, out)
		}
	}
}

func childDefaultDPI() {
	_, err := CaptureVirtualDesktop()
	if err == nil {
		fmt.Println("default unexpected success")
		os.Exit(2)
	}
	msg := err.Error()
	if !strings.Contains(msg, "process is not Per-Monitor DPI Aware") || strings.Contains(msg, "although the process is") {
		fmt.Println(msg)
		os.Exit(2)
	}
	fmt.Println("default_OK")
	os.Exit(0)
}

func childThreadDPI() {
	runtime.LockOSThread()
	if err := window.EnablePerMonitorDPI(); err != nil || !window.IsPerMonitorDPIAware() {
		fmt.Printf("thread process not aware: %v\n", err)
		os.Exit(2)
	}
	set := syscall.NewLazyDLL("user32.dll").NewProc("SetThreadDpiAwarenessContext")
	if err := set.Find(); err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	old, _, _ := set.Call(window.DPI_AWARENESS_CONTEXT_SYSTEM_AWARE)
	if old == 0 {
		fmt.Println("SetThreadDpiAwarenessContext failed")
		os.Exit(2)
	}
	_, err := CaptureVirtualDesktop()
	set.Call(old)
	if err == nil || !strings.Contains(err.Error(), "calling thread is not Per-Monitor DPI Aware although the process is") {
		fmt.Printf("thread error %v\n", err)
		os.Exit(2)
	}
	img, err := CaptureVirtualDesktop()
	if err != nil {
		fmt.Printf("restore capture %v\n", err)
		os.Exit(2)
	}
	if img == nil || img.Bounds().Empty() {
		fmt.Println("restore empty")
		os.Exit(2)
	}
	fmt.Println("thread_OK")
	os.Exit(0)
}

func TestCaptureRegionPixels(t *testing.T) {
	if !window.IsPerMonitorDPIAware() {
		if err := window.EnablePerMonitorDPI(); err != nil || !window.IsPerMonitorDPIAware() {
			t.Skipf("per-monitor DPI unavailable: %v", err)
		}
	}
	full, err := CaptureVirtualDesktop()
	if err != nil {
		t.Skipf("capture unavailable: %v", err)
	}
	vb := VirtualBounds()
	if full.Bounds().Dx() != int(vb.Right-vb.Left) || full.Bounds().Dy() != int(vb.Bottom-vb.Top) {
		t.Fatalf("full %v virtual %v", full.Bounds(), vb)
	}
	if full.Bounds().Dx() < 32 || full.Bounds().Dy() < 24 {
		t.Skipf("virtual desktop %v is smaller than the region check", full.Bounds())
	}
	checkAlpha(t, full)

	// Request extends above and left of the virtual origin. The result is the
	// overlap, whose (0, 0) matches the full image's (0, 0).
	const clipX, clipY = int32(8), int32(4)
	partial, err := CaptureRegion(vb.Left-clipX, vb.Top-clipY, clipX+16, clipY+12)
	if err != nil {
		t.Fatal(err)
	}
	if partial.Bounds() != image.Rect(0, 0, 16, 12) {
		t.Fatalf("partial bounds %v", partial.Bounds())
	}
	if !pixelsEqual(partial, full, 0, 0, 16, 12) {
		t.Fatal("partial clip does not match the full capture")
	}

	region, err := CaptureRegionWithOptions(vb.Left, vb.Top, 32, 24, CaptureOptions{PreserveAlpha: true})
	if err != nil {
		t.Fatal(err)
	}
	if region.Bounds() != image.Rect(0, 0, 32, 24) {
		t.Fatalf("region bounds %v", region.Bounds())
	}
	if !pixelsEqual(region, full, 0, 0, 32, 24) {
		// The desktop may have changed between the two captures. One retry
		// pair distinguishes that from a stable offset bug.
		full, err = CaptureVirtualDesktop()
		if err != nil {
			t.Fatal(err)
		}
		region, err = CaptureRegionWithOptions(vb.Left, vb.Top, 32, 24, CaptureOptions{PreserveAlpha: false})
		if err != nil {
			t.Fatal(err)
		}
		if !pixelsEqual(region, full, 0, 0, 32, 24) {
			t.Fatal("region does not match the same pixels of the full capture")
		}
	}
	checkAlpha(t, region)
}

func checkAlpha(t *testing.T, img *image.RGBA) {
	t.Helper()
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 255 {
			t.Fatalf("alpha %d at byte %d", img.Pix[i], i)
		}
	}
}

func pixelsEqual(region, full *image.RGBA, x, y, w, h int) bool {
	for row := 0; row < h; row++ {
		rs := row*region.Stride + x*4
		fs := (y+row)*full.Stride + x*4
		for col := 0; col < w*4; col++ {
			if region.Pix[rs+col] != full.Pix[fs+col] {
				return false
			}
		}
	}
	return true
}
