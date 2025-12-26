package screen

import (
	"syscall"
	"unsafe"

	"github.com/rpdg/winput/window"
)

// SM_CXVIRTUALSCREEN = 78
// SM_CYVIRTUALSCREEN = 79
// SM_XVIRTUALSCREEN = 76
// SM_YVIRTUALSCREEN = 77

// VirtualBounds returns the bounding rectangle of the entire virtual desktop.
// This includes all monitors.
func VirtualBounds() Rect {
	x, _, _ := window.ProcGetSystemMetrics.Call(76)
	y, _, _ := window.ProcGetSystemMetrics.Call(77)
	w, _, _ := window.ProcGetSystemMetrics.Call(78)
	h, _, _ := window.ProcGetSystemMetrics.Call(79)

	return Rect{
		Left:   int32(x),
		Top:    int32(y),
		Right:  int32(x) + int32(w),
		Bottom: int32(y) + int32(h),
	}
}

// ImageToVirtual converts coordinates from a "Full Virtual Desktop Screenshot"
// to actual Windows Virtual Desktop coordinates.
//
// Use this when you capture the entire multi-monitor desktop as a single image
// (origin 0,0) and find a match at (imageX, imageY).
//
// Returns (x, y) ready for use with winput.MoveMouseTo / winput.ClickMouseAt.
//
// Constraints:
// 1. The image MUST be a capture of the entire virtual desktop (all monitors).
// 2. The capture process MUST be Per-Monitor DPI Aware (matching winput).
// 3. Do NOT modify the returned negative coordinates; they are valid.
func ImageToVirtual(imageX, imageY int32) (int32, int32) {
	vx, _, _ := window.ProcGetSystemMetrics.Call(76) // SM_XVIRTUALSCREEN
	vy, _, _ := window.ProcGetSystemMetrics.Call(77) // SM_YVIRTUALSCREEN

	return imageX + int32(vx), imageY + int32(vy)
}

// Monitors returns a list of all active monitors.
func Monitors() ([]Monitor, error) {
	var monitors []Monitor

	cb := syscall.NewCallback(func(hMonitor uintptr, hdcMonitor uintptr, lprcMonitor uintptr, dwData uintptr) uintptr {
		var mi monitorInfoExW
		mi.Size = uint32(unsafe.Sizeof(mi))

		ret, _, _ := window.ProcGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi)))
		if ret != 0 {
			mon := Monitor{
				Handle: hMonitor,
				Bounds: Rect{
					Left:   mi.Monitor.Left,
					Top:    mi.Monitor.Top,
					Right:  mi.Monitor.Right,
					Bottom: mi.Monitor.Bottom,
				},
				WorkArea: Rect{
					Left:   mi.Work.Left,
					Top:    mi.Work.Top,
					Right:  mi.Work.Right,
					Bottom: mi.Work.Bottom,
				},
				Primary: (mi.Flags & 1) != 0, // MONITORINFOF_PRIMARY = 1
			}
			monitors = append(monitors, mon)
		}
		return 1
	})

	window.ProcEnumDisplayMonitors.Call(0, 0, cb, 0)
	return monitors, nil
}

type rectStruct struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type monitorInfoExW struct {
	Size    uint32
	Monitor rectStruct
	Work    rectStruct
	Flags   uint32
	Device  [32]uint16
}
