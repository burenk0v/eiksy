package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"eiksy/internal/cli"
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

	if version != "" {
		return version
	}

	return "dev"
}

func main() {
	if handled, exitCode := cli.Run(os.Args[1:], resolveReleaseVersion(), os.Stdout, os.Stderr, func(config cli.TUIConfig) error {
		return tui.Run(tui.Config{AltScreen: config.AltScreen, InitialView: config.InitialView})
	}); handled {
		os.Exit(exitCode)
	}

	_, _ = fmt.Fprintln(os.Stderr, "eiksy-cli: no command specified")
	os.Exit(2)
}
