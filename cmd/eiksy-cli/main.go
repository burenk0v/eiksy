package main

import (
		"fmt"
	"os"
	"runtime/debug"
	"strings"

	"eiksy/internal/app"
	"eiksy/internal/cli"
	"eiksy/internal/storage/disk"
	"eiksy/internal/tui"
)

var releaseVersion = "dev"

func resolveReleaseVersion() string {
	version := strings.TrimSpace(releaseVersion)
	if version != "" && version != "dev" {
		return version
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		if buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
			return buildInfo.Main.Version
		}
		for _, setting := range buildInfo.Settings {
			if setting.Key == "vcs.revision" && setting.Value != "" {
				shortRevision := setting.Value
				if len(shortRevision) > 7 {
					shortRevision = shortRevision[:7]
				}
				return "dev-" + shortRevision
			}
		}
	}

	return version
}

func newBackend() *app.Service {
	store, err := disk.NewStore()
	if err != nil {
		panic(fmt.Sprintf("eiksy-cli: initialize storage: %v", err))
	}
	return app.NewTUIService(store)
}

func main() {
	if len(os.Args) == 1 {
		if err := tui.Run(newBackend()); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "eiksy-cli: tui: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if handled, exitCode := cli.Run(os.Args[1:], resolveReleaseVersion(), os.Stdout, os.Stderr); handled {
		os.Exit(exitCode)
	}

	_, _ = fmt.Fprintln(os.Stderr, "eiksy-cli: unexpected command state")
	os.Exit(2)
}
