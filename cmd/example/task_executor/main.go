package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/rpdg/winput"
)

func main() {
	fmt.Println("=== winput: Task Executor & Environment Probe Example ===")
	fmt.Println("This example demonstrates task-level fixed-backend executors and non-intrusive environment probes.")

	// 1. Discover target window
	w, err := winput.FindByClass("Notepad")
	if err != nil {
		log.Println("❌ Notepad not found. Please open Notepad to run this demo.")
		return
	}
	fmt.Printf("✅ Found target window: %x\n", w.HWND)

	// 2. Probe Target Environment Facts (read-only, does not inject input)
	targetReport, err := winput.ProbeTarget(w)
	if err != nil {
		if errors.Is(err, winput.ErrTargetChanged) {
			log.Printf("⚠️  Target changed during probe: %v", err)
		} else {
			log.Printf("⚠️  Target probe failed: %v", err)
		}
	} else {
		fmt.Printf("🔍 Target Process: %s, Class: %s\n", targetReport.Features.ProcessName, targetReport.Features.TargetClass)
		fmt.Printf("🔍 Integrity: %s, Caller UIAccess: %s\n", targetReport.Features.IntegrityRelation, targetReport.Features.CallerUIAccess)
		for _, w := range targetReport.Warnings {
			fmt.Printf("   ⚠️ Warning: %s\n", w)
		}
	}

	// 3. Probe HID Hardware Driver Readiness (does not modify global backend)
	hidStatus := winput.ProbeHID()
	fmt.Printf("🔍 Local HID Backend Status: %s\n", hidStatus)

	// 4. Create Task-Level Executor bound to BackendMessage
	// The executor retains BackendMessage regardless of any later SetBackend calls.
	executor, err := winput.NewInput(winput.BackendMessage)
	if err != nil {
		log.Fatalf("❌ Failed to create Input executor: %v", err)
	}

	// 5. Window-targeted Facade
	winInput, err := executor.ForWindow(w)
	if err != nil {
		log.Fatalf("❌ Failed to create WindowInput facade: %v", err)
	}

	fmt.Println("👉 Typing via WindowInput facade...")
	if err := winInput.Type("Hello from Task-Level WindowInput!\n"); err != nil {
		log.Printf("⚠️  Type error: %v", err)
	}

	// 6. Desktop / Windowless Facade & Clipboard Paste
	deskInput, err := executor.Desktop()
	if err != nil {
		log.Fatalf("❌ Failed to create DesktopInput facade: %v", err)
	}

	fmt.Println("👉 Pasting text via DesktopInput.PasteByClipboard...")
	// PasteByClipboard injects Ctrl+V and safely restores previous clipboard Unicode text
	if err := deskInput.PasteByClipboard("Pasted via fixed-backend clipboard facade.\n"); err != nil {
		log.Printf("⚠️  PasteByClipboard error: %v", err)
	}

	fmt.Println("=== Done ===")
}
