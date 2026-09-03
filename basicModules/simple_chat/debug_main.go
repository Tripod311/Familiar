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
)

var connector *RPCConnector
var hostConnector *RPCConnector
var server *Server

func main() {
	moduleStream, hostStream := net.Pipe()

	// Сторона тестируемого UI-модуля.
	connector = NewConnector(
		"simple_chat",
		moduleStream,
		moduleStream,
	)

	hostConnector = NewConnector(
		"debug_host",
		hostStream,
		hostStream,
	)

	connector.On("packetReceived", ProcessPacket)

	hostConnector.On("packetReceived", ProcessHostPacket)

	connector.Start()
	hostConnector.Start()

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

func ProcessPacket(event *Event) {
	packet := event.Data.(RPCPacket)

	fmt.Fprint(os.Stderr, "PACKET RECV")

	switch packet.Method {
	case "load":
		server = NewServer(packet.Params, SendRequest)

		if err := server.Start(); err != nil {
			connector.Respond(packet.ID, nil, &RPCError{
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
		connector.Respond(packet.ID, nil, &RPCError{
			Code:    1,
			Message: fmt.Sprintf("Unknown RPC method: %s", packet.Method),
		})
	}
}

func SendRequest(message string) (string, error) {
	req := []Message{
		{
			Role:    RoleUser,
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

func ProcessHostPacket(event *Event) {
	packet, ok := event.Data.(RPCPacket)
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
		var messages []Message

		if err := json.Unmarshal(packet.Params, &messages); err != nil {
			hostConnector.Respond(
				packet.ID,
				nil,
				&RPCError{
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

		hostConnector.Respond(
			packet.ID,
			"This is a mock model response.",
			nil,
		)

	default:
		fmt.Fprintf(
			os.Stderr,
			"UNKNOWN MODULE REQUEST: %s\n",
			packet.Method,
		)

		hostConnector.Respond(
			packet.ID,
			nil,
			&RPCError{
				Code:    -32601,
				Message: "method not found",
			},
		)
	}
}

func UnloadDebugModule() error {
	responseChan, err := hostConnector.Send(
		"unload",
		nil,
	)
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
	responseChan <-chan RPCPacket,
	timeout time.Duration,
) (RPCPacket, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case response, ok := <-responseChan:
		if !ok {
			return RPCPacket{}, errors.New(
				"connector closed before response",
			)
		}

		return response, nil

	case <-timer.C:
		return RPCPacket{}, errors.New(
			"RPC response timeout",
		)
	}
}

func ShutdownDebug() {
	if connector != nil {
		connector.Stop()
	}

	if hostConnector != nil {
		hostConnector.Stop()
	}
}
