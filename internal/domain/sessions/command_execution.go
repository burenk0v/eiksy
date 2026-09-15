package sessions

// CommandExecutionErrorType classifies a command execution failure so callers
// do not need to parse human-readable error messages.
type CommandExecutionErrorType string

const (
	CommandExecutionErrorConnection CommandExecutionErrorType = "ssh_connection_error"
	CommandExecutionErrorTimeout    CommandExecutionErrorType = "timeout"
	CommandExecutionErrorNonZero    CommandExecutionErrorType = "non_zero_exit"
	CommandExecutionErrorInvalid    CommandExecutionErrorType = "invalid_request"
	CommandExecutionErrorInternal   CommandExecutionErrorType = "execution_error"
)

// CommandExecutionResult is the bounded, structured result of one non-interactive
// command executed through an existing SSH session.
type CommandExecutionResult struct {
	Success    bool                       `json:"success"`
	ExitCode   int                        `json:"exitCode"`
	Stdout     string                     `json:"stdout,omitempty"`
	Stderr     string                     `json:"stderr,omitempty"`
	DurationMs int64                      `json:"durationMs"`
	ErrorType  CommandExecutionErrorType  `json:"errorType,omitempty"`
	Error      string                     `json:"error,omitempty"`
}
