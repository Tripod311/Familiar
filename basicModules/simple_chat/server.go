package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"time"
)

//go:embed client
var defaultClient embed.FS

type SPAServer struct {
	rootDir http.FileSystem
}

func (spa *SPAServer) Serve(w http.ResponseWriter, r *http.Request) {
	fileServer := http.FileServer(spa.rootDir)
	p := path.Clean(r.URL.Path)

	f, err := spa.rootDir.Open(p)
	if err == nil {
		info, statErr := f.Stat()
		f.Close()

		if statErr == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
	}

	fallbackRequest := r.Clone(r.Context())
	fallbackRequest.URL.Path = "/"
	fallbackRequest.URL.RawPath = ""

	fileServer.ServeHTTP(w, fallbackRequest)
}

type Server struct {
	Port      int    `json:"port"`
	ClientDir string `json:"clientDir"`
	statics   *SPAServer
	context   context.Context
	cancel    context.CancelFunc
	instance  *http.Server

	sendRequest func(string) (string, error)
}

func NewServer(loadPacket json.RawMessage, sendRequest func(string) (string, error)) *Server {
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

func (server *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/request", server.HandleRequest)
	mux.HandleFunc("/", server.statics.Serve)

	addr := fmt.Sprintf("0.0.0.0:%d", server.Port)

	listener, err := net.Listen("tcp4", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		server.Port = tcpAddr.Port
	}

	server.instance = &http.Server{
		Addr:    listener.Addr().String(),
		Handler: mux,
	}

	fmt.Fprintf(
		os.Stderr,
		"server started: http://127.0.0.1:%d (listening on %s)\n",
		server.Port,
		listener.Addr(),
	)

	go func() {
		err := server.instance.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "server error:", err)
		}
	}()

	return nil
}

func (server *Server) Stop() error {
	if server.instance == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	return server.instance.Shutdown(ctx)
}

// API handlers
func (server *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Message string `json:"message"`
	}

	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if body.Message == "" {
		http.Error(w, `field "message" is required`, http.StatusBadRequest)
		return
	}

	res, err := server.sendRequest(body.Message)
	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": res,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "response encode error:", err)
	}
}
