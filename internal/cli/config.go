package cli

import "fmt"

// TUIConfig contains presentation-only options for the terminal UI.
type TUIConfig struct {
	AltScreen   bool
	InitialView string
}

func defaultTUIConfig() TUIConfig {
	return TUIConfig{AltScreen: true, InitialView: "chat"}
}

func parseTUIArgs(args []string) (TUIConfig, error) {
	config := defaultTUIConfig()
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--no-alt-screen":
			config.AltScreen = false
		case "--view":
			if i+1 >= len(args) {
				return TUIConfig{}, fmt.Errorf("--view requires a value")
			}
			i++
			view := args[i]
			switch view {
			case "chat", "terminal", "files", "tools":
				config.InitialView = view
			default:
				return TUIConfig{}, fmt.Errorf("invalid --view %q (want chat, terminal, files, or tools)", view)
			}
		default:
			return TUIConfig{}, fmt.Errorf("unknown tui option %q", args[i])
		}
	}
	return config, nil
}
