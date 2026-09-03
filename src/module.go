package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	sdk "tripod311/familiar-sdk"
)

type Module struct {
	sdk.Emitter

	Name        string   `json:"name"`
	Description string   `json:"description"`
	Exec        string   `json:"exec"`
	Args        []string `json:"args"`

	connector *sdk.RPCConnector
	instance  *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	logger    *log.Logger

	started bool
}

func NewModule(path string) (*Module, error) {
	manifestPath := filepath.Join(path, "manifest.json")

	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf(
			"module %s error: manifest not found: %w",
			path,
			err,
		)
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf(
			"module %s error while reading manifest: %w",
			path,
			err,
		)
	}

	var module Module

	if err := json.Unmarshal(byteValue, &module); err != nil {
		return nil, fmt.Errorf(
			"module %s error: corrupted manifest: %w",
			path,
			err,
		)
	}

	module.Exec, err = filepath.Abs(filepath.Join(path, module.Exec))
	if err != nil {
		return nil, fmt.Errorf(
			"module %s error while resolving executable: %w",
			path,
			err,
		)
	}

	module.logger = log.New(
		os.Stderr,
		fmt.Sprintf("[module:%s] ", module.Name),
		log.Ldate|log.Ltime|log.Lmicroseconds,
	)

	module.Listeners = make(map[string]map[uint64]func(*sdk.Event))

	return &module, nil
}

func (module *Module) Start() error {
	module.instance = exec.Command(module.Exec, module.Args...)
	module.instance.Dir = filepath.Dir(module.Exec)
	module.instance.Env = os.Environ()

	stdin, err := module.instance.StdinPipe()
	if err != nil {
		return fmt.Errorf("create stdin pipe: %w", err)
	}

	stdout, err := module.instance.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}

	stderr, err := module.instance.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe: %w", err)
	}

	module.stdin = stdin
	module.stdout = stdout
	module.stderr = stderr

	module.connector = sdk.NewConnector(module.Name, module.stdout, module.stdin)
	module.connector.On("packetReceived", module.processPacket)

	if err := module.instance.Start(); err != nil {
		return fmt.Errorf("start module %s: %w", module.Name, err)
	}

	go module.stderrLoop()

	module.logger.Printf(
		"started: %s",
		module.Description,
	)

	module.started = true

	return nil
}

func (module *Module) Stop() {
	if !module.started {
		return
	}

	response, err := module.Send("unload", nil)

	if err != nil {
		fmt.Printf("Module %s unload error: %s", module.Name, err)
	}

	if response.Error != nil {
		fmt.Printf("Module %s unload error: %s", module.Name, fmt.Errorf("(%d) %s", response.Error.Code, response.Error.Message))
	}

	module.connector.Stop()
	module.instance.Process.Kill()
}

func (module *Module) Send(method string, params json.RawMessage) (*sdk.RPCPacket, error) {
	resChan, err := module.connector.Send(method, params)

	if err != nil {
		return nil, err
	}

	response := <-resChan

	return &response, nil
}

func (module *Module) stderrLoop() {
	scanner := bufio.NewScanner(module.stderr)

	scanner.Buffer(make([]byte, 4096), 1024*1024)

	for scanner.Scan() {
		module.logger.Print(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		module.logger.Printf("stderr read error: %s", err)
	}
}

func (module *Module) processPacket(event *sdk.Event) {
	module.Emit(*event)
}

func (module *Module) moduleDisconnected(event *sdk.Event) {
	message := fmt.Sprintf("Module %s stopped: %s", module.Name, event.Data)

	module.Emit(sdk.Event{
		Command: "closed",
		Data:    message,
	})
}
