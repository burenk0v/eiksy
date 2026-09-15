package sessions

// CommandExecutionResult is the bounded, structured result of one non-interactive
// command executed through an existing SSH session.
type CommandExecutionResult struct {
	Success    bool   `json:"success"`
	ExitCode   int    `json:"exitCode"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}
