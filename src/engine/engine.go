package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

type ModelKey struct {
	EngineID string
	ModelID  string
}

type Engine struct {
	ModelsDir  string
	ChatsDir   string
	EnginesDir string
	PluginsDir string
	PortRange  *PortRange
	Chats      map[string]*Chat
	Models     map[ModelKey]*Model
	modelUsage map[ModelKey]uint
}

func NewEngine() *Engine {
	modelsDir, _ := filepath.Abs("../models")
	chatsDir, _ := filepath.Abs("../chats")
	enginesDir, _ := filepath.Abs("../inference_engines")
	pluginsDir, _ := filepath.Abs("../plugins")
	return &Engine{
		ModelsDir:  modelsDir,
		ChatsDir:   chatsDir,
		EnginesDir: enginesDir,
		PluginsDir: pluginsDir,
		PortRange:  NewPortRange(14000, 14100),
		Chats:      make(map[string]*Chat),
		Models:     make(map[ModelKey]*Model),
		modelUsage: make(map[ModelKey]uint),
	}
}

func (engine *Engine) ListServers() []string {
	entries, err := os.ReadDir(engine.EnginesDir)
	if err != nil {
		log.Printf("Error on reading engines dir: %s", err)
		return make([]string, 0)
	}

	var result []string

	for _, entry := range entries {
		if entry.IsDir() {
			path, err := filepath.Abs(engine.EnginesDir + "/" + entry.Name())
			if err != nil {
				continue
			}
			file, err := os.Open(path + "/manifest.json")
			if err != nil {
				continue
			}

			byteValue, err := io.ReadAll(file)
			if err != nil {
				continue
			}

			var binary Server

			err = json.Unmarshal(byteValue, &binary)
			if err != nil {
				continue
			}

			result = append(result, binary.Name)
		}
	}

	return result
}

func (engine *Engine) ListModels() []Model {
	entries, err := os.ReadDir(engine.ModelsDir)
	if err != nil {
		log.Printf("Error on reading models dir: %s", err)
		return make([]Model, 0)
	}

	var result []Model

	for _, entry := range entries {
		if entry.IsDir() {
			path, err := filepath.Abs(engine.ModelsDir + "/" + entry.Name())
			if err != nil {
				continue
			}
			file, err := os.Open(path + "/manifest.json")
			if err != nil {
				continue
			}

			byteValue, err := io.ReadAll(file)
			if err != nil {
				continue
			}

			var model Model

			err = json.Unmarshal(byteValue, &model)
			if err != nil {
				continue
			}

			result = append(result, model)
		}
	}

	return result
}

func (engine *Engine) ListChats() []Chat {
	entries, err := os.ReadDir(engine.ChatsDir)
	if err != nil {
		log.Printf("Error on reading chats dir: %s", err)
		return make([]Chat, 0)
	}

	var result []Chat

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path, err := filepath.Abs(engine.ChatsDir + "/" + entry.Name())

		if err != nil {
			continue
		}

		file, err := os.Open(path)

		if err != nil {
			continue
		}

		byteValue, err := io.ReadAll(file)
		if err != nil {
			continue
		}

		var desc Chat

		err = json.Unmarshal(byteValue, &desc)
		if err != nil {
			continue
		}
		desc.ID = entry.Name()

		result = append(result, desc)
	}

	return result
}

func (engine *Engine) GetChat(fileName string) (*Chat, error) {
	chat, exists := engine.Chats[fileName]
	if exists {
		return chat, nil
	}

	file, err := os.Open(engine.ChatsDir + "/" + fileName)

	if err != nil {
		return nil, fmt.Errorf("Chat configuration not found: %s", err)
	}

	byteValue, err := io.ReadAll(file)

	if err != nil {
		return nil, fmt.Errorf("Failed to read chat configuration: %s", err)
	}

	chat = NewChat()

	err = json.Unmarshal(byteValue, &chat)
	if err != nil {
		return nil, fmt.Errorf("Corrupted chat configuration: %s", err)
	}

	key := ModelKey{
		EngineID: chat.EngineID,
		ModelID:  chat.ModelID,
	}

	model, exists := engine.Models[key]
	if exists {
		engine.modelUsage[key]++
	} else {
		model, err = engine.loadModel(key.ModelID, key.EngineID)
		if err != nil {
			return nil, fmt.Errorf("Failed to load model %s: %s", key.ModelID, err)
		}
		engine.Models[key] = model
		engine.modelUsage[key] = 1
		model.Start()
	}
	chat.SetModel(model)

	engine.Chats[fileName] = chat

	return chat, nil
}

func (engine *Engine) CloseChat(fileName string) {
	chat, exists := engine.Chats[fileName]
	if exists {
		chat.Close()
		key := ModelKey{
			EngineID: chat.EngineID,
			ModelID:  chat.ModelID,
		}
		engine.modelUsage[key]--
		if engine.modelUsage[key] == 0 {
			engine.Models[key].Stop()
		}
	}
}

func (engine *Engine) loadModel(modelID, engineID string) (*Model, error) {
	file, err := os.Open(fmt.Sprintf("%s/%s/manifest.json", engine.EnginesDir, engineID))
	if err != nil {
		return nil, fmt.Errorf("Can't find inference engine %s: %s", engineID, err)
	}

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("Can't read inference engine %s manifest: %s", engineID, err)
	}

	srv := NewServer()
	err = json.Unmarshal(byteValue, &srv)
	if err != nil {
		return nil, fmt.Errorf("Inference engine %s corrupted manifest: %s", engineID, err)
	}

	srv.Port = engine.PortRange.Acquire()
	srv.Exec = fmt.Sprintf("%s/%s/%s", engine.EnginesDir, engineID, srv.Exec)

	file, err = os.Open(fmt.Sprintf("%s/%s/manifest.json", engine.ModelsDir, modelID))
	if err != nil {
		return nil, fmt.Errorf("Can't find model %s: %s", modelID, err)
	}

	byteValue, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("Can't read model %s manifest: %s", modelID, err)
	}

	model := NewModel()
	err = json.Unmarshal(byteValue, &model)
	if err != nil {
		return nil, fmt.Errorf("Model %s corrupted manifest: %s", engineID, err)
	}

	model.Server = srv
	model.FileName = fmt.Sprintf("%s/%s/%s", engine.ModelsDir, modelID, model.FileName)
	srv.Model = model.FileName

	return model, nil
}

func (engine *Engine) Close() {
	for _, chat := range engine.Chats {
		chat.Close()
	}
	for _, model := range engine.Models {
		model.Stop()
	}
}
