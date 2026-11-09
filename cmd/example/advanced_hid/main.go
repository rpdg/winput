package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/rpdg/winput"
)

func main() {
	fmt.Println("=== winput: HID Backend Example ===")
	fmt.Println("⚠️  This mode moves the PHYSICAL cursor and simulates hardware input.")
	fmt.Println("⚠️  Requires Interception driver installed.")

	// 1. Setup HID
	// You can specify path to dll if not in current dir
	winput.SetHIDLibraryPath("interception.dll")
	if err := winput.SetBackend(winput.BackendHID); err != nil {
		if errors.Is(err, winput.ErrDriverNotInstalled) {
			log.Fatal("❌ Interception driver not found. Please install it.")
		}
		log.Fatalf("❌ HID Setup failed: %v", err)
	}

	// 2. Find Window (Optional in HID mode, but good for relative coords)
	w, err := winput.FindByProcessName("notepad.exe")
	if err != nil || len(w) == 0 {
		log.Println("❌ Notepad not found. Please open Notepad.")
		return
	}
	target := w[0]

	// Bring to front (User manual action required usually)
	fmt.Println("Please focus the Notepad window within 3 seconds...")
	time.Sleep(3 * time.Second)

	// 3. Perform Input
	fmt.Println("👉 Moving Mouse (Human-like trajectory)...")
	// Moves to (100, 100) inside the window
	if err := target.Move(100, 100); err != nil {
		log.Fatal(err)
	}

	fmt.Println("👉 Typing via Hardware Simulation...")
	target.Click(100, 100) // Click to focus caret
	// HID Type sends only while this window's top-level ancestor is foreground.
	if err := target.Type("HID Input Test"); err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Done ===")
}
