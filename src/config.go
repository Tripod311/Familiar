package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

type ModuleConfiguration struct {
	Name          string          `json:"name"`
	Configuration json.RawMessage `json:"configuration"`
}

type AppConfiguration struct {
	Engine       string                         `json:"engine"`
	Model        string                         `json:"model"`
	Port         int                            `json:"port"`
	LoopLimit    uint                           `json:"loopLimit"`
	Temperature  float64                        `json:"temperature"`
	TopP         float64                        `json:"top_p"`
	MaxTokens    uint                           `json:"max_tokens"`
	Verbose      bool                           `json:"verbose"`
	StartTimeout uint                           `json:"startTimeout"`
	Main         ModuleConfiguration            `json:"main"`
	Helpers      map[string]ModuleConfiguration `json:"helpers"`
}

type Configuration struct {
	EnginesDir string           `json:"enginesDir"`
	ModelsDir  string           `json:"modelsDir"`
	ModulesDir string           `json:"modulesDir"`
	App        AppConfiguration `json:"app"`
}

func parseConfig(configPath string) Configuration {
	if len(configPath) == 0 {
		configPath = "config.json"
	}

	configPath, err := ResolveAppPath(configPath)
	if err != nil {
		log.Fatalf("can't resolve configuration path: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("can't read configuration file %q: %v", configPath, err)
	}

	var result Configuration

	if err := json.Unmarshal(data, &result); err != nil {
		log.Fatalf("corrupted configuration file %q: %v", configPath, err)
	}

	configDir := filepath.Dir(configPath)

	result.EnginesDir = resolveConfigPath(configDir, result.EnginesDir)
	result.ModelsDir = resolveConfigPath(configDir, result.ModelsDir)
	result.ModulesDir = resolveConfigPath(configDir, result.ModulesDir)

	return result
}

func resolveConfigPath(configDir, configuredPath string) string {
	if filepath.IsAbs(configuredPath) {
		return filepath.Clean(configuredPath)
	}

	return filepath.Join(configDir, configuredPath)
}
