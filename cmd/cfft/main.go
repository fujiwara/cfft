package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/fujiwara/cfft"
)

var Version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfft.Version = Version
	return cfft.RunCLI(ctx, os.Args[1:])
}
