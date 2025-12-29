package winput_test

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/rpdg/winput"
	"github.com/rpdg/winput/screen"
)

// Define command line flags
// Run with: go test -v -hid
var useHID = flag.Bool("hid", false, "Run tests using HID backend (requires driver and admin)")

// TestMain parses flags
func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

// setupTestApp launches notepad and returns its Window object
func setupTestApp(t *testing.T) (*winput.Window, *exec.Cmd) {
	cmd := exec.Command("notepad.exe")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start notepad: %v", err)
	}

	// Wait for window initialization
	time.Sleep(1 * time.Second)

	// Try finding window by process name
	wins, err := winput.FindByProcessName("notepad.exe")
	if err != nil || len(wins) == 0 {
		// Clean up
		cmd.Process.Kill()
		t.Fatalf("Could not find notepad window after launch: %v", err)
	}

	// Assume the first one is the target
	targetWin := wins[0]

	if !targetWin.IsVisible() {
		t.Log("Warning: Notepad window is not visible")
	}

	return targetWin, cmd
}

func cleanupTestApp(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}

func abs(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// -----------------------------------------------------------------------------
// 1. Window Discovery & State Tests
// -----------------------------------------------------------------------------

func TestWindowDiscovery(t *testing.T) {
	w, cmd := setupTestApp(t)
	defer cleanupTestApp(cmd)

	t.Run("IsValid", func(t *testing.T) {
		if !w.IsValid() {
			t.Error("Window should be valid")
		}
	})

	t.Run("FindByPID", func(t *testing.T) {
		pid := uint32(cmd.Process.Pid)
		wins, err := winput.FindByPID(pid)
		if err != nil {
			t.Fatalf("Failed to find by PID %d: %v", pid, err)
		}
		if len(wins) == 0 {
			t.Error("FindByPID returned empty list")
		}
	})

	t.Run("Coordinates", func(t *testing.T) {
		w, h, err := w.ClientRect()
		if err != nil {
			t.Errorf("ClientRect failed: %v", err)
		}
		t.Logf("Notepad Client Area: %dx%d", w, h)
		if w <= 0 || h <= 0 {
			t.Error("Client area dimensions seem invalid")
		}
	})
}

// -----------------------------------------------------------------------------
// 2. Mouse Input Tests (Global & Relative)
// -----------------------------------------------------------------------------

func TestMouseInput(t *testing.T) {
	// Test Message Backend (Default)
	winput.SetBackend(winput.BackendMessage)

	w, cmd := setupTestApp(t)
	defer cleanupTestApp(cmd)

	t.Run("GlobalMove", func(t *testing.T) {
		// Move to screen 100, 100
		targetX, targetY := int32(100), int32(100)
		if err := winput.MoveMouseTo(targetX, targetY); err != nil {
			t.Fatalf("MoveMouseTo failed: %v", err)
		}

		// Verify
		curX, curY, _ := winput.GetCursorPos()
		if curX != targetX || curY != targetY {
			t.Errorf("Mouse position mismatch. Expected %d,%d, Got %d,%d", targetX, targetY, curX, curY)
		}
	})

	t.Run("WindowRelativeMove", func(t *testing.T) {
		// Move to client area (50, 50)
		if err := w.Move(50, 50); err != nil {
			t.Fatalf("Window.Move failed: %v", err)
		}

		// Note: BackendMessage sends WM_MOUSEMOVE but does NOT move the physical cursor.
		// So GetCursorPos() will return the old position (from GlobalMove).
		// We cannot verify internal app state here.
		// However, we can verify that the library logic didn't crash.
		t.Log("Window.Move executed (Message Backend does not move physical cursor)")
	})

	t.Run("Click", func(t *testing.T) {
		// Click center to focus
		w.Click(100, 100)
	})
}

// -----------------------------------------------------------------------------
// 3. Keyboard Input Tests
// -----------------------------------------------------------------------------

func TestKeyboardInput(t *testing.T) {
	winput.SetBackend(winput.BackendMessage)

	w, cmd := setupTestApp(t)
	defer cleanupTestApp(cmd)

	// Ensure focus
	w.Click(200, 200)
	time.Sleep(500 * time.Millisecond)

	t.Run("TypeString", func(t *testing.T) {
		text := "Hello winput"
		if err := winput.Type(text); err != nil {
			// In CI/Headless environments, SendInput (Global Type) often fails.
			// This is not a library bug but an environment limitation.
			if err.Error() == "SendInput self-test failed; unsupported in this context" {
				t.Skipf("Skipping Global Type test: %v", err)
			}
			t.Errorf("Type failed: %v", err)
		}
	})

	t.Run("Hotkey SelectAll", func(t *testing.T) {
		// Ctrl + A
		if err := winput.PressHotkey(winput.KeyCtrl, winput.KeyA); err != nil {
			t.Errorf("PressHotkey failed: %v", err)
		}
	})

	t.Run("WindowType", func(t *testing.T) {
		// Target specific window
		if err := w.Type("Test"); err != nil {
			t.Errorf("Window.Type failed: %v", err)
		}
	})
}

// -----------------------------------------------------------------------------
// 4. HID Backend Tests (Requires Driver)
// -----------------------------------------------------------------------------

func TestBackendHID(t *testing.T) {
	if !*useHID {
		t.Skip("Skipping HID tests. Use -hid flag to enable (requires admin & driver).")
	}

	winput.SetBackend(winput.BackendHID)
	// Assuming dll is in path or current dir
	winput.SetHIDLibraryPath("interception.dll")

	// Check init
	if err := winput.MoveMouseTo(0, 0); err != nil {
		t.Fatalf("HID Initialization failed (drivers installed?): %v", err)
	}

	t.Run("HID_MouseTrajectory", func(t *testing.T) {
		start := time.Now()
		// HID move should have delay (human-like)
		winput.MoveMouseTo(500, 500)
		duration := time.Since(start)

		t.Logf("HID Move took %v", duration)
		if duration < 10*time.Millisecond {
			t.Error("HID Move was too fast, trajectory simulation might be broken")
		}

		x, y, _ := winput.GetCursorPos()
		if abs(x-500) > 5 || abs(y-500) > 5 {
			// HID has jitter, allow error
			t.Errorf("HID Move destination inaccurate. Got %d,%d", x, y)
		}
	})

	t.Run("HID_Type", func(t *testing.T) {
		winput.ClickMouseAt(500, 500)
		if err := winput.Type("hid test"); err != nil {
			t.Errorf("HID Type failed: %v", err)
		}
	})

	t.Run("HID_DBL_CLICK", func(t *testing.T) {
		e := winput.DoubleClickMouseAt(50, 50)
		if e != nil {
			t.Error("HID double click error")
		}
	})
}

// -----------------------------------------------------------------------------
// 5. Multi-Monitor Support Tests
// -----------------------------------------------------------------------------

func TestMultiMonitorSupport(t *testing.T) {
	if err := winput.EnablePerMonitorDPI(); err != nil {
		t.Logf("Warning: Failed to enable Per-Monitor DPI: %v", err)
	}

	monitors, err := screen.Monitors()
	if err != nil {
		t.Fatalf("Failed to enumerate monitors: %v", err)
	}

	t.Logf("Detected %d monitor(s)", len(monitors))
	for i, m := range monitors {
		t.Logf("Monitor %d: Primary=%v, Bounds=%+v", i, m.Primary, m.Bounds)
	}

	// Test A: Virtual Bounds Consistency
	t.Run("VirtualBoundsConsistency", func(t *testing.T) {
		vBounds := screen.VirtualBounds()
		t.Logf("Virtual Desktop Bounds: %+v", vBounds)

		for i, m := range monitors {
			if m.Bounds.Left < vBounds.Left ||
				m.Bounds.Top < vBounds.Top ||
				m.Bounds.Right > vBounds.Right ||
				m.Bounds.Bottom > vBounds.Bottom {
				t.Errorf("Monitor %d is outside reported VirtualBounds. Mon: %+v, Virtual: %+v",
					i, m.Bounds, vBounds)
			}
		}
	})

	// Test B: Cross Monitor Movement
	t.Run("CrossMonitorMovement", func(t *testing.T) {
		if len(monitors) < 2 {
			t.Skip("Skipping cross-monitor test: only 1 monitor detected")
		}

		for i, m := range monitors {
			t.Run(fmt.Sprintf("MoveToMonitor_%d", i), func(t *testing.T) {
				// Move to center to avoid dead zones
				centerX := (m.Bounds.Left + m.Bounds.Right) / 2
				centerY := (m.Bounds.Top + m.Bounds.Bottom) / 2

				t.Logf("Attempting to move to Monitor %d Center: (%d, %d)", i, centerX, centerY)

				if err := winput.MoveMouseTo(centerX, centerY); err != nil {
					t.Errorf("Failed to move to monitor %d: %v", i, err)
					return
				}

				time.Sleep(50 * time.Millisecond)

				currX, currY, err := winput.GetCursorPos()
				if err != nil {
					t.Errorf("Failed to get cursor pos: %v", err)
					return
				}

				if abs(currX-centerX) > 2 || abs(currY-centerY) > 2 {
					t.Errorf("Position mismatch on Monitor %d. Target(%d,%d), Got(%d,%d)",
						i, centerX, centerY, currX, currY)
				} else {
					t.Logf("Success: Reached Monitor %d", i)
				}
			})
		}
	})

	// Test C: ImageToVirtual Consistency
	t.Run("ImageToVirtualConsistency", func(t *testing.T) {
		vBounds := screen.VirtualBounds()

		// Simulate a point on the screenshot (e.g., 100, 100 from top-left)hidtest
		imgX, imgY := int32(100), int32(100)

		virtX, virtY := screen.ImageToVirtual(imgX, imgY)

		expectedX := vBounds.Left + imgX
		expectedY := vBounds.Top + imgY

		if virtX != expectedX || virtY != expectedY {
			t.Errorf("ImageToVirtual calculation mismatch. Expected (%d, %d), Got (%d, %d). Origin was (%d, %d)",
				expectedX, expectedY, virtX, virtY, vBounds.Left, vBounds.Top)
		} else {
			t.Logf("ImageToVirtual correct: Image(%d, %d) + Origin(%d, %d) -> Virtual(%d, %d)",
				imgX, imgY, vBounds.Left, vBounds.Top, virtX, virtY)
		}
	})
}
