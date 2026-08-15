package engine

import "fmt"

type Chat struct {
	ID              string    `json:"-"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	ModelID         string    `json:"model"`
	EngineID        string    `json:"engine"`
	Plugins         []string  `json:"plugins"`
	Model           *Model    `json:"-"`
	pluginInstances []*Plugin `json:"-"`
}

func NewChat() *Chat {
	result := Chat{}
	result.Plugins = make([]string, 0)
	result.pluginInstances = make([]*Plugin, 0)
	return &result
}

func (chat *Chat) SetModel(model *Model) {
	chat.Model = model
	<-model.IsReady
	chat.Model.Server.On("BeforeRequest", chat.beforeRequest)
	chat.Model.Server.On("AfterRequest", chat.afterRequest)
}

func (chat *Chat) LoadPlugins() {

}

func (chat *Chat) Close() {
	for _, plugin := range chat.pluginInstances {
		plugin.Unload()
	}
}

func (chat *Chat) Request(message string) (string, error) {
	response, err := chat.Model.Server.Request(message)

	if err != nil {
		return "", fmt.Errorf("Request failed: %s", err)
	}

	return response, nil
}

func (chat *Chat) beforeRequest(event *Event) {
	for _, plugin := range chat.pluginInstances {
		plugin.BeforeRequest(event)
	}
}

func (chat *Chat) afterRequest(event *Event) {
	for _, plugin := range chat.pluginInstances {
		plugin.AfterRequest(event)
	}
}
