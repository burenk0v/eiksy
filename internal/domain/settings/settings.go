package settings

type WindowLayout struct {
	SidebarWidth   int `json:"sidebarWidth"`
	AssistantWidth int `json:"assistantWidth"`
}

type AppSettings struct {
	Theme            string       `json:"theme"`
	DefaultProtocol  string       `json:"defaultProtocol"`
	WindowLayout     WindowLayout `json:"windowLayout"`
	PromptBeforeAI   bool         `json:"promptBeforeAi"`
	AllowCloudModels bool         `json:"allowCloudModels"`
}
