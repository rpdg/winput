package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/rpdg/winput"
)

func main() {
	fmt.Println("=== winput: Basic Message Backend Example ===")
	fmt.Println("This example uses PostMessageW. It does NOT require focus or mouse movement.")

	// 1. Enable DPI Awareness
	winput.EnablePerMonitorDPI()

	// 2. Find Window
	// Target: Notepad
	w, err := winput.FindByClass("Notepad")
	if err != nil {
		log.Println("❌ Notepad not found. Please open Notepad to run this test.")
		return
	}
	fmt.Printf("✅ Found Notepad handle: %x\n", w.HWND)

	// NOTE: For some applications like Notepad, the main window handle
	// is just a container. Real input must be sent to a child window (the Edit control).
	edit, err := w.FindChildByClass("Edit")
	if err != nil {
		log.Printf("⚠️  Could not find 'Edit' child window: %v. Using main window instead.", err)
		edit = w
	} else {
		fmt.Printf("✅ Found Notepad Edit control: %x\n", edit.HWND)
	}

	// 3. Input Operations
	// Type text into the EDIT control
	fmt.Println("👉 Typing text...")
	if err := edit.Type("Hello from winput (Message Backend)!\n"); err != nil {
		if errors.Is(err, winput.ErrWindowNotVisible) {
			log.Fatal("❌ Window is minimized. Please restore it.")
		}
		log.Fatal(err)
	}

	// Press Hotkey (Select All: Ctrl+A)
	// fmt.Println("👉 Testing Hotkey (Ctrl+A)...")
	// edit.PressHotkey(winput.KeyCtrl, winput.KeyA)
	// time.Sleep(500 * time.Millisecond)

	// Mouse Click (Right Click context menu inside Edit control)
	fmt.Println("👉 Testing Right Click...")
	edit.ClickRight(100, 100)

	fmt.Println("=== Done ===")
}
