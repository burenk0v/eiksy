package main

import (
	"embed"
	"fmt"
	"os"

	"eiksy/internal/cli"
	"eiksy/internal/tui"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if handled, exitCode := cli.Run(os.Args[1:], resolveReleaseVersion(), os.Stdout, os.Stderr, tui.Run); handled {
		os.Exit(exitCode)
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:                    "Eiksy",
		Width:                    1440,
		Height:                   900,
		MinWidth:                 1200,
		MinHeight:                700,
		DisableResize:            false,
		Frameless:                false,
		EnableDefaultContextMenu: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 11, G: 18, B: 32, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
