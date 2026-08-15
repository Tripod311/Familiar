package api

import (
	"fmt"
	"net/http"
	"tripod311/familiar/engine"
)

type API struct {
	Port      int
	Host      string
	SPAServer SPAServer
	Engine    engine.Engine
}

func (api *API) Listen() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", api.SPAServer.Serve)

	http.ListenAndServe(api.Host+":"+fmt.Sprint(api.Port), nil)
}

func NewAPI(port int, host string, clientDir string, runner engine.Engine) *API {
	return &API{
		Port: port,
		Host: host,
		SPAServer: SPAServer{
			rootDir: http.Dir(clientDir),
		},
		Engine: runner,
	}
}
