package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	// "tripod311/familiar/api"
	"tripod311/familiar/engine"
)

type Configuration struct {
	ClientDir     string `json:"clientDir"`
	EnginesDir    string `json:"enginesDir"`
	ModelsDir     string `json:"modelsDir"`
	PluginsDir    string `json:"pluginsDir"`
	ChatsDir      string `json:"chatsDir"`
	APIHost       string `json:"apiHost"`
	APIPort       int    `json:"apiPort"`
	ServerMinPort int    `json:"serverMinPort"`
	ServerMaxPort int    `json:"serverMaxPort"`
}

func parseConfig() (*Configuration, error) {
	file, err := os.Open("./config.json")
	if err != nil {
		return nil, fmt.Errorf("Can't find configuration file, using default configuration")
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

	return &result, nil
}

func main() {
	var runner *engine.Engine

	config, err := parseConfig()
	if err != nil {
		log.Print(err)
		runner = engine.NewEngine()
	} else {
		runner = engine.NewEngine()
		path, err := filepath.Abs(config.ChatsDir)
		if err == nil {
			runner.ChatsDir = path
		}
		path, err = filepath.Abs(config.ModelsDir)
		if err == nil {
			runner.ModelsDir = path
		}
		path, err = filepath.Abs(config.EnginesDir)
		if err == nil {
			runner.EnginesDir = path
		}
		path, err = filepath.Abs(config.PluginsDir)
		if err == nil {
			runner.PluginsDir = path
		}
		runner.PortRange.MinPort = config.ServerMinPort
		runner.PortRange.MaxPort = config.ServerMaxPort
		runner.PortRange.LastPort = config.ServerMinPort
	}

	// apiServer := api.NewAPI(config.APIPort, config.APIHost, config.ClientDir, runner)
	// apiServer.Listen()
	chat, err := runner.GetChat("test.json")
	if err != nil {
		log.Fatal(err)
	}
	response, err := chat.Request("Hello")
	if err != nil {
		log.Fatalf("Error: %s", err)
	} else {
		log.Print(response)
	}
	runner.Close()
}
