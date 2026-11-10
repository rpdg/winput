package main

import (
	"fmt"
	"image/png"
	"log"
	"os"

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

	monitors, err := screen.Monitors()
	if err != nil {
		log.Printf("⚠️  Could not query monitors: %v", err)
	}
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

	// Save captured image to a file
	f, err := os.Create("capture.png")
	if err != nil {
		log.Printf("❌ Failed to create capture.png: %v", err)
	} else {
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			log.Printf("❌ Failed to encode PNG: %v", err)
		} else {
			fmt.Println("✅ Image saved to capture.png")
		}
	}

	// 4. Vision Coordinate Mapping (ImageToVirtual)
	// Converts OpenCV / template-match image coordinates from a full virtual desktop capture
	// to Windows Virtual Desktop coordinates (supporting multi-monitors and negative offsets).
	imgCenterX := int32(img.Bounds().Dx() / 2)
	imgCenterY := int32(img.Bounds().Dy() / 2)
	vx, vy := screen.ImageToVirtual(imgCenterX, imgCenterY)
	fmt.Printf("👉 Mapped image center (%d, %d) -> Virtual Desktop (%d, %d)\n", imgCenterX, imgCenterY, vx, vy)

	// 5. Global Input Demo
	fmt.Printf("👉 Moving mouse to virtual desktop center (%d, %d)...\n", vx, vy)
	if err := winput.MoveMouseTo(vx, vy); err != nil {
		log.Printf("⚠️  MoveMouseTo failed: %v", err)
	}

	// Simulate typing "globally" (goes to the active foreground window)
	fmt.Println("👉 Typing globally...")
	if err := winput.Type("Global Input Simulation"); err != nil {
		log.Printf("⚠️  Type failed: %v", err)
	}

	fmt.Println("=== Done ===")
}
