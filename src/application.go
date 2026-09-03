package main

import (
	"fmt"
	"os"
	"sync"
	sdk "tripod311/familiar-sdk"
)

type Application struct {
	Model   *Model
	Main    *Module
	Helpers map[string]*Module

	done        chan struct{}
	stopOnce    sync.Once
	cleanupOnce sync.Once
}

func NewApplication() *Application {
	return &Application{
		done: make(chan struct{}),
	}
}

func (app *Application) Cleanup() {
	app.Stop()

	app.cleanupOnce.Do(func() {
		fmt.Fprintln(os.Stderr, "Stopping Familiar...")

		if app.Main != nil {
			app.Main.Stop()
		}

		for _, module := range app.Helpers {
			if module != nil {
				module.Stop()
			}
		}

		if app.Model != nil {
			app.Model.Stop()
		}

		fmt.Fprintln(os.Stderr, "Familiar stopped")
	})
}

func (app *Application) ProcessRequest(rpc sdk.RPCPacket) error {
	switch rpc.Method {
	case "log":
		fmt.Println(string(rpc.Params))
	case "useHelper":
	case "useModel":
	}

	return nil
}

func (app *Application) Stop() {
	app.stopOnce.Do(func() {
		close(app.done)
	})
}

func (app *Application) ProcessMainEvent(event *sdk.Event) {
	packet := event.Data.(sdk.RPCPacket)

	switch packet.Method {
	case "modelRequest":
	case "moduleRequest":
	}
}

func (app *Application) ProcessModuleClosed(event *sdk.Event) {
	select {
	case <-app.done:
		// Application already stopping
		return
	default:
	}

	reason, ok := event.Data.(string)
	if !ok {
		reason = "unknown reason"
	}

	fmt.Fprintf(
		os.Stderr,
		"Module unexpectedly closed: %s\n",
		reason,
	)

	app.Stop()
}
