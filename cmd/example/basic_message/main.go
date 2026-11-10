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
	if err := winput.EnablePerMonitorDPI(); err != nil {
		log.Printf("⚠️  Could not enable DPI awareness: %v", err)
	}

	// 2. Find Window
	// Target: Notepad
	w, err := winput.FindByClass("Notepad")
	if err != nil {
		log.Println("❌ Notepad not found. Please open Notepad to run this test.")
		return
	}
	fmt.Printf("✅ Found Notepad handle: %x\n", w.HWND)

	// NOTE: For classic Win32 Notepad, real input is sent to the child "Edit" control.
	// For modern Windows 11 Notepad (WinUI 3), fallback to the main window container.
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

	// Read text: Value() provides best-effort retrieval with UI Automation fallback for modern apps.
	value, err := edit.Value()
	if err != nil {
		log.Printf("⚠️  Could not read back text: %v", err)
	} else {
		fmt.Printf("✅ Read back text: %q\n", value)
	}

	// Press Hotkey (Select All: Ctrl+A)
	fmt.Println("👉 Testing Hotkey (Ctrl+A)...")
	if err := edit.PressHotkey(winput.KeyCtrl, winput.KeyA); err != nil {
		log.Printf("⚠️  PressHotkey failed: %v", err)
	}

	// Mouse Click (Right Click context menu inside Edit control)
	fmt.Println("👉 Testing Right Click...")
	if err := edit.ClickRight(100, 100); err != nil {
		log.Printf("⚠️  ClickRight failed: %v", err)
	}

	fmt.Println("=== Done ===")
}
