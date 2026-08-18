package familiar

import (
	"fmt"
	"log"
	"time"
)

type Model struct {
	Name   string
	Server *Server `json:"-"`

	IsReady chan struct{} `json:"-"`
}

func NewModel() *Model {
	result := Model{}
	result.IsReady = make(chan struct{})
	return &result
}

func (model *Model) Start(startTimeout uint, verbose bool) {
	model.Server.On("Started", model.serverStarted)
	model.Server.On("Stopped", model.serverDown)

	model.Server.Start(time.Second*time.Duration(startTimeout), verbose)
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
	log.Fatal("Model %s is down.", model.Name)
}

func (model *Model) Request(req *ServerRequest) (string, error) {
	response, err := model.Server.Request(req)

	if err != nil {
		return "", fmt.Errorf("Request failed: %s", err)
	}

	return response, nil
}
