package main

import (
	"cmp"
	"context"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/LaPingvino/lexington/cli"
)

// Version information, set by build flags (the release workflow); without
// them, what Go recorded: the module version for go install …@vX, the
// commit and its time for a build from a checkout.
var (
	version = ""
	commit  = ""
	date    = ""
)

// buildVersion fills in what the build flags left unset from the build info.
func buildVersion(bi *debug.BuildInfo, version, commit, date string) (string, string, string) {
	if bi != nil {
		if v := bi.Main.Version; version == "" && v != "" && v != "(devel)" {
			version = v
		}
		for _, kv := range bi.Settings {
			switch {
			case kv.Key == "vcs.revision" && commit == "":
				commit = kv.Value
			case kv.Key == "vcs.time" && date == "":
				date = kv.Value
			}
		}
	}
	return cmp.Or(version, "dev"), cmp.Or(commit, "unknown"), cmp.Or(date, "unknown")
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	start := time.Now()
	bi, _ := debug.ReadBuildInfo()
	cli.Version, cli.Commit, cli.Date = buildVersion(bi, version, commit, date)
	code := cli.Main(ctx, os.Args[1:])
	log.Printf("Conversion took %v", time.Since(start))
	cancel()
	os.Exit(code)
}
