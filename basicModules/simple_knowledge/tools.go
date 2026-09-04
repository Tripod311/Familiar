package main

import (
	"encoding/json"
	sdk "tripod311/familiar-sdk"
)

var SearchTool = sdk.ToolDescription{
	Name:        "knowledge_search",
	Description: "Search the knowledge base for information matching the provided keywords.",
	Strict:      true,
	Parameters: json.RawMessage(`{
		"type": "object",
		"properties": {
			"keywords": {
				"type": "array",
				"description": "Keywords used to search the knowledge base.",
				"items": {
					"type": "string"
				}
			}
		},
		"required": ["keywords"],
		"additionalProperties": false
	}`),
}

var AppendTool = sdk.ToolDescription{
	Name:        "knowledge_append",
	Description: "Append new information to the persistent knowledge base.",
	Strict:      true,
	Parameters: json.RawMessage(`{
		"type": "object",
		"properties": {
			"tags": {
				"type": "array",
				"description": "Tags describing the stored information.",
				"items": {
					"type": "string"
				}
			},
			"content": {
				"type": "string",
				"description": "Information to store in the knowledge base."
			}
		},
		"required": ["tags", "content"],
		"additionalProperties": false
	}`),
}
