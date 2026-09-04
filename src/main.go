package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
)

func LoadModel(config Configuration) (*Model, error) {
	engineDir := filepath.Join(
		config.EnginesDir,
		config.App.Engine,
	)

	manifestPath := filepath.Join(
		engineDir,
		"manifest.json",
	)

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf(
			"can't read inference engine %s manifest: %w",
			config.App.Engine,
			err,
		)
	}

	srv := NewServer()

	if err := json.Unmarshal(manifestData, srv); err != nil {
		return nil, fmt.Errorf(
			"inference engine %s has corrupted manifest: %w",
			config.App.Engine,
			err,
		)
	}

	srv.Port = config.App.Port
	srv.LoopLimit = config.App.LoopLimit
	srv.Temperature = config.App.Temperature
	srv.TopP = config.App.TopP
	srv.Exec = resolveConfigPath(engineDir, srv.Exec)

	modelPath := resolveConfigPath(
		config.ModelsDir,
		config.App.Model,
	)

	modelInfo, err := os.Stat(modelPath)
	if err != nil {
		return nil, fmt.Errorf(
			"model %s is not available: %w",
			config.App.Model,
			err,
		)
	}

	if modelInfo.IsDir() {
		return nil, fmt.Errorf(
			"model path %q points to a directory",
			modelPath,
		)
	}

	model := NewModel()
	model.Server = srv
	srv.Model = modelPath

	return model, nil
}

func run(configPath string) error {
	app := NewApplication()

	config := parseConfig(configPath)

	model, err := LoadModel(config)
	if err != nil {
		return err
	}

	mainModule, err := NewModule(
		filepath.Join(
			config.ModulesDir,
			config.App.Main.Name,
		),
	)
	if err != nil {
		return err
	}

	helpers := make(map[string]*Module)

	for key, moduleConf := range config.App.Helpers {
		module, err := NewModule(
			filepath.Join(
				config.ModulesDir,
				moduleConf.Name,
			),
		)
		if err != nil {
			return err
		}

		helpers[key] = module
	}

	app.Model = model
	app.Main = mainModule
	app.Helpers = helpers

	defer app.Cleanup()

	app.Main.On("packetReceived", app.ProcessMainEvent)
	app.Main.On("closed", app.ProcessModuleClosed)

	for _, module := range app.Helpers {
		module.On("closed", app.ProcessModuleClosed)
	}

	app.Model.Start(config.App.StartTimeout, config.App.Verbose)

	if err := app.Main.Start(); err != nil {
		return fmt.Errorf(
			"module %s failed to start: %w",
			app.Main.Name,
			err,
		)
	}

	for key, module := range app.Helpers {
		if err := module.Start(); err != nil {
			return fmt.Errorf(
				"helper %s (%s) failed to start: %w",
				key,
				module.Name,
				err,
			)
		}
	}

	for key, moduleConf := range config.App.Helpers {
		response, err := app.Helpers[key].Send(
			"load",
			moduleConf.Configuration,
		)
		if err != nil {
			return fmt.Errorf(
				"helper %s (%s) failed to load: %w",
				key,
				moduleConf.Name,
				err,
			)
		}

		if response.Error != nil {
			return fmt.Errorf(
				"helper %s (%s) failed to load: (%d) %s",
				key,
				moduleConf.Name,
				response.Error.Code,
				response.Error.Message,
			)
		}
	}

	response, err := app.Main.Send(
		"load",
		config.App.Main.Configuration,
	)
	if err != nil {
		return fmt.Errorf(
			"module %s failed to load: %w",
			app.Main.Name,
			err,
		)
	}

	if response.Error != nil {
		return fmt.Errorf(
			"module %s failed to load: (%d) %s",
			app.Main.Name,
			response.Error.Code,
			response.Error.Message,
		)
	}

	// collect functions
	for key, moduleConf := range config.App.Helpers {
		err := app.Helpers[key].GatherFunctions()
		if err != nil {
			return fmt.Errorf(
				"helper %s (%s) failed to gather functions: %w",
				key,
				moduleConf.Name,
				err,
			)
		}
	}

	err = app.Main.GatherFunctions()
	if err != nil {
		return fmt.Errorf(
			"module %s failed to load: (%d) %s",
			app.Main.Name,
			response.Error.Code,
			response.Error.Message,
		)
	}

	signals := make(chan os.Signal, 1)

	signal.Notify(
		signals,
		os.Interrupt,
	)
	defer signal.Stop(signals)

	fmt.Fprintln(os.Stderr, "Familiar started")

	select {
	case receivedSignal := <-signals:
		fmt.Fprintf(
			os.Stderr,
			"Received signal %s\n",
			receivedSignal,
		)

		app.Stop()

	case <-app.done:
		// Some module crashed
	}

	return nil
}

func main() {
	configPath := flag.String(
		"config",
		"",
		"path to the configuration file",
	)

	flag.Parse()

	if err := run(*configPath); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
