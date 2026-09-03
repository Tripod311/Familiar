package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

type RPCPacket struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type RPCConnector struct {
	Emitter

	rpcPrefix    string
	nextPacketID uint64
	pending      map[string]chan RPCPacket
	writeMu      sync.Mutex
	stateMu      sync.Mutex
	wg           sync.WaitGroup
	stopOnce     sync.Once
	reader       io.ReadCloser
	writer       io.Writer
	stopped      bool

	encoder *json.Encoder
	decoder *json.Decoder
}

func NewConnector(prefix string, reader io.ReadCloser, writer io.Writer) *RPCConnector {
	result := RPCConnector{
		rpcPrefix: prefix,
		reader:    reader,
		writer:    writer,
		pending:   make(map[string]chan RPCPacket),
		encoder:   json.NewEncoder(writer),
		decoder:   json.NewDecoder(reader),
	}
	result.Listeners = make(map[string]map[uint64]func(*Event))

	return &result
}

func (connector *RPCConnector) Start() {
	connector.wg.Add(1)

	go func() {
		defer connector.wg.Done()

		connector.readLoop()
	}()
}

func (connector *RPCConnector) Wait() {
	connector.wg.Wait()
}

func (connector *RPCConnector) Stop() {
	connector.shutdown()
	connector.wg.Wait()
}

func (connector *RPCConnector) Send(method string, params json.RawMessage) (chan RPCPacket, error) {
	connector.stateMu.Lock()

	if connector.stopped {
		connector.stateMu.Unlock()
		return nil, errors.New("RPC connector is stopped")
	}

	result := make(chan RPCPacket, 1)
	intId := connector.nextPacketID
	connector.nextPacketID++
	id := fmt.Sprintf("%s_%d", connector.rpcPrefix, intId)

	connector.pending[id] = result
	connector.stateMu.Unlock()

	connector.writeMu.Lock()
	err := connector.encoder.Encode(RPCPacket{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	})
	connector.writeMu.Unlock()
	if err != nil {
		connector.stateMu.Lock()

		if connector.pending[id] == result {
			delete(connector.pending, id)
			close(result)
		}

		connector.stateMu.Unlock()

		return nil, fmt.Errorf("RPC send error: %s", err)
	}

	return result, nil
}

func (connector *RPCConnector) Respond(
	id string,
	result any,
	rpcError *RPCError,
) error {
	connector.writeMu.Lock()
	defer connector.writeMu.Unlock()

	return connector.encoder.Encode(RPCPacket{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   rpcError,
	})
}

func (connector *RPCConnector) readLoop() {
	for {
		var packet RPCPacket

		if err := connector.decoder.Decode(&packet); err != nil {
			connector.stateMu.Lock()
			stopped := connector.stopped
			connector.stateMu.Unlock()

			if stopped {
				return
			}

			if errors.Is(err, io.EOF) {
				connector.Emit(Event{
					Command: "closed",
					Data:    "eof",
				})
				connector.shutdown()
				return
			}

			continue
		}

		connector.stateMu.Lock()

		pending, exists := connector.pending[packet.ID]
		if exists {
			delete(connector.pending, packet.ID)
		}

		connector.stateMu.Unlock()

		if exists {
			pending <- packet
			close(pending)
		} else {
			connector.Emit(Event{
				Command: "packetReceived",
				Data:    packet,
			})
		}
	}
}

func (connector *RPCConnector) shutdown() {
	connector.stopOnce.Do(func() {
		connector.stateMu.Lock()
		connector.stopped = true

		pending := connector.pending
		connector.pending = make(map[string]chan RPCPacket)

		connector.stateMu.Unlock()

		for _, responseChan := range pending {
			close(responseChan)
		}

		_ = connector.reader.Close()
	})
}
