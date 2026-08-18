package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type Module struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Exec        string   `json:"exec"`
	Args        []string `json:"args"`

	instance exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	encoder  *json.Encoder
	decoder  *json.Decoder
	reqId    uint64

	RPCChan chan RPCPacket
}

func NewModule(path string) (*Module, error) {
	file, err := os.Open(fmt.Sprintf("%s/manifest.json", path))
	if err != nil {
		return nil, fmt.Errorf("Module %s error, manifest not found", path)
	}

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("Module %s error on reading manifest", path)
	}

	var module Module

	err = json.Unmarshal(byteValue, &module)
	if err != nil {
		return nil, fmt.Errorf("Module %s error, corrupted manifest", path)
	}

	module.Exec, _ = filepath.Abs(fmt.Sprintf("%s/%s", path, module.Exec))
	module.RPCChan = make(chan RPCPacket, 1)

	return &module, nil
}

func (module *Module) Start() error {
	module.instance = *exec.Command(module.Exec, module.Args...)
	module.instance.Dir = filepath.Dir(module.Exec)
	module.instance.Env = os.Environ()

	stdin, err := module.instance.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := module.instance.StdoutPipe()
	if err != nil {
		return err
	}

	module.instance.Stderr = os.Stderr
	module.stdin = stdin
	module.stdout = stdout
	module.encoder = json.NewEncoder(module.stdin)
	module.decoder = json.NewDecoder(module.stdout)

	err = module.instance.Start()
	if err != nil {
		return err
	}

	go module.readLoop()

	return nil
}

func (module *Module) Stop() {
	module.Send("unload", json.RawMessage{})

	unloadResult := <-module.RPCChan

	if unloadResult.Error != nil {
		fmt.Printf("Module %s unload error: %s", module.Name, unloadResult.Error.Message)
	}

	module.instance.Process.Kill()
}

func (module *Module) Send(method string, params json.RawMessage) {
	module.encoder.Encode(RPCPacket{
		JSONRPC: "2.0",
		ID:      module.reqId,
		Method:  method,
		Params:  params,
	})
	module.reqId++
}

func (module *Module) readLoop() {
	for {
		var res RPCPacket

		if err := module.decoder.Decode(&res); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			fmt.Fprintln(os.Stderr, "decode error:", err)
			continue
		}

		module.RPCChan <- res
	}
}
