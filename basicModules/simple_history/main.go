package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	sdk "tripod311/familiar-sdk"
)

const dumpPath = "dump.json"

type HistoryConfig struct {
	Store bool `json:"store"`
	Size  int  `json:"size"`
}

var config HistoryConfig
var history []*sdk.Message
var module *sdk.ExternalModule

func main() {
	history = make([]*sdk.Message, 0)

	module = sdk.NewExternalModule()

	module.LoadHandle = Setup
	module.UnloadHandle = Shutdown
	module.MethodHandle = ProcessPacket

	module.Start()
}

func Shutdown(params json.RawMessage) (json.RawMessage, error) {
	if config.Store && config.Size > 0 {
		Dump()
	}

	return nil, nil
}

func Setup(params json.RawMessage) (json.RawMessage, error) {
	err := json.Unmarshal(params, &config)
	if err != nil {
		return nil, fmt.Errorf("History setup error: %s", err)
	}

	if config.Store {
		Load()
	}

	return nil, nil
}

func trimHistory() {
	if config.Size <= 0 {
		clear(history)
		history = nil
		return
	}

	if len(history) <= config.Size {
		return
	}

	excess := len(history) - config.Size

	copy(history, history[excess:])

	clear(history[config.Size:])

	history = history[:config.Size]
}

func Load() {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(
				os.Stderr,
				"History dump not found; starting with empty history",
			)
			return
		}

		fmt.Fprintf(
			os.Stderr,
			"History load error: %s\n",
			err,
		)
		return
	}

	var loaded []*sdk.Message

	if err := json.Unmarshal(data, &loaded); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"History dump decode error: %s\n",
			err,
		)
		return
	}

	history = loaded
	trimHistory()

	fmt.Fprintf(
		os.Stderr,
		"History loaded: %d messages\n",
		len(history),
	)
}

func Dump() {
	trimHistory()

	data, err := json.MarshalIndent(history, "", "\t")
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"History dump encode error: %s\n",
			err,
		)
		return
	}

	if err := os.WriteFile(dumpPath, data, 0644); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"History dump write error: %s\n",
			err,
		)
		return
	}

	fmt.Fprintf(
		os.Stderr,
		"History saved: %d messages\n",
		len(history),
	)
}

func ProcessPacket(method string, params json.RawMessage) (json.RawMessage, error) {
	switch method {
	case "get":
		bytes, err := json.Marshal(history)
		if err != nil {
			return nil, fmt.Errorf("History get error: %s", err)
		}

		return bytes, nil
	case "append":
		var msg sdk.Message

		if err := json.Unmarshal(params, &msg); err != nil {
			return nil, fmt.Errorf(
				"history append error: %w",
				err,
			)
		}

		history = append(history, &msg)
		trimHistory()

		return nil, nil
	default:
		return nil, fmt.Errorf("Unknown method: %s", method)
	}
}
