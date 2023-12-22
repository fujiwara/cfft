package main

import (
	"context"
	"log"
	"os"

	app "github.com/fujiwara/cfft"
)

func main() {
	ctx := context.TODO()
	if err := run(ctx); err != nil {
		log.Printf("[error] %s", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	return app.Run(ctx)
}
