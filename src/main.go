package familiar

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

type ModuleConfiguration struct {
	Name          string          `json:"name"`
	Configuration json.RawMessage `json:"configuration"`
}

type AppConfiguration struct {
	Engine       string                `json:"engine"`
	Model        string                `json:"model"`
	Port         int                   `json:"port"`
	Verbose      bool                  `json:"verbose"`
	StartTimeout uint                  `json:"startTimeout"`
	Main         ModuleConfiguration   `json:"main"`
	Helpers      []ModuleConfiguration `json:"helpers"`
}

type Configuration struct {
	EnginesDir string           `json:"enginesDir"`
	ModelsDir  string           `json:"modelsDir"`
	ModulesDir string           `json:"modulesDir"`
	App        AppConfiguration `json:"app"`
}

type Application struct {
	Model   *Model
	Main    *Module
	Helpers []*Module
}

func (app *Application) Cleanup() {
	for _, m := range app.Helpers {
		m.Stop()
	}

	app.Main.Stop()

	app.Model.Stop()
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

func LoadModel(config Configuration) (*Model, error) {
	file, err := os.Open(fmt.Sprintf("%s/%s/manifest.json", config.EnginesDir, config.App.Engine))
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
	srv.Exec = fmt.Sprintf("%s/%s/%s", config.EnginesDir, config.App.Engine, srv.Exec)

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

func main() {
	var app Application

	signals := make(chan os.Signal, 1)

	// create model
	config := parseConfig()
	model, err := LoadModel(config)
	if err != nil {
		log.Fatal(err)
	}

	// load main module
	mainModule, err := NewModule(fmt.Sprintf("%s/%s", config.ModulesDir, config.App.Main.Name))
	if err != nil {
		log.Fatal(err)
	}
	// load helpers
	helpers := make([]*Module, len(config.App.Helpers))

	for _, moduleConf := range config.App.Helpers {
		m, err := NewModule(fmt.Sprintf("%s/%s", config.ModulesDir, moduleConf.Name))
		if err != nil {
			log.Fatal(err)
		}
		helpers = append(helpers, m)
	}

	// start everything
	app.Model = model
	app.Main = mainModule
	app.Helpers = helpers
	defer app.Cleanup()

	app.Model.Start(60, true)
	err = app.Main.Start()
	if err != nil {
		log.Fatalf("Module (%s) failed to start: %s", app.Main.Name, err)
	}
	app.Main.Send("load", config.App.Main.Configuration)
	response := <-app.Main.RPCChan
	if response.Error != nil {
		log.Fatalf("Module (%s) failed to load: %s", app.Main.Name, response.Error.Message)
	}
	for index, h := range app.Helpers {
		err = h.Start()
		if err != nil {
			log.Fatalf("Helper (%s) failed to start: %s", h.Name, err)
		}
		h.Send("load", config.App.Helpers[index].Configuration)
		response := <-h.RPCChan
		if response.Error != nil {
			log.Fatalf("Helper (%s) failed to load: %s", h.Name, response.Error.Message)
		}
	}

	fmt.Println("Running")

mainLoop:
	for {
		select {
		case sig := <-signals:
			fmt.Printf("Received %s", sig)
			break mainLoop
		case req := <-app.Main.RPCChan:
			err := app.ProcessRequest(req)
			if err != nil {

			}
		}
	}
}

func (app *Application) ProcessRequest(rpc RPCPacket) error {
	switch rpc.Method {
	case "log":
		fmt.Println(string(rpc.Params))
	case "useHelper":
	case "useModel":
	}

	return nil
}
