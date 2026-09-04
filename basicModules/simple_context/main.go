package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	sdk "tripod311/familiar-sdk"
)

const dumpPath = "dump.json"
const promptPath = "prompt.md"
const memorySchemaPath = "memory.json"

type MemoryDef struct {
	Schema       json.RawMessage `json:"schema"`
	InitialState json.RawMessage `json:"initialState"`
}

type ContextConfig struct {
	Store     bool `json:"store"`
	UseMemory bool `json:"useMemory"`
}

var config ContextConfig
var memorySchema json.RawMessage
var memoryState json.RawMessage
var systemPrompt string
var module *sdk.ExternalModule

func main() {
	module = sdk.NewExternalModule()

	module.LoadHandle = Setup
	module.UnloadHandle = Shutdown
	module.MethodHandle = ProcessPacket
	module.GatherFunctions = GatherFunctions
	module.CallFunction = CallFunction

	module.Start()
}

func Shutdown(params json.RawMessage) (json.RawMessage, error) {
	if config.Store && config.UseMemory {
		Dump()
	}

	return nil, nil
}

func Setup(params json.RawMessage) (json.RawMessage, error) {
	err := json.Unmarshal(params, &config)
	if err != nil {
		return nil, fmt.Errorf("Context manager setup error: %s", err)
	}

	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, fmt.Errorf("Context manager prompt reading error: %s", err)
	}
	systemPrompt = string(promptData)

	if !config.UseMemory {
		return nil, nil
	}

	memoryData, err := os.ReadFile(memorySchemaPath)
	if err != nil {
		return nil, fmt.Errorf("Context manager memory schema reading error: %s", err)
	}

	var def MemoryDef
	err = json.Unmarshal(memoryData, &def)
	if err != nil {
		return nil, fmt.Errorf("Context manager error, corrupted memory schema: %s", err)
	}

	memorySchema = def.Schema
	memoryState = def.InitialState

	if config.Store {
		dumpedStateData, err := os.ReadFile(dumpPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintln(
					os.Stderr,
					"Context memory dump not found; starting with initial state",
				)
				return nil, nil
			}

			return nil, fmt.Errorf("Context memory dump reading error: %s", err)
		}

		if !json.Valid(dumpedStateData) {
			return nil, fmt.Errorf("Context memory dump corrupted")
		}

		memoryState = dumpedStateData

		fmt.Fprintln(
			os.Stderr,
			"Context memory dump loaded",
		)
	}

	return nil, nil
}

func Dump() {
	data, err := json.MarshalIndent(memoryState, "", "\t")
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Context memory dump encode error: %s\n",
			err,
		)
		return
	}

	if err := os.WriteFile(dumpPath, data, 0644); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Context memory dump write error: %s\n",
			err,
		)
		return
	}

	fmt.Fprint(
		os.Stderr,
		"Context memory saved",
	)
}

func ProcessPacket(method string, params json.RawMessage) (json.RawMessage, error) {
	switch method {
	case "get_context":
		systemContent := systemPrompt

		if config.UseMemory {
			systemContent += fmt.Sprintf(
				"\n\nCurrent memory state:\n<data role=\"memory\">\n%s\n</data>",
				memoryState,
			)
		}

		result := []sdk.Message{
			{
				Role:    sdk.RoleSystem,
				Content: systemContent,
			},
		}

		bytes, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf(
				"context serialization error: %w",
				err,
			)
		}

		return bytes, nil

	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

func GatherFunctions() ([]sdk.ToolDescription, error) {
	if config.UseMemory {
		return []sdk.ToolDescription{
			{
				Name:        "update_memory",
				Description: "Updates memory state",
				Strict:      true,
				Parameters:  memorySchema,
			},
		}, nil
	} else {
		return make([]sdk.ToolDescription, 0), nil
	}
}

func CallFunction(name string, arguments string) (json.RawMessage, error) {
	switch name {
	case "update_memory":
		if !config.UseMemory {
			return nil, fmt.Errorf("useMemory set to false")
		}

		fmt.Fprintf(os.Stderr, "MEMORY UPDATE: %s", arguments)
		memoryState = []byte(arguments)

		return nil, nil
	default:
		return nil, fmt.Errorf("Unknown function: %s", name)
	}
}
