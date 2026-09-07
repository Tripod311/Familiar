package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const LIMIT = 20

type SearchArguments struct {
	Query   string                  `json:"query"`
	Filters map[string]SearchFilter `json:"filters"`
}

type SearchFilter struct {
	Op    string `json:"op"`
	Value any    `json:"value"`
}

type SearchResult struct {
	Name     string          `json:"name"`
	Document json.RawMessage `json:"document"`
	Score    float64         `json:"score"`
}

func (store *Store) Search(args SearchArguments) ([]SearchResult, error) {
	if err := store.ensureDir(); err != nil {
		return nil, err
	}

	if err := store.validateSearchFilters(args.Filters); err != nil {
		return nil, err
	}

	keywords := splitKeywords(args.Query)

	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read records directory: %w",
			err,
		)
	}

	results := make([]SearchResult, 0)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(store.Dir, entry.Name())

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to read document %s: %w",
				entry.Name(),
				err,
			)
		}

		var document map[string]any

		if err := json.Unmarshal(data, &document); err != nil {
			return nil, fmt.Errorf(
				"failed to decode document %s: %w",
				entry.Name(),
				err,
			)
		}

		matches, err := matchFilters(
			document,
			args.Filters,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to filter document %s: %w",
				entry.Name(),
				err,
			)
		}

		if !matches {
			continue
		}

		score := calculateScore(
			document,
			keywords,
		)

		// If query was specified, completely unrelated documents
		// should not be returned.
		if len(keywords) > 0 && score == 0 {
			continue
		}

		name := strings.TrimSuffix(
			entry.Name(),
			filepath.Ext(entry.Name()),
		)

		results = append(results, SearchResult{
			Name:     name,
			Document: json.RawMessage(data),
			Score:    score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Name < results[j].Name
		}

		return results[i].Score > results[j].Score
	})

	if len(results) > LIMIT {
		results = results[:LIMIT]
	}

	return results, nil
}

func (store *Store) validateSearchFilters(
	filters map[string]SearchFilter,
) error {
	if len(filters) == 0 {
		return nil
	}

	propertiesRaw, exists := store.Schema.parsed["properties"]
	if !exists {
		return fmt.Errorf("schema does not define properties")
	}

	properties, ok := propertiesRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid schema properties")
	}

	for field, filter := range filters {
		propertyRaw, exists := properties[field]
		if !exists {
			return fmt.Errorf(
				"unknown filter field: %s",
				field,
			)
		}

		property, ok := propertyRaw.(map[string]any)
		if !ok {
			return fmt.Errorf(
				"invalid schema for field: %s",
				field,
			)
		}

		fieldType, _ := property["type"].(string)

		if err := validateFilter(
			field,
			fieldType,
			filter,
		); err != nil {
			return err
		}
	}

	return nil
}

func validateFilter(
	field string,
	fieldType string,
	filter SearchFilter,
) error {
	switch fieldType {
	case "string":
		switch filter.Op {
		case "eq", "contains", "prefix":
		default:
			return fmt.Errorf(
				"unsupported operator %s for string field %s",
				filter.Op,
				field,
			)
		}

		if _, ok := filter.Value.(string); !ok {
			return fmt.Errorf(
				"filter value for field %s must be string",
				field,
			)
		}

	case "number", "integer":
		switch filter.Op {
		case "eq", "gt", "gte", "lt", "lte":
		default:
			return fmt.Errorf(
				"unsupported operator %s for numeric field %s",
				filter.Op,
				field,
			)
		}

		if _, ok := filter.Value.(float64); !ok {
			return fmt.Errorf(
				"filter value for field %s must be number",
				field,
			)
		}

	case "boolean":
		if filter.Op != "eq" {
			return fmt.Errorf(
				"unsupported operator %s for boolean field %s",
				filter.Op,
				field,
			)
		}

		if _, ok := filter.Value.(bool); !ok {
			return fmt.Errorf(
				"filter value for field %s must be boolean",
				field,
			)
		}

	default:
		return fmt.Errorf(
			"field %s of type %s cannot be filtered",
			field,
			fieldType,
		)
	}

	return nil
}

func matchFilters(
	document map[string]any,
	filters map[string]SearchFilter,
) (bool, error) {
	for field, filter := range filters {
		value, exists := document[field]
		if !exists {
			return false, nil
		}

		match, err := matchFilter(value, filter)
		if err != nil {
			return false, err
		}

		if !match {
			return false, nil
		}
	}

	return true, nil
}

func matchFilter(
	value any,
	filter SearchFilter,
) (bool, error) {
	switch expected := filter.Value.(type) {
	case string:
		actual, ok := value.(string)
		if !ok {
			return false, nil
		}

		actualLower := strings.ToLower(actual)
		expectedLower := strings.ToLower(expected)

		switch filter.Op {
		case "eq":
			return actualLower == expectedLower, nil

		case "contains":
			return strings.Contains(
				actualLower,
				expectedLower,
			), nil

		case "prefix":
			return strings.HasPrefix(
				actualLower,
				expectedLower,
			), nil
		}

	case float64:
		actual, ok := value.(float64)
		if !ok {
			return false, nil
		}

		switch filter.Op {
		case "eq":
			return actual == expected, nil

		case "gt":
			return actual > expected, nil

		case "gte":
			return actual >= expected, nil

		case "lt":
			return actual < expected, nil

		case "lte":
			return actual <= expected, nil
		}

	case bool:
		actual, ok := value.(bool)
		if !ok {
			return false, nil
		}

		if filter.Op == "eq" {
			return actual == expected, nil
		}
	}

	return false, fmt.Errorf(
		"unsupported filter operation: %s",
		filter.Op,
	)
}

func splitKeywords(query string) []string {
	fields := strings.Fields(
		strings.ToLower(query),
	)

	result := make([]string, 0, len(fields))

	for _, field := range fields {
		field = strings.TrimSpace(field)

		if field != "" {
			result = append(result, field)
		}
	}

	return result
}

func calculateScore(
	document map[string]any,
	keywords []string,
) float64 {
	if len(keywords) == 0 {
		return 0
	}

	values := make([]string, 0)

	collectStrings(document, &values)

	var score float64

	for _, value := range values {
		lower := strings.ToLower(value)

		for _, keyword := range keywords {
			count := strings.Count(
				lower,
				keyword,
			)

			score += float64(count)
		}
	}

	return score
}

func collectStrings(
	value any,
	result *[]string,
) {
	switch typed := value.(type) {
	case string:
		*result = append(*result, typed)

	case []any:
		for _, item := range typed {
			collectStrings(item, result)
		}

	case map[string]any:
		for _, item := range typed {
			collectStrings(item, result)
		}
	}
}
