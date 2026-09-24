package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"eiksy/internal/app"
	"eiksy/internal/cli"
	"eiksy/internal/sftp"
	"eiksy/internal/ssh"
	"eiksy/internal/storage/disk"
	"eiksy/internal/storage/memory"
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

func newBackend() *app.Service {
	store, err := disk.NewStore()
	if err != nil {
		service := app.NewService(memory.NewStore(), ssh.NewManager(), sftp.NewManager())
		service.SetRuntimeContext(context.Background(), func(string, ...interface{}) {})
		return service
	}
	service := app.NewService(store, ssh.NewManager(), sftp.NewManager())
	service.SetRuntimeContext(context.Background(), func(string, ...interface{}) {})
	return service
}

func main() {
	if len(os.Args) == 1 {
		if err := tui.Run(tui.Config{AltScreen: true, InitialView: "sessions", Backend: newBackend()}); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "eiksy-cli: tui: %v
", err)
			os.Exit(1)
		}
		return
	}

	if handled, exitCode := cli.Run(os.Args[1:], resolveReleaseVersion(), os.Stdout, os.Stderr); handled {
		os.Exit(exitCode)
	}

	_, _ = fmt.Fprintln(os.Stderr, "eiksy-cli: no command specified")
	os.Exit(2)
}
