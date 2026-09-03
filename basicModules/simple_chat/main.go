//go:build !debug

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	sdk "tripod311/familiar-sdk"
)

var connector *sdk.RPCConnector
var server *Server

func main() {
	connector = sdk.NewConnector(
		"simple_chat",
		os.Stdin,
		os.Stdout,
	)

	connector.On("packetReceived", ProcessPacket)
	connector.On("closed", Shutdown)

	connector.Start()
	connector.Wait()
}

func Shutdown(event *sdk.Event) {
	fmt.Fprintf(os.Stderr, "UI stopped")
	if server != nil {
		server.Stop()
	}
}

func ProcessPacket(event *sdk.Event) {
	packet := event.Data.(sdk.RPCPacket)

	fmt.Fprint(os.Stderr, "PACKET RECV")

	switch packet.Method {
	case "load":
		server = NewServer(packet.Params, SendRequest)

		if err := server.Start(); err != nil {
			connector.Respond(packet.ID, nil, &sdk.RPCError{
				Code:    -32000,
				Message: err.Error(),
			})
			return
		}

		connector.Respond(packet.ID, nil, nil)
	case "unload":
		if server != nil {
			server.Stop()
		}
		connector.Respond(packet.ID, nil, nil)
	default:
		fmt.Fprintf(os.Stderr, "Unknown RPC method: %s", packet.Method)
		connector.Respond(packet.ID, nil, &sdk.RPCError{
			Code:    1,
			Message: fmt.Sprintf("Unknown RPC method: %s", packet.Method),
		})
	}
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

	resChan, err := connector.Send("modelRequest", data)
	if err != nil {
		return "", fmt.Errorf("send model request: %w", err)
	}

	response, ok := <-resChan
	if !ok {
		return "", errors.New(
			"RPC connector closed before receiving model response",
		)
	}

	if response.Error != nil {
		return "", fmt.Errorf(
			"model request failed (%d): %s",
			response.Error.Code,
			response.Error.Message,
		)
	}

	result, ok := response.Result.(string)
	if !ok {
		return "", fmt.Errorf(
			"unexpected model response type: %T",
			response.Result,
		)
	}

	return result, nil
}
