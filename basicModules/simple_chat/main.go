//go:build !debug

package main

import (
	"encoding/json"
	"fmt"
	"os"
	sdk "tripod311/familiar-sdk"
)

type UIConfig struct {
	Port      int    `json:"port"`
	ClientDir string `json:"clientDir"`
	History   string `json:"history"`
	Context   string `json:"context"`
}

var config UIConfig
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
	err := json.Unmarshal(params, &config)
	if err != nil {
		return nil, fmt.Errorf("Server config error: %s", err)
	}

	server = NewServer(config.Port, config.ClientDir, SendRequest)

	if err := server.Start(); err != nil {
		return nil, fmt.Errorf("Server start error: %s", err)
	}

	return nil, nil
}

func SendRequest(message string) (string, error) {
	// fetch context
	req, err := FetchContext()
	if err != nil {
		return "", err
	}

	// fetch history
	hist, err := FetchHistory()
	if err != nil {
		return "", err
	}

	req = append(req, hist...)

	req = append(req, sdk.Message{
		Role:    sdk.RoleUser,
		Content: message,
	})

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

	// append history
	err = AppendHistory(resMsg)
	if err != nil {
		return resMsg.Content, err
	}

	return resMsg.Content, nil
}

func FetchContext() ([]sdk.Message, error) {
	var result []sdk.Message

	if len(config.Context) > 0 {
		contextRequest := sdk.ModuleRequest{
			Module: config.Context,
			Method: "get_context",
		}
		bytes, err := json.Marshal(contextRequest)
		if err != nil {
			return nil, fmt.Errorf("Context request building error: %s", err)
		}

		raw, err := module.Send("moduleRequest", bytes)
		if err != nil {
			return nil, fmt.Errorf("Context request sending error: %s", err)
		}

		err = json.Unmarshal(raw, &result)
		if err != nil {
			return nil, fmt.Errorf("Context module response error: %s", err)
		}
	}

	return result, nil
}

func FetchHistory() ([]sdk.Message, error) {
	var result []sdk.Message

	if len(config.History) > 0 {
		historyRequest := sdk.ModuleRequest{
			Module: config.History,
			Method: "get",
		}
		bytes, err := json.Marshal(historyRequest)
		if err != nil {
			return nil, fmt.Errorf("History request building error: %s", err)
		}

		raw, err := module.Send("moduleRequest", bytes)
		if err != nil {
			return nil, fmt.Errorf("History request sending error: %s", err)
		}

		err = json.Unmarshal(raw, &result)
		if err != nil {
			return nil, fmt.Errorf("History module response error: %s", err)
		}
	}

	return result, nil
}

func AppendHistory(msg sdk.Message) error {
	if len(config.History) > 0 {
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("History append message serialization error: %s", err)
		}

		historyRequest := sdk.ModuleRequest{
			Module: config.History,
			Method: "append",
			Params: msgBytes,
		}
		bytes, err := json.Marshal(historyRequest)
		if err != nil {
			return fmt.Errorf("History request sending error: %s", err)
		}

		_, err = module.Send("moduleRequest", bytes)
		if err != nil {
			return fmt.Errorf("History request building error: %s", err)
		}
	}

	return nil
}
