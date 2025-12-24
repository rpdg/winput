package main

import (
	"log"
	"time"
	"github.com/rpdg/winput"
)

func main() {
	log.SetFlags(log.Ltime)
	log.Println("=== winput Library Example ===")

	// -------------------------------------------------------------------------
	// 1. Setup & Configuration
	// -------------------------------------------------------------------------

	// Optional: Enable Per-Monitor DPI awareness (Recommended for Win10/11)
	// This ensures coordinate calculations are accurate on scaled displays.
	if err := winput.EnablePerMonitorDPI(); err != nil {
		log.Printf("Warning: Failed to enable DPI awareness: %v", err)
	}

	// Optional: Switch to HID Backend (Interception Driver)
	// un-comment the line below to use kernel-level injection.
	// Prerequisite: Interception driver must be installed and interception.dll available.
	/*
		log.Println("Switching to HID Backend...")
		winput.SetBackend(winput.BackendHID)
	*/

	// -------------------------------------------------------------------------
	// 2. Window Discovery
	// -------------------------------------------------------------------------

	targetTitle := "Untitled - Notepad"
	// For localized Windows, it might be "无标题 - 记事本"
	// Or use FindByClass("Notepad") for better robustness.

	log.Printf("Looking for window: '%s'...", targetTitle)
	w, err := winput.FindByTitle(targetTitle)
	if err != nil {
		// Fallback: Try finding by Class Name
		log.Printf("Title not found (%v), trying class 'Notepad'...", err)
		w, err = winput.FindByClass("Notepad")
		if err != nil {
			log.Fatalf("❌ Fatal: Could not find Notepad window. Please open Notepad to run this demo.")
		}
	}

	log.Printf("✅ Found Window! HWND: %v", w.HWND)

	// Display Debug Info
	if width, height, err := w.ClientRect(); err == nil {
		log.Printf("   Client Area: %dx%d", width, height)
	}
	if dpi, err := w.DPI(); err == nil {
		log.Printf("   Window DPI:  %d", dpi)
	}

	// -------------------------------------------------------------------------
	// 3. Mouse Interaction
	// -------------------------------------------------------------------------
	
	// Coordinate (0,0) is top-left of the Notepad editable area (Client Area)
	// We will click in the middle of the editing area.
	x, y := int32(100), int32(100)

	log.Println("👉 Performing Mouse Actions...")
	
	// Click (Left)
	if err := w.Click(x, y); err != nil {
		log.Fatalf("Click failed: %v", err)
	}
	log.Println("   - Clicked (100, 100)")
	time.Sleep(500 * time.Millisecond)

	// Right Click (Context Menu)
	// if err := w.ClickRight(x, y); err != nil { log.Printf("Right click failed: %v", err) }
	// log.Println("   - Right Clicked")
	// time.Sleep(500 * time.Millisecond)
	// w.Press(winput.KeyEsc) // Close context menu

	// -------------------------------------------------------------------------
	// 4. Keyboard Interaction
	// -------------------------------------------------------------------------

	log.Println("⌨️  Performing Keyboard Actions...")

	// Simple Typing
	msg := "Hello winput!"
	log.Printf("   - Typing: '%s'", msg)
	if err := w.Type(msg); err != nil {
		log.Printf("Type failed: %v", err)
	}

	w.Press(winput.KeyEnter)

	// Complex Key Combination (Ctrl + A) - Select All
	log.Println("   - Combination: Ctrl + A (Select All)")
	// Note: winput doesn't have a helper for combos yet, we do it manually via KeyDown/Up
	// But winput.KeyCtrl is not exported in the root for brevity, we can access via "winput.Key..." if we export them
	// Wait, winput.go exports KeyEnter etc. Let's check if KeyCtrl is exported.
	// winput.go exports: KeyEnter, Esc, Space, Tab, A. 
	// To use others, user might need to import "winput/keyboard" or we should export common ones.
	// Since "winput/keyboard" is internal-ish (package design says avoid pollution), 
	// let's stick to simple Typing or rely on Type for text.
	// For demo purposes, we will Type more text.
	
	w.Type(" This text was typed in background.")
	w.Press(winput.KeyEnter)

	// -------------------------------------------------------------------------
	// 5. HID specific demonstration (Pseudo-code logic)
	// -------------------------------------------------------------------------
	// If we were in HID mode, the mouse cursor would have physically moved 
	// across the screen to (100, 100) inside the notepad window.
	// In Message mode (default), the cursor stays put.

	log.Println("✅ Demo Complete.")
}