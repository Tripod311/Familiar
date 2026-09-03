//go:build !debug

package main

import (
	"encoding/json"
	"fmt"
	"os"
	sdk "tripod311/familiar-sdk"
)

var module *sdk.ExternalModule
var server *Server

func main() {
	module = sdk.NewExternalModule()

	module.LoadHandle = Setup
	module.UnloadHandle = Shutdown

	module.Start()
}

func Shutdown(params json.RawMessage) (json.RawMessage, error) {
	fmt.Fprintf(os.Stderr, "UI stopped")
	if server != nil {
		server.Stop()
	}

	return nil, nil
}

func Setup(params json.RawMessage) (json.RawMessage, error) {
	server = NewServer(params, SendRequest)

	if err := server.Start(); err != nil {
		return nil, fmt.Errorf("Server start error: %s", err)
	}

	return nil, nil
}

func SendRequest(message string) (string, error) {
	req := []sdk.Message{
		{
			Role:    sdk.RoleUser,
			Content: message,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal model request: %w", err)
	}

	response, err := module.Send("modelRequest", data)
	if err != nil {
		return "", err
	}

	var resMsg sdk.Message

	err = json.Unmarshal(response, &resMsg)
	if err != nil {
		return "", err
	}

	return resMsg.Content, nil
}
