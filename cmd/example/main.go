package main

import (
	"fmt"
	"winput"
	"time"
)

func main() {
	fmt.Println("winput example")
	
	// Example usage logic
	w, err := winput.FindByTitle("Untitled - Notepad")
	if err != nil {
		// Try localized or class
		w, err = winput.FindByClass("Notepad")
	}
	
	if err != nil {
		fmt.Println("Notepad not found, skipping interaction test.")
		fmt.Println("Error:", err)
		return
	}
	
	fmt.Println("Found Notepad window:", w.HWND)
	
	// Optional: Switch to HID Backend (requires Interception driver installed)
	// winput.SetBackend(winput.BackendHID)
	// Note: Initialization error will occur on first interaction if driver is missing.
	
	if err := winput.EnablePerMonitorDPI(); err != nil {
		fmt.Println("DPI Enable Error:", err)
	}
	
	dpi, err := w.DPI()
	fmt.Println("Window DPI:", dpi, err)
	
	w.Click(100, 100)
	w.Type("Hello via winput!")
	w.Press(winput.KeyEnter)
	
	time.Sleep(1 * time.Second)
	fmt.Println("Done.")
}
