package hid

import (
	"fmt"
	"github.com/rpdg/winput/hid/interception"
	"github.com/rpdg/winput/internal/motion"
	"github.com/rpdg/winput/screen"
	"github.com/rpdg/winput/window"
	"math/rand"
	"time"
)

func interceptionMotion() mouseMotion {
	return mouseMotion{
		runtime: motion.Runtime{Cursor: window.GetCursorPos, Sleep: time.Sleep, Intn: rand.Intn, After: time.After},
		inside:  func(x, y int32) bool { return insideVirtualBounds(x, y, screen.VirtualBounds()) },
		acquire: func() (motion.Session, func(), error) {
			c, d, unlock, err := acquireMouse()
			if err != nil {
				return motion.Session{}, nil, err
			}
			return mouseSession(c, d, interception.SendMouse), unlock, nil
		},
		setCursor: func(x, y int32) error {
			r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
			if r == 0 {
				return fmt.Errorf("SetCursorPos failed")
			}
			return nil
		},
		doubleClickTime: func() time.Duration {
			r, _, _ := window.ProcGetDoubleClickTime.Call()
			return time.Duration(r) * time.Millisecond
		},
		outside: ErrOutsideVirtualDesktop, notConverged: ErrMoveNotConverged,
	}
}

// Bind captured phase handles directly to raw sends, never exported hid input
// functions (which would recursively acquire initMutex in a pending writer).
func mouseSession(c interception.Context, d interception.Device,
	send func(interception.Context, interception.Device, *interception.MouseStroke) error) motion.Session {
	return motion.Session{
		MoveRelative: func(dx, dy int32) error {
			stroke := interception.MouseStroke{Flags: interception.MouseFlagMoveRelative, X: dx, Y: dy}
			return send(c, d, &stroke)
		},
		Button: func(b motion.Button, down bool) error {
			var state uint16
			switch b {
			case motion.Left:
				state = interception.MouseStateLeftUp
				if down {
					state = interception.MouseStateLeftDown
				}
			case motion.Right:
				state = interception.MouseStateRightUp
				if down {
					state = interception.MouseStateRightDown
				}
			case motion.Middle:
				state = interception.MouseStateMiddleUp
				if down {
					state = interception.MouseStateMiddleDown
				}
			default:
				panic("invalid internal motion button")
			}
			stroke := interception.MouseStroke{State: state}
			return send(c, d, &stroke)
		},
	}
}
