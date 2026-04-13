package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rpdg/winput/internal/mcpadapter"
)

func main() {
	allowMutations := flag.Bool("allow-mutations", false, "allow state-changing tools such as click and type_text")
	allowSensitive := flag.Bool("allow-sensitive", false, "allow sensitive tools such as switch_backend")
	flag.Parse()

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
