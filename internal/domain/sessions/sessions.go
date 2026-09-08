package sessions

// ts_type string
type EncryptedString string

type Profile struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Group            string            `json:"group"`
	Tags             []string          `json:"tags"`
	Favorite         bool              `json:"favorite"`
	ProtocolID       string            `json:"protocolId"`
	Host             string            `json:"host"`
	Port             int               `json:"port"`
	Username         string            `json:"username"`
	Password         EncryptedString   `json:"-"`
	KeyPassphrase    EncryptedString   `json:"-"`
	HasPassword      bool              `json:"hasPassword"`
	HasKeyPassphrase bool              `json:"hasKeyPassphrase"`
	SecretRef        string            `json:"secretRef,omitempty"`
	Options          map[string]string `json:"options,omitempty"`
	LastLaunchedAt   string            `json:"lastLaunchedAt,omitempty"`
}

// ProfileInput is used as the parameter type for CreateSessionProfile in the
// Wails API. Unlike Profile, it exposes secret fields in JSON so that Wails
// generates the corresponding TypeScript properties. Those values are persisted
// only in the secure secret store.
type ProfileInput struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Group          string            `json:"group"`
	Tags           []string          `json:"tags"`
	Favorite       bool              `json:"favorite"`
	ProtocolID     string            `json:"protocolId"`
	Host           string            `json:"host"`
	Port           int               `json:"port"`
	Username       string            `json:"username"`
	Password       string            `json:"password,omitempty"`
	KeyPassphrase  string            `json:"keyPassphrase,omitempty"`
	SecretRef      string            `json:"secretRef,omitempty"`
	Options        map[string]string `json:"options,omitempty"`
	LastLaunchedAt string            `json:"lastLaunchedAt,omitempty"`
}

func (p ProfileInput) ToProfile() Profile {
	return Profile{
		ID:             p.ID,
		Name:           p.Name,
		Group:          p.Group,
		Tags:           p.Tags,
		Favorite:       p.Favorite,
		ProtocolID:     p.ProtocolID,
		Host:           p.Host,
		Port:           p.Port,
		Username:       p.Username,
		Password:       EncryptedString(p.Password),
		KeyPassphrase:  EncryptedString(p.KeyPassphrase),
		SecretRef:      p.SecretRef,
		Options:        p.Options,
		LastLaunchedAt: p.LastLaunchedAt,
	}
}

type HistoryEntry struct {
	ProfileID   string `json:"profileId"`
	ProfileName string `json:"profileName"`
	LaunchedAt  string `json:"launchedAt"`
}
