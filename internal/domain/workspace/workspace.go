package workspace

type SidebarSection struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type Tab struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ProtocolID  string `json:"protocolId"`
	ProfileID   string `json:"profileId"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type Layout struct {
	SidebarSections []SidebarSection `json:"sidebarSections"`
	ActiveTabID     string           `json:"activeTabId,omitempty"`
}

type Event struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Subject string `json:"subject"`
	At      string `json:"at"`
}
