//go:build debug

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	sdk "tripod311/familiar-sdk"
)

var module *sdk.ExternalModule
var hostConnector *sdk.RPCConnector
var server *Server

func main() {
	moduleStream, hostStream := net.Pipe()

	module = sdk.NewExternalModuleWithIO(
		moduleStream,
		moduleStream,
	)

	module.LoadHandle = Setup
	module.UnloadHandle = Shutdown

	hostConnector = sdk.NewConnector(
		"debug_host",
		hostStream,
		hostStream,
	)

	hostConnector.On("packetReceived", ProcessHostPacket)
	hostConnector.Start()

	go module.Start()

	if err := LoadDebugModule(); err != nil {
		fmt.Fprintln(os.Stderr, "debug load failed:", err)
		ShutdownDebug()
		os.Exit(1)
	}

	fmt.Fprintln(
		os.Stderr,
		"Debug host started; press Ctrl+C to stop",
	)

	signals := make(chan os.Signal, 1)
	signal.Notify(
		signals,
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer signal.Stop(signals)

	<-signals

	fmt.Fprintln(os.Stderr, "shutdown signal received")

	if err := UnloadDebugModule(); err != nil {
		fmt.Fprintln(os.Stderr, "debug unload failed:", err)
	}

	ShutdownDebug()
}

func Setup(params json.RawMessage) (json.RawMessage, error) {
	server = NewServer(params, SendRequest)

	if err := server.Start(); err != nil {
		return nil, fmt.Errorf("server start error: %w", err)
	}

	return nil, nil
}

func Shutdown(params json.RawMessage) (json.RawMessage, error) {
	fmt.Fprintln(os.Stderr, "UI stopped")

	if server != nil {
		if err := server.Stop(); err != nil {
			return nil, fmt.Errorf("server stop error: %w", err)
		}
	}

	return nil, nil
}

func SendRequest(message string) (string, error) {
	request := []sdk.Message{
		{
			Role:    sdk.RoleUser,
			Content: message,
		},
	}

	data, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal model request: %w", err)
	}

	response, err := module.Send("modelRequest", data)
	if err != nil {
		return "", fmt.Errorf("send model request: %w", err)
	}

	var responseMessage sdk.Message

	if err := json.Unmarshal(response, &responseMessage); err != nil {
		return "", fmt.Errorf("decode model response: %w", err)
	}

	return responseMessage.Content, nil
}

func LoadDebugModule() error {
	params := json.RawMessage(`{
		"port": 8080
	}`)

	responseChan, err := hostConnector.Send("load", params)
	if err != nil {
		return fmt.Errorf("send load: %w", err)
	}

	response, err := WaitResponse(responseChan, 5*time.Second)
	if err != nil {
		return err
	}

	if response.Error != nil {
		return fmt.Errorf(
			"load failed (%d): %s",
			response.Error.Code,
			response.Error.Message,
		)
	}

	return nil
}

func ProcessHostPacket(event *sdk.Event) {
	packet, ok := event.Data.(sdk.RPCPacket)
	if !ok {
		fmt.Fprintf(
			os.Stderr,
			"unexpected event type: %T\n",
			event.Data,
		)
		return
	}

	switch packet.Method {
	case "modelRequest":
		ProcessModelRequest(packet)

	default:
		fmt.Fprintf(
			os.Stderr,
			"unknown module request: %s\n",
			packet.Method,
		)

		hostConnector.Respond(
			packet.ID,
			nil,
			&sdk.RPCError{
				Code:    -32601,
				Message: "method not found",
			},
		)
	}
}

func ProcessModelRequest(packet sdk.RPCPacket) {
	var messages []sdk.Message

	if err := json.Unmarshal(packet.Params, &messages); err != nil {
		hostConnector.Respond(
			packet.ID,
			nil,
			&sdk.RPCError{
				Code:    -32602,
				Message: "invalid model request",
			},
		)
		return
	}

	fmt.Fprintf(
		os.Stderr,
		"MODEL REQUEST: %+v\n",
		messages,
	)

	mockResponse, err := json.Marshal(sdk.Message{
		Role:    sdk.RoleAssistant,
		Content: "This is a mock model response.",
	})
	if err != nil {
		hostConnector.Respond(
			packet.ID,
			nil,
			&sdk.RPCError{
				Code:    -32603,
				Message: err.Error(),
			},
		)
		return
	}

	hostConnector.Respond(
		packet.ID,
		json.RawMessage(mockResponse),
		nil,
	)
}

func UnloadDebugModule() error {
	responseChan, err := hostConnector.Send("unload", nil)
	if err != nil {
		return fmt.Errorf("send unload: %w", err)
	}

	response, err := WaitResponse(responseChan, 5*time.Second)
	if err != nil {
		return err
	}

	if response.Error != nil {
		return fmt.Errorf(
			"unload failed (%d): %s",
			response.Error.Code,
			response.Error.Message,
		)
	}

	return nil
}

func WaitResponse(
	responseChan <-chan sdk.RPCPacket,
	timeout time.Duration,
) (sdk.RPCPacket, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case response, ok := <-responseChan:
		if !ok {
			return sdk.RPCPacket{}, errors.New(
				"connector closed before response",
			)
		}

		return response, nil

	case <-timer.C:
		return sdk.RPCPacket{}, errors.New(
			"RPC response timeout",
		)
	}
}

func ShutdownDebug() {
	if module != nil {
		module.Stop()
	}

	if hostConnector != nil {
		hostConnector.Stop()
	}
}
