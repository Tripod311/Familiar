package main

import (
	"encoding/json"
	"fmt"
	"math"
)

type Schema struct {
	Raw json.RawMessage

	Create json.RawMessage
	Update json.RawMessage

	parsed map[string]any
}

func NewSchema(raw json.RawMessage) (*Schema, error) {
	var parsed map[string]any

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}

	if parsed["type"] != "object" {
		return nil, fmt.Errorf("root schema must be an object")
	}

	create, err := buildCreateSchema(parsed)
	if err != nil {
		return nil, err
	}

	update, err := buildUpdateSchema(parsed)
	if err != nil {
		return nil, err
	}

	return &Schema{
		Raw:    raw,
		Create: create,
		Update: update,
		parsed: parsed,
	}, nil
}

func (schema *Schema) Validate(data json.RawMessage) error {
	var value any

	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	return validateValue(value, schema.parsed, "")
}

func (schema *Schema) ValidatePatch(data json.RawMessage) error {
	var value any

	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	patchSchema := cloneMap(schema.parsed)

	// Patch does not require fields that are required
	// for the complete document.
	delete(patchSchema, "required")

	return validateValue(value, patchSchema, "")
}

func buildCreateSchema(schema map[string]any) (json.RawMessage, error) {
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to build create schema: %w", err)
	}

	return data, nil
}

func buildUpdateSchema(schema map[string]any) (json.RawMessage, error) {
	updateSchema := cloneMap(schema)

	delete(updateSchema, "required")

	data, err := json.Marshal(updateSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to build update schema: %w", err)
	}

	return data, nil
}

func validateValue(value any, schema map[string]any, path string) error {
	expectedType, _ := schema["type"].(string)

	switch expectedType {
	case "object":
		return validateObject(value, schema, path)

	case "array":
		return validateArray(value, schema, path)

	case "string":
		if _, ok := value.(string); !ok {
			return validationError(path, "expected string")
		}

	case "number":
		if _, ok := value.(float64); !ok {
			return validationError(path, "expected number")
		}

	case "integer":
		number, ok := value.(float64)
		if !ok || math.Trunc(number) != number {
			return validationError(path, "expected integer")
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			return validationError(path, "expected boolean")
		}

	default:
		return validationError(
			path,
			fmt.Sprintf("unsupported schema type: %s", expectedType),
		)
	}

	return nil
}

func validateObject(value any, schema map[string]any, path string) error {
	object, ok := value.(map[string]any)
	if !ok {
		return validationError(path, "expected object")
	}

	properties := map[string]any{}

	if rawProperties, exists := schema["properties"]; exists {
		var ok bool

		properties, ok = rawProperties.(map[string]any)
		if !ok {
			return validationError(path, "invalid properties schema")
		}
	}

	if required, exists := schema["required"]; exists {
		items, ok := required.([]any)
		if !ok {
			return validationError(path, "invalid required schema")
		}

		for _, item := range items {
			name, ok := item.(string)
			if !ok {
				return validationError(path, "invalid required field")
			}

			if _, exists := object[name]; !exists {
				return validationError(
					joinPath(path, name),
					"required field is missing",
				)
			}
		}
	}

	for key, value := range object {
		property, exists := properties[key]

		if !exists {
			return validationError(
				joinPath(path, key),
				"unknown field",
			)
		}

		propertySchema, ok := property.(map[string]any)
		if !ok {
			return validationError(
				joinPath(path, key),
				"invalid property schema",
			)
		}

		if err := validateValue(
			value,
			propertySchema,
			joinPath(path, key),
		); err != nil {
			return err
		}
	}

	return nil
}

func validateArray(value any, schema map[string]any, path string) error {
	array, ok := value.([]any)
	if !ok {
		return validationError(path, "expected array")
	}

	rawItems, exists := schema["items"]
	if !exists {
		return nil
	}

	itemSchema, ok := rawItems.(map[string]any)
	if !ok {
		return validationError(path, "invalid items schema")
	}

	for index, item := range array {
		itemPath := fmt.Sprintf("%s[%d]", path, index)

		if err := validateValue(item, itemSchema, itemPath); err != nil {
			return err
		}
	}

	return nil
}

func validationError(path string, message string) error {
	if path == "" {
		return fmt.Errorf("%s", message)
	}

	return fmt.Errorf("%s: %s", path, message)
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}

	return parent + "." + child
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))

	for key, value := range source {
		result[key] = value
	}

	return result
}
