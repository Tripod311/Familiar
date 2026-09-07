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

func NewApplication(model *Model, mainModule *Module, helpers map[string]*Module) *Application {
	return &Application{
		Model:   model,
		Main:    mainModule,
		Helpers: helpers,
		done:    make(chan struct{}),
	}
}

func (app *Application) Launch(mainConf *json.RawMessage, helpersConf map[string]*json.RawMessage) error {
	// attach listeners

	app.Main.On("packetReceived", app.ProcessMainEvent)
	app.Main.On("closed", app.ProcessModuleClosed)

	for _, module := range app.Helpers {
		module.On("packetReceived", func(event *sdk.Event) {
			app.ProcessHelperEvent(module, event)
		})
		module.On("closed", app.ProcessModuleClosed)
	}

	// start modules

	if err := app.Main.Start(); err != nil {
		return fmt.Errorf(
			"module %s failed to start: %w",
			app.Main.Name,
			err,
		)
	}

	for key, module := range app.Helpers {
		if err := module.Start(); err != nil {
			return fmt.Errorf(
				"helper %s (%s) failed to start: %w",
				key,
				module.Name,
				err,
			)
		}
	}

	// send load packets

	for key, module := range app.Helpers {
		conf, exist := helpersConf[key]
		if !exist || conf == nil || *conf == nil {
			return fmt.Errorf("Corrupted module configuration: %s", module.Name)
		}

		response, err := module.Send(
			"load",
			*conf,
		)
		if err != nil {
			return fmt.Errorf(
				"helper %s (%s) failed to load: %w",
				key,
				module.Name,
				err,
			)
		}

		if response.Error != nil {
			return fmt.Errorf(
				"helper %s (%s) failed to load: (%d) %s",
				key,
				module.Name,
				response.Error.Code,
				response.Error.Message,
			)
		}
	}

	if mainConf == nil || *mainConf == nil {
		return fmt.Errorf("Corrupted module configuration: %s", app.Main.Name)
	}

	response, err := app.Main.Send(
		"load",
		*mainConf,
	)
	if err != nil {
		return fmt.Errorf(
			"module %s failed to load: %w",
			app.Main.Name,
			err,
		)
	}

	if response.Error != nil {
		return fmt.Errorf(
			"module %s failed to load: (%d) %s",
			app.Main.Name,
			response.Error.Code,
			response.Error.Message,
		)
	}

	// collect functions

	for key, module := range app.Helpers {
		err := module.GatherFunctions()
		if err != nil {
			return fmt.Errorf(
				"helper %s (%s) failed to gather functions: %w",
				key,
				module.Name,
				err,
			)
		}
	}

	err = app.Main.GatherFunctions()
	if err != nil {
		return fmt.Errorf(
			"module %s failed to gather functions: %s",
			app.Main.Name,
			err,
		)
	}

	return nil
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
		app.modelRequestMain(&packet)
	case "moduleRequest":
		app.moduleRequest(app.Main, &packet)
	default:
		app.Main.Respond(packet.ID, nil, &sdk.RPCError{
			Code:    1,
			Message: fmt.Sprintf("Unknown method: %s", packet.Method),
		})
	}
}

func (app *Application) ProcessHelperEvent(module *Module, event *sdk.Event) {
	packet := event.Data.(sdk.RPCPacket)

	switch packet.Method {
	case "modelRequest":
		res, err := app.modelRequestHelper(module, &packet)
		if err != nil {
			module.Respond(packet.ID, nil, &sdk.RPCError{
				Code:    1,
				Message: fmt.Sprintf("Packet processing error: %s", err),
			})
			return
		}

		if err := module.Respond(packet.ID, res, nil); err != nil {
			fmt.Printf("Error on response: %s", err)
		}

	default:
		module.Respond(packet.ID, nil, &sdk.RPCError{
			Code:    1,
			Message: fmt.Sprintf("Unknown method: %s", packet.Method),
		})
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

func (app *Application) modelRequestMain(packet *sdk.RPCPacket) {
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
}

func (app *Application) modelRequestHelper(
	invoker *Module,
	packet *sdk.RPCPacket,
) (json.RawMessage, error) {
	var req sdk.ServerRequest

	if err := json.Unmarshal(packet.Params, &req); err != nil {
		return nil, fmt.Errorf(
			"helper model request deserialization error: %w",
			err,
		)
	}

	for i := range req.Tools {
		req.Tools[i].Call = invoker.CallFunction
	}

	res, err := app.Model.Request(&req)
	if err != nil {
		return nil, fmt.Errorf(
			"helper model request failed: %w",
			err,
		)
	}

	bytes, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf(
			"helper model response serialization error: %w",
			err,
		)
	}

	return bytes, nil
}

func (app *Application) moduleRequest(invoker *Module, packet *sdk.RPCPacket) {
	var moduleRequest sdk.ModuleRequest

	err := json.Unmarshal(packet.Params, &moduleRequest)
	if err != nil {
		invoker.Respond(packet.ID, nil, &sdk.RPCError{
			Code:    1,
			Message: fmt.Sprintf("Invalid module request: %s", err),
		})
		return
	}

	helper, exists := app.Helpers[moduleRequest.Module]
	if !exists {
		invoker.Respond(packet.ID, nil, &sdk.RPCError{
			Code:    1,
			Message: fmt.Sprintf("Module not found: %s", moduleRequest.Module),
		})
		return
	}

	res, err := helper.Send(moduleRequest.Method, moduleRequest.Params)
	if err != nil {
		invoker.Respond(packet.ID, nil, &sdk.RPCError{
			Code:    1,
			Message: fmt.Sprintf("Module returned error: %s", err),
		})
		return
	} else if res.Error != nil {
		invoker.Respond(packet.ID, nil, res.Error)
	} else {
		invoker.Respond(packet.ID, res.Result, nil)
	}
}
