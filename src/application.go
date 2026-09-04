package main

import (
	"encoding/json"
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

func (app *Application) Stop() {
	app.stopOnce.Do(func() {
		close(app.done)
	})
}

func (app *Application) ProcessMainEvent(event *sdk.Event) {
	packet := event.Data.(sdk.RPCPacket)

	switch packet.Method {
	case "modelRequest":
		tools := make([]sdk.Tool, 0)

		for _, desc := range app.Main.Tools {
			tools = append(tools, sdk.Tool{
				Type:     "function",
				Function: desc,
				Call:     app.Main.CallFunction,
			})
		}

		for _, module := range app.Helpers {
			for _, desc := range module.Tools {
				tools = append(tools, sdk.Tool{
					Type:     "function",
					Function: desc,
					Call:     module.CallFunction,
				})
			}
		}

		var messages []sdk.Message

		err := json.Unmarshal(packet.Params, &messages)
		if err != nil {
			app.Main.Respond(packet.ID, nil, &sdk.RPCError{
				Code:    1,
				Message: fmt.Sprintf("Packet reading error: %s", err),
			})
			return
		}

		req := sdk.ServerRequest{
			Tools:    tools,
			Messages: messages,
		}

		res, err := app.Model.Request(&req)
		if err != nil {
			app.Main.Respond(packet.ID, nil, &sdk.RPCError{
				Code:    1,
				Message: fmt.Sprintf("Packet processing error: %s", err),
			})
			return
		}

		bytes, err := json.Marshal(res)
		if err != nil {
			app.Main.Respond(packet.ID, nil, &sdk.RPCError{
				Code:    1,
				Message: fmt.Sprintf("Packet response error: %s", err),
			})
			return
		}

		err = app.Main.Respond(packet.ID, bytes, nil)
		if err != nil {
			fmt.Printf("Error on response: %s", err)
		}
	case "moduleRequest":
		var moduleRequest sdk.ModuleRequest

		err := json.Unmarshal(packet.Params, &moduleRequest)
		if err != nil {
			app.Main.Respond(packet.ID, nil, &sdk.RPCError{
				Code:    1,
				Message: fmt.Sprintf("Invalid module request: %s", err),
			})
			return
		}

		helper, exists := app.Helpers[moduleRequest.Module]
		if !exists {
			app.Main.Respond(packet.ID, nil, &sdk.RPCError{
				Code:    1,
				Message: fmt.Sprintf("Module not found: %s", moduleRequest.Module),
			})
			return
		}

		res, err := helper.Send(moduleRequest.Method, moduleRequest.Params)
		if err != nil {
			app.Main.Respond(packet.ID, nil, &sdk.RPCError{
				Code:    1,
				Message: fmt.Sprintf("Module returned error: %s", err),
			})
			return
		} else {
			app.Main.Respond(packet.ID, res.Result, nil)
		}
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
