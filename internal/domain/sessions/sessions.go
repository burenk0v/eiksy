package sessions

import (
	"encoding/base64"
)

type Profile struct {
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
	SecretRef      string            `json:"secretRef,omitempty"`
	Options        map[string]string `json:"options,omitempty"`
	LastLaunchedAt string            `json:"lastLaunchedAt,omitempty"`
}

// ProfileInput is used as the parameter type for CreateSessionProfile in the
// Wails API. Unlike Profile, it exposes the Password field in JSON so that
// Wails generates the corresponding TypeScript property. The password is never
// persisted to disk; it is only used in-flight during the IPC call.
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
	SecretRef      string            `json:"secretRef,omitempty"`
	Options        map[string]string `json:"options,omitempty"`
	LastLaunchedAt string            `json:"lastLaunchedAt,omitempty"`
}

// ToProfile converts a ProfileInput to a Profile, copying all fields including
// the password (which is not persisted when the Profile is later saved).
func (p ProfileInput) ToProfile() Profile {
	password, err := base64.StdEncoding.DecodeString(p.Password)
	if err != nil {
		password = make([]byte, 1)
	}
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
		Password:       string(password),
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
