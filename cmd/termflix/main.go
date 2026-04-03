package main

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/log"

	"github.com/example/termflix/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "termflix:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	logger := log.NewWithOptions(os.Stderr, log.Options{
		Prefix: "termflix",
		Level:  log.InfoLevel,
	})

	a, err := app.New(ctx, os.Args[1:], logger)
	if err != nil {
		return err
	}

	return a.Run(ctx)
}

