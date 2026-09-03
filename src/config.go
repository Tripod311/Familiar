package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
)

type ModuleConfiguration struct {
	Name          string          `json:"name"`
	Configuration json.RawMessage `json:"configuration"`
}

type AppConfiguration struct {
	Engine       string                         `json:"engine"`
	Model        string                         `json:"model"`
	Port         int                            `json:"port"`
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

func parseConfig() Configuration {
	file, err := os.Open("./config.json")
	if err != nil {
		log.Fatal("configuration.json not found")
	}

	byteValue, err := io.ReadAll(file)
	if err != nil {
		log.Fatal("Can't read configuration file")
	}

	var result Configuration

	err = json.Unmarshal(byteValue, &result)
	if err != nil {
		log.Fatal("Corrupted configuration file")
	}

	return result
}
