package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LaPingvino/lexington/cli"
)

// Version information (set by build flags)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	start := time.Now()
	cli.Version, cli.Commit, cli.Date = version, commit, date
	code := cli.Main(ctx, os.Args[1:])
	log.Printf("Conversion took %v", time.Since(start))
	cancel()
	os.Exit(code)
}
