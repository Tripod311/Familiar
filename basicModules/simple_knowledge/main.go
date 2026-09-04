package main

import (
	"encoding/json"
	"fmt"
	sdk "tripod311/familiar-sdk"
)

type KnowledgeConfig struct {
	AllowAppend     bool   `json:"allowAppend"`
	MaxAppendixSize uint64 `json:"maxAppendixSize"`
	SearchLimit     int    `json:"searchLimit"`
}

var config KnowledgeConfig
var base Knowledge
var module *sdk.ExternalModule

func main() {
	module = sdk.NewExternalModule()

	module.LoadHandle = Setup
	module.UnloadHandle = Shutdown
	module.GatherFunctions = GatherFunctions
	module.CallFunction = CallFunction

	module.Start()
}

func Shutdown(params json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

func Setup(params json.RawMessage) (json.RawMessage, error) {
	err := json.Unmarshal(params, &config)
	if err != nil {
		return nil, fmt.Errorf("Knowledge setup error: %s", err)
	}

	if !config.AllowAppend {
		base.MaxAppendixSize = 0
	} else {
		base.MaxAppendixSize = config.MaxAppendixSize
	}
	base.Load()

	return nil, nil
}

func GatherFunctions() ([]sdk.ToolDescription, error) {
	if config.AllowAppend {
		return []sdk.ToolDescription{
			SearchTool,
			AppendTool,
		}, nil
	} else {
		return []sdk.ToolDescription{
			SearchTool,
		}, nil
	}
}

func CallFunction(name string, arguments string) (json.RawMessage, error) {
	switch name {
	case "knowledge_search":
		var args SearchArguments

		err := json.Unmarshal([]byte(arguments), &args)
		if err != nil {
			return nil, fmt.Errorf("Invalid arguments: %s", err)
		}

		result := base.Search(args.Keywords, config.SearchLimit)

		resBytes, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("Serialization error: %s", err)
		}

		return resBytes, nil
	case "knowledge_append":
		var newChunk KnowledgeChunk

		err := json.Unmarshal([]byte(arguments), &newChunk)
		if err != nil {
			return nil, fmt.Errorf("Invalid chunk: %s", err)
		}

		base.Append(newChunk.Tags, newChunk.Content)

		return nil, nil
	default:
		return nil, fmt.Errorf("Unknown function: %s", name)
	}
}
