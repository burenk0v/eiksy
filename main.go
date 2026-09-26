package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"eiksy/internal/cli"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

const webviewUserDataDirName = "eiksy"

func webviewUserDataPath() string {
	if runtime.GOOS != "windows" {
		return ""
	}

	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return ""
	}
	return filepath.Join(configDir, webviewUserDataDirName)
}

func main() {
	if handled, exitCode := cli.Run(os.Args[1:], resolveReleaseVersion(), os.Stdout, os.Stderr); handled {
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
		Windows: &windows.Options{
			WebviewUserDataPath: webviewUserDataPath(),
		},
		OnStartup:    app.startup,
		OnDomReady:   app.restoreWindow,
		OnBeforeClose: app.beforeClose,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
