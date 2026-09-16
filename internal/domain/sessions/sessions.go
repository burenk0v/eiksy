package sessions

// Profile contains only non-secret session metadata. Credential material lives
// exclusively in secure storage and is resolved by the application at runtime.
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
	HasPassword      bool              `json:"hasPassword"`
	HasKeyPassphrase bool              `json:"hasKeyPassphrase"`
	SecretRef        string            `json:"secretRef,omitempty"`
	Options          map[string]string `json:"options,omitempty"`
	LastLaunchedAt   string            `json:"lastLaunchedAt,omitempty"`
}

// ProfileInput is the Wails API DTO. Secret values exist only for the duration
// of the create/update request and are written directly to secure storage.
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

func (p ProfileInput) Metadata() Profile {
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