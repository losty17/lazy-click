package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"lazy-click/internal/app"
)

func main() {
	showVersion := flag.Bool("version", false, "show version")
	showVersionShort := flag.Bool("v", false, "show version")
	flag.Parse()

	if *showVersion || *showVersionShort {
		fmt.Printf("lazy-click %s\n", app.Version)
		return
	}

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
