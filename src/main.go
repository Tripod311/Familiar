package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func LoadModel(config Configuration) (*Model, error) {
	path := fmt.Sprintf("%s/%s/manifest.json", config.EnginesDir, config.App.Engine)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("filepath.Abs %s: %s", config.App.Engine, err)
	}

	file, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("Can't find inference engine %s: %s", config.App.Engine, err)
	}

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("Can't read inference engine %s manifest: %s", config.App.Engine, err)
	}

	srv := NewServer()
	err = json.Unmarshal(byteValue, &srv)
	if err != nil {
		return nil, fmt.Errorf("Inference engine %s corrupted manifest: %s", config.App.Engine, err)
	}

	srv.Port = config.App.Port
	srv.LoopLimit = config.App.LoopLimit
	path = fmt.Sprintf("%s/%s/%s", config.EnginesDir, config.App.Engine, srv.Exec)
	absPath, err = filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("filepath.Abs %s: %s", config.App.Engine, err)
	}
	srv.Exec = absPath

	modelPath, err := filepath.Abs(fmt.Sprintf("%s/%s", config.ModelsDir, config.App.Model))
	if err != nil {
		return nil, fmt.Errorf("Model %s, can't get absolute path: %s", config.App.Model, err)
	}
	file, err = os.Open(modelPath)
	if err != nil {
		return nil, fmt.Errorf("Model %s is not loaded: %s", config.App.Model, err)
	}

	model := NewModel()
	model.Server = srv
	srv.Model = modelPath

	return model, nil
}

func run() error {
	app := NewApplication()

	config := parseConfig()

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
		syscall.SIGTERM,
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
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
