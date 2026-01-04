package main

import (
	"fmt"
	"log"

	"github.com/rpdg/winput"
	"github.com/rpdg/winput/screen"
)

func main() {
	fmt.Println("=== winput: Global Vision Example ===")
	fmt.Println("This mode uses absolute screen coordinates and screen capture.")

	// 1. Enable DPI Awareness (Mandatory for screen capture accuracy)
	if err := winput.EnablePerMonitorDPI(); err != nil {
		log.Printf("Warning: Could not enable DPI awareness: %v", err)
	}

	// 2. Screen Geometry Info
	bounds := screen.VirtualBounds()
	fmt.Printf("🖥️  Virtual Desktop: [%d, %d, %d, %d]\n", bounds.Left, bounds.Top, bounds.Right, bounds.Bottom)

	monitors, _ := screen.Monitors()
	for i, m := range monitors {
		fmt.Printf("   Monitor %d: %+v (Primary: %v)\n", i, m.Bounds, m.Primary)
	}

	// 3. Screen Capture Demo
	fmt.Println("👉 Capturing virtual desktop...")
	img, err := screen.CaptureVirtualDesktop()
	if err != nil {
		log.Fatalf("❌ Capture failed: %v", err)
	}
	fmt.Printf("✅ Captured %dx%d image (Standard *image.RGBA)\n", img.Bounds().Dx(), img.Bounds().Dy())

	// 4. Global Input Demo
	if len(monitors) > 0 {
		// Move to center of primary monitor
		center := monitors[0].Bounds
		cx := (center.Left + center.Right) / 2
		cy := (center.Top + center.Bottom) / 2

		fmt.Printf("👉 Moving mouse to center of primary monitor (%d, %d)...\n", cx, cy)
		winput.MoveMouseTo(cx, cy)
	}

	// Simulate typing "globally" (goes to active window)
	fmt.Println("👉 Typing globally...")
	winput.Type("Global Input Simulation")

	fmt.Println("=== Done ===")
}