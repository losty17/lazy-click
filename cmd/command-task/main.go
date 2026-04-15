package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"command-task/internal/app"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	runtime, err := app.Bootstrap(ctx)
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	if err := runtime.Run(ctx); err != nil {
		log.Fatalf("runtime failed: %v", err)
	}
}
