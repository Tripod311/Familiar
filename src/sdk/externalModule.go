package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type ExternalModule struct {
	connector *RPCConnector

	LoadHandle     func(params json.RawMessage) (json.RawMessage, error)
	UnloadHandle   func(params json.RawMessage) (json.RawMessage, error)
	MethodHandle   func(method string, params json.RawMessage) (json.RawMessage, error)
	ShutdownHandle func() error
}

func NewExternalModule() *ExternalModule {
	result := ExternalModule{
		connector: NewConnector("", os.Stdin, os.Stdout),
	}

	result.connector.On("packetReceived", result.ProcessPacket)
	result.connector.On("closed", result.Shutdown)

	return &result
}

func NewExternalModuleWithIO(
	reader io.ReadCloser,
	writer io.Writer,
) *ExternalModule {
	result := &ExternalModule{
		connector: NewConnector("", reader, writer),
	}

	result.connector.On("packetReceived", result.ProcessPacket)
	result.connector.On("closed", result.Shutdown)

	return result
}

func (em *ExternalModule) Start() {
	em.connector.Start()
	em.connector.Wait()
}

func (em *ExternalModule) ProcessPacket(event *Event) {
	packet := event.Data.(RPCPacket)

	switch packet.Method {
	case "load":
		if em.LoadHandle != nil {
			result, err := em.LoadHandle(packet.Params)

			if err != nil {
				em.connector.Respond(packet.ID, nil, &RPCError{
					Code:    1,
					Message: fmt.Sprint(err),
				})
				return
			}

			em.connector.Respond(packet.ID, result, nil)
		} else {
			em.connector.Respond(packet.ID, nil, nil)
		}
	case "unload":
		if em.UnloadHandle != nil {
			result, err := em.UnloadHandle(packet.Params)

			if err != nil {
				em.connector.Respond(packet.ID, nil, &RPCError{
					Code:    1,
					Message: fmt.Sprint(err),
				})
				return
			}

			em.connector.Respond(packet.ID, result, nil)
			return
		}

		em.connector.Respond(packet.ID, nil, nil)
		em.Shutdown(&Event{})

		go em.Stop()
	default:
		if em.MethodHandle != nil {
			result, err := em.MethodHandle(packet.Method, packet.Params)

			if err != nil {
				em.connector.Respond(packet.ID, nil, &RPCError{
					Code:    1,
					Message: fmt.Sprint(err),
				})
				return
			}

			em.connector.Respond(packet.ID, result, nil)
		} else {
			em.connector.Respond(packet.ID, nil, &RPCError{
				Code:    404,
				Message: "method handler is not attached",
			})
		}
	}
}

func (em *ExternalModule) Stop() {
	em.connector.Stop()
}

func (em *ExternalModule) Shutdown(event *Event) {
	if em.ShutdownHandle != nil {
		err := em.ShutdownHandle()

		if err != nil {
			fmt.Fprint(os.Stderr, err)
		}
	}
}

func (em *ExternalModule) Send(method string, params json.RawMessage) (json.RawMessage, error) {
	resChan, err := em.connector.Send(method, params)

	if err != nil {
		return nil, err
	}

	response, ok := <-resChan

	if !ok {
		return nil, fmt.Errorf("RPC connector closed before response")
	}

	if response.Error != nil {
		return nil, fmt.Errorf("Response error: %s", response.Error.Message)
	} else {
		return response.Result, nil
	}
}
