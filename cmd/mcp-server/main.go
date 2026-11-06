package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rpdg/winput"
	"github.com/rpdg/winput/internal/mcpadapter"
	"github.com/rpdg/winput/window"
)

func main() {
	allowMutations := flag.Bool("allow-mutations", false, "allow state-changing tools such as click and type_text")
	allowSensitive := flag.Bool("allow-sensitive", false, "allow sensitive tools such as switch_backend")
	flag.Parse()

	// capture_screen requires per-monitor DPI awareness. The success
	// criterion is the awareness check, not the nil return of
	// EnablePerMonitorDPI: the library falls back to system-aware
	// (SetProcessDPIAware) on old paths and returns nil while
	// IsPerMonitorDPIAware is still false. An already-aware process (for
	// example via an application manifest) makes the second
	// SetProcessDpiAwarenessContext call fail, so the warning must follow
	// the awareness check, never the error alone. Whatever the outcome, the
	// server keeps running and capture_screen fails with the library error
	// when awareness is missing (Q7).
	if !window.IsPerMonitorDPIAware() {
		if err := winput.EnablePerMonitorDPI(); err != nil || !window.IsPerMonitorDPIAware() {
			log.Printf("warning: per-monitor DPI awareness unavailable (err=%v); capture_screen may fail with a DPI error", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := mcpadapter.NewServer(mcpadapter.Config{
		AllowMutations: *allowMutations,
		AllowSensitive: *allowSensitive,
	})
	if err := server.Serve(ctx, os.Stdin, os.Stdout); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
