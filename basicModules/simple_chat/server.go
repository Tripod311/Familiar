package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
)

//go:embed client
var defaultClient embed.FS

type SPAServer struct {
	rootDir http.FileSystem
}

func (spa *SPAServer) Serve(w http.ResponseWriter, r *http.Request) {
	p := path.Clean(r.URL.Path)

	f, err := spa.rootDir.Open(p)
	if err == nil {
		info, statErr := f.Stat()
		f.Close()

		if statErr == nil && !info.IsDir() {
			http.FileServer(spa.rootDir).ServeHTTP(w, r)
			return
		}
	}

	r.URL.Path = "/index.html"
	http.FileServer(spa.rootDir).ServeHTTP(w, r)
}

type Server struct {
	Port      int    `json:"port"`
	ClientDir string `json:"clientDir"`
	statics   *SPAServer
	context   context.Context
	cancel    context.CancelFunc
	instance  *http.Server

	sendRequest func(string)
}

func Build(loadPacket json.RawMessage, sendRequest func(string)) *Server {
	var result Server
	json.Unmarshal(loadPacket, &result)

	if len(result.ClientDir) > 0 {
		result.statics = &SPAServer{
			rootDir: http.Dir(result.ClientDir),
		}
	} else {
		serverRoot, _ := fs.Sub(defaultClient, "client")

		result.statics = &SPAServer{
			rootDir: http.FS(serverRoot),
		}
	}
	result.sendRequest = sendRequest

	return &result
}

func (server *Server) Start() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", server.statics.Serve)
	mux.HandleFunc("/request", server.HandleRequest)

	server.context, server.cancel = context.WithCancel(
		context.Background(),
	)
	server.instance = &http.Server{
		Addr:    "0.0.0.0:" + fmt.Sprint(server.Port),
		Handler: mux,
	}

	server.serve()
}

func (server *Server) serve() {
	go func() {
		if err := server.instance.ListenAndServe(); err != nil {
			// shutdown immediately
		}
	}()

	<-server.context.Done()

	if err := server.instance.Shutdown(server.context); err != nil {
		// log error
	}
}

func (server *Server) Stop() {
	server.cancel()
}

// API handlers
func (server *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {

}
