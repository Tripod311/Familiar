package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var validDocumentName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type Store struct {
	Dir    string
	Schema *Schema
}

func NewStore(dir string, schema json.RawMessage) (*Store, error) {
	schemaInstance, err := NewSchema(schema)
	if err != nil {
		return nil, err
	}

	return &Store{
		Dir:    dir,
		Schema: schemaInstance,
	}, nil
}

func (store *Store) Create(name string, document json.RawMessage) error {
	path, err := store.documentPath(name)
	if err != nil {
		return err
	}

	if err := store.ensureDir(); err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("document already exists: %s", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to check document %s: %w", name, err)
	}

	if err := store.Schema.Validate(document); err != nil {
		return fmt.Errorf(
			"document %s does not match schema: %w",
			name,
			err,
		)
	}

	var value map[string]any

	if err := json.Unmarshal(document, &value); err != nil {
		return fmt.Errorf("invalid document JSON: %w", err)
	}

	data, err := json.MarshalIndent(value, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to encode document %s: %w", name, err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to create document %s: %w", name, err)
	}

	return nil
}

func (store *Store) Update(name string, patch json.RawMessage) error {
	path, err := store.documentPath(name)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("document not found: %s", name)
		}

		return fmt.Errorf("failed to read document %s: %w", name, err)
	}

	if err := store.Schema.ValidatePatch(patch); err != nil {
		return fmt.Errorf(
			"patch for document %s does not match schema: %w",
			name,
			err,
		)
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("failed to decode document %s: %w", name, err)
	}

	var values map[string]any
	if err := json.Unmarshal(patch, &values); err != nil {
		return fmt.Errorf("invalid patch JSON: %w", err)
	}

	for key, value := range values {
		document[key] = value
	}

	data, err = json.Marshal(document)
	if err != nil {
		return fmt.Errorf("failed to encode document %s: %w", name, err)
	}

	if err := store.Schema.Validate(data); err != nil {
		return fmt.Errorf(
			"updated document %s does not match schema: %w",
			name,
			err,
		)
	}

	data, err = json.MarshalIndent(document, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to encode document %s: %w", name, err)
	}

	if err := writeFileAtomic(path, data, 0644); err != nil {
		return fmt.Errorf("failed to update document %s: %w", name, err)
	}

	return nil
}

func (store *Store) Delete(name string) error {
	path, err := store.documentPath(name)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("document not found: %s", name)
		}

		return fmt.Errorf("failed to delete document %s: %w", name, err)
	}

	return nil
}

func (store *Store) Get(name string) (json.RawMessage, error) {
	path, err := store.documentPath(name)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("document not found: %s", name)
		}

		return nil, fmt.Errorf("failed to read document %s: %w", name, err)
	}

	if !json.Valid(data) {
		return nil, fmt.Errorf("document contains invalid JSON: %s", name)
	}

	return json.RawMessage(data), nil
}

func (store *Store) List() ([]string, error) {
	if err := store.ensureDir(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read records directory: %w", err)
	}

	documents := make([]string, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		documents = append(
			documents,
			entry.Name()[:len(entry.Name())-len(".json")],
		)
	}

	sort.Strings(documents)

	return documents, nil
}

func (store *Store) ensureDir() error {
	if err := os.MkdirAll(store.Dir, 0755); err != nil {
		return fmt.Errorf("failed to create records directory: %w", err)
	}

	return nil
}

func (store *Store) documentPath(name string) (string, error) {
	if !validDocumentName.MatchString(name) {
		return "", fmt.Errorf("invalid document name: %s", name)
	}

	return filepath.Join(store.Dir, name+".json"), nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	file, err := os.CreateTemp(dir, ".record-*")
	if err != nil {
		return err
	}

	tempPath := file.Name()

	defer func() {
		file.Close()
		os.Remove(tempPath)
	}()

	if err := file.Chmod(perm); err != nil {
		return err
	}

	if _, err := file.Write(data); err != nil {
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	return os.Rename(tempPath, path)
}
