package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type ServerStatus int

const (
	DOWN = iota
	STARTING
	READY
)

type Server struct {
	Emitter
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Exec              string   `json:"exec"`
	Args              []string `json:"args"`
	Port              int
	Model             string
	Status            ServerStatus
	instance          exec.Cmd
	exitError         error
	pollExitTimeout   time.Duration
	pollContext       context.Context
	pollContextCancel context.CancelFunc
}

func NewServer() *Server {
	result := Server{}
	result.Listeners = make(map[string]map[uint64]func(*Event))
	return &result
}

func (server *Server) Start(timeout time.Duration, verbose bool) {
	if server.Status != DOWN {
		return
	}

	server.pollExitTimeout = timeout

	args := make([]string, len(server.Args))
	for i, a := range server.Args {
		switch a {
		case "%MODEL%":
			args[i] = server.Model
		case "%PORT%":
			args[i] = fmt.Sprint(server.Port)
		default:
			args[i] = a
		}
	}

	server.instance = *exec.Command(server.Exec, args...)

	if verbose {
		server.instance.Stdout = os.Stdout
		server.instance.Stderr = os.Stderr
	}

	server.instance.Dir = filepath.Dir(server.Exec)
	server.instance.Env = os.Environ()

	err := server.instance.Start()
	if err != nil {
		fmt.Printf(
			"Error on starting %s inference engine: %s\n",
			server.Name,
			err,
		)
		return
	}

	server.Status = STARTING

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	server.pollContext = ctx
	server.pollContextCancel = cancel

	go server.pollHealth(ctx, cancel)

	go func() {
		server.instance.Wait()
		server.handleExit()
	}()
}

func (server *Server) Stop() {
	if server.instance.Process == nil {
		return
	}

	server.exitError = nil
	server.Status = DOWN
	ev := Event{
		Command: "Stopped",
		Data:    nil,
	}
	server.Emit(ev)
	server.instance.Process.Kill()
}

func (server *Server) pollHealth(ctx context.Context, cancel context.CancelFunc) {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", server.Port)

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	timer := time.NewTimer(server.pollExitTimeout)

	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	for {
		select {
		case <-timer.C:
			server.exitError = fmt.Errorf("Inference engine timeout")
			server.instance.Process.Kill()
			return
		case <-ticker.C:
			resp, err := client.Get(url)

			if err != nil {
				continue
			}

			resp.Body.Close()

			if server.Status == STARTING {
				if resp.StatusCode == http.StatusOK {
					timer.Stop()
					server.started()
				}
			} else {
				if resp.StatusCode != http.StatusOK {
					server.stopped()
				}
			}
		}
	}
}

func (server *Server) started() {
	server.Status = READY
	ev := Event{
		Command: "Started",
		Data:    nil,
	}
	server.Emit(ev)
}

func (server *Server) stopped() {
	server.exitError = fmt.Errorf("Inference engine health poll timeout")
	ev := Event{
		Command: "Stopped",
		Data:    nil,
	}
	server.Emit(ev)
	server.instance.Process.Kill()
}

func (server *Server) handleExit() {
	server.pollContextCancel()

	if server.exitError != nil {
		fmt.Printf("Inference engine stopped with error: %s", server.exitError)
	}

	server.Status = DOWN
}

func (server *Server) Request(req *ServerRequest) (string, error) {
	for iteration := 0; iteration < 16; iteration++ {
		res, err := server.complete(req)

		if err != nil {
			return "", fmt.Errorf("Error: %s", err)
		}

		if len(res.Choices) == 0 {
			return "", fmt.Errorf("Error: empty response")
		}

		assistant := res.Choices[0].Message
		req.Messages = append(req.Messages, assistant)

		// process response
		if len(assistant.ToolCalls) > 0 {
			// process tool calls
			for _, call := range assistant.ToolCalls {
				server.Emit(Event{
					Command: "ToolCall",
					Data: ToolCallEventData{
						Request: req,
						Call:    &call,
					},
				})
			}
		} else {
			return assistant.Content, nil
		}
	}

	return "", fmt.Errorf("Tool loop exceeded iteration limit")
}

func (server *Server) complete(req *ServerRequest) (*ServerResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", server.Port),
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf(
			"llama-server returned %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	var result ServerResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
