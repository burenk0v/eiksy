package connections

import "context"

// Protocol identifies a transport supported by Eiksy.
type Protocol string

const (
	ProtocolSSH   Protocol = "ssh"
	ProtocolSFTP  Protocol = "sftp"
	ProtocolRDP   Protocol = "rdp"
	ProtocolLocal Protocol = "local"
)

// Profile contains non-secret connection metadata. Credential material is
// resolved by the application and is deliberately outside this abstraction.
type Profile struct {
	ID            string
	Name          string
	Protocol      Protocol
	Host          string
	Port          int
	Username      string
	CredentialRef string
	Options       map[string]string
}

// Connection is the common lifecycle boundary for infrastructure transports.
type Connection interface {
	Connect(context.Context, Profile) error
	Disconnect(context.Context) error
	Connected() bool
}

// CommandExecutor is separate from Connection because not every transport
// supports command execution. Capability checks belong above this interface.
type CommandExecutor interface {
	Exec(context.Context, string) (CommandResult, error)
}

type CommandResult struct {
	Success    bool
	ExitCode   int
	Stdout     string
	Stderr     string
	DurationMs int64
}
