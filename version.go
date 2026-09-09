package main

import (
	"runtime/debug"
	"strings"
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
