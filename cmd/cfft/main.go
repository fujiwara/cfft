package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	app "github.com/fujiwara/cfft"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx); err != nil {
		log.Printf("[error] %s", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	return app.RunCLI(ctx, os.Args[1:])
}
