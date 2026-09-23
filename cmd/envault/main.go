package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/giovalgas/envault/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Execute(ctx, cli.NewApp(version), os.Args[1:])
}
