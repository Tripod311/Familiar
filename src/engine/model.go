package engine

import (
	"fmt"
	"time"
)

type Model struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	FileName    string  `json:"filename"`
	Server      *Server `json:"-"`

	IsReady chan struct{} `json:"-"`
}

func NewModel() *Model {
	result := Model{}
	result.IsReady = make(chan struct{})
	return &result
}

func (model *Model) Start() {
	model.Server.On("Started", model.serverStarted)
	model.Server.On("Stopped", model.serverDown)

	model.Server.Start(time.Second * 30)
}

func (model *Model) Stop() {
	model.Server.ClearAll()
	model.Server.Stop()
}

func (model *Model) serverStarted(ev *Event) {
	fmt.Printf("Model %s has started", model.Name)
	close(model.IsReady)
}

func (model *Model) serverDown(ev *Event) {
	fmt.Printf("Model %s is down. Restarting...", model.Name)
	model.Server.Start(time.Second * 30)
}
