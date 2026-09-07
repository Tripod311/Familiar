package main

import (
	"encoding/json"
	"fmt"
	"os"

	sdk "tripod311/familiar-sdk"
)

const (
	docsPath   = "docs"
	schemaPath = "schema.json"
)

var module *sdk.ExternalModule
var store *Store

func main() {
	module = sdk.NewExternalModule()

	module.LoadHandle = Setup
	module.MethodHandle = ProcessPacket
	module.GatherFunctions = GatherFunctions
	module.CallFunction = CallFunction

	module.Start()
}

func Setup(params json.RawMessage) (json.RawMessage, error) {
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, nil
	}

	if !json.Valid(schemaBytes) {
		return nil, fmt.Errorf("Corrupted schema")
	}

	store, err = NewStore(docsPath, schemaBytes)
	if err != nil {
		return nil, fmt.Errorf("Error on store creation: %s", err)
	}

	return nil, nil
}

func ProcessPacket(method string, params json.RawMessage) (json.RawMessage, error) {
	switch method {
	default:
		return nil, fmt.Errorf("Unknown method: %s", method)
	}
}

func GatherFunctions() ([]sdk.ToolDescription, error) {
	createParameters, err := buildDocumentToolSchema(
		"document",
		store.Schema.Create,
	)
	if err != nil {
		return nil, err
	}

	updateParameters, err := buildDocumentToolSchema(
		"patch",
		store.Schema.Update,
	)
	if err != nil {
		return nil, err
	}

	return []sdk.ToolDescription{
		{
			Name:        "records_create",
			Description: "Creates a new structured document. The document must follow the configured record schema. The name uniquely identifies the document and must follow the naming rules specified for this records collection.",
			Parameters:  createParameters,
		},
		{
			Name:        "records_update",
			Description: "Updates an existing structured document. Only include fields that should be changed in the patch. Fields not present in the patch remain unchanged.",
			Parameters:  updateParameters,
		},
		{
			Name:        "records_delete",
			Description: "Deletes an existing document by its name. Use only when the document should be permanently removed.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Name of the document to delete."
					}
				},
				"required": ["name"]
			}`),
		},
		{
			Name:        "records_get",
			Description: "Returns a structured document by its exact name.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Exact name of the document to retrieve."
					}
				},
				"required": ["name"]
			}`),
		},
		{
			Name:        "records_list",
			Description: "Returns the names of all documents currently stored in this records collection.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "records_search",
			Description: "Searches structured records using optional keyword matching and field filters. Filters are combined with AND. String fields support eq, contains, and prefix. Numeric fields support eq, gt, gte, lt, and lte. Boolean fields support eq. Results are ranked by keyword relevance and limited to the requested amount.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Optional free-text search query. Matching is performed against string values in documents and used to rank results by relevance."
					},
					"filters": {
						"type": "object",
						"description": "Optional field filters. Each key must be a field defined in the record schema. All filters are combined with AND.",
						"additionalProperties": {
							"type": "object",
							"properties": {
								"op": {
									"type": "string",
									"description": "Comparison operator. Use eq, contains, or prefix for strings; eq, gt, gte, lt, or lte for numbers; eq for booleans.",
									"enum": [
										"eq",
										"contains",
										"prefix",
										"gt",
										"gte",
										"lt",
										"lte"
									]
								},
								"value": {
									"description": "Value to compare the record field against."
								}
							},
							"required": [
								"op",
								"value"
							]
						}
					}
				}
			}`),
		},
	}, nil
}

func CallFunction(name string, arguments string) (json.RawMessage, error) {
	switch name {
	case "records_create":
		var args struct {
			Name     string          `json:"name"`
			Document json.RawMessage `json:"document"`
		}

		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return nil, fmt.Errorf("Invalid arguments: %s", err)
		}

		if err := store.Create(args.Name, args.Document); err != nil {
			return nil, err
		}

		return nil, nil

	case "records_update":
		var args struct {
			Name  string          `json:"name"`
			Patch json.RawMessage `json:"patch"`
		}

		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return nil, fmt.Errorf("Invalid arguments: %s", err)
		}

		if err := store.Update(args.Name, args.Patch); err != nil {
			return nil, err
		}

		return nil, nil

	case "records_delete":
		var args struct {
			Name string `json:"name"`
		}

		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return nil, fmt.Errorf("Invalid arguments: %s", err)
		}

		if err := store.Delete(args.Name); err != nil {
			return nil, err
		}

		return nil, nil

	case "records_get":
		var args struct {
			Name string `json:"name"`
		}

		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return nil, fmt.Errorf("Invalid arguments: %s", err)
		}

		return store.Get(args.Name)

	case "records_list":
		result, err := store.List()
		if err != nil {
			return nil, err
		}

		bytes, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("Serialization error: %s", err)
		}

		return bytes, nil

	case "records_search":
		var args SearchArguments

		err := json.Unmarshal([]byte(arguments), &args)
		if err != nil {
			return nil, err
		}

		result, err := store.Search(args)
		if err != nil {
			return nil, err
		}

		bytes, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("Serialization error: %s", err)
		}

		return bytes, nil

	default:
		return nil, fmt.Errorf("Unknown function: %s", name)
	}
}

func buildDocumentToolSchema(
	fieldName string,
	documentSchema json.RawMessage,
) (json.RawMessage, error) {
	var document any

	if err := json.Unmarshal(documentSchema, &document); err != nil {
		return nil, fmt.Errorf(
			"Failed to build tool schema: %s",
			err,
		)
	}

	parameters := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Unique document name.",
			},
			fieldName: document,
		},
		"required": []string{
			"name",
			fieldName,
		},
	}

	bytes, err := json.Marshal(parameters)
	if err != nil {
		return nil, fmt.Errorf(
			"Failed to serialize tool schema: %s",
			err,
		)
	}

	return bytes, nil
}
