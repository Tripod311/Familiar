package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

const (
	configPath    = "configuration.json"
	openRouterURL = "https://openrouter.ai/api/v1/chat/completions"
)

type Config struct {
	OpenRouterKey   string `json:"openRouterKey"`
	OpenRouterModel string `json:"openRouterModel"`
}

var config Config

var client = &http.Client{
	// No global timeout: LLM requests may take a long time.
	Timeout: 0,
}

func main() {
	port := flag.Int(
		"port",
		14000,
		"Local HTTP server port",
	)

	flag.Parse()

	if err := loadConfig(); err != nil {
		log.Fatal(err)
	}

	http.HandleFunc(
		"/health",
		handleHealth,
	)

	http.HandleFunc(
		"/v1/chat/completions",
		handleChatCompletions,
	)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)

	fmt.Fprintf(
		os.Stderr,
		"OpenRouter proxy listening on %s\n",
		addr,
	)

	server := &http.Server{
		Addr:    addr,
		Handler: nil,

		// Explicitly leave request timeouts disabled.
		ReadTimeout:  0,
		WriteTimeout: 0,
		IdleTimeout:  0,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func loadConfig() error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf(
			"failed to read %s: %w",
			configPath,
			err,
		)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf(
			"failed to decode %s: %w",
			configPath,
			err,
		)
	}

	if config.OpenRouterKey == "" {
		return fmt.Errorf("openRouterKey is empty")
	}

	if config.OpenRouterModel == "" {
		return fmt.Errorf("openRouterModel is empty")
	}

	return nil
}

func handleChatCompletions(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodPost {
		http.Error(
			writer,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(
			writer,
			fmt.Sprintf("failed to read request body: %s", err),
			http.StatusBadRequest,
		)
		return
	}

	var payload map[string]json.RawMessage

	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(
			writer,
			fmt.Sprintf("invalid JSON request: %s", err),
			http.StatusBadRequest,
		)
		return
	}

	model, err := json.Marshal(config.OpenRouterModel)
	if err != nil {
		http.Error(
			writer,
			fmt.Sprintf("failed to encode model: %s", err),
			http.StatusInternalServerError,
		)
		return
	}

	payload["model"] = model

	body, err = json.Marshal(payload)
	if err != nil {
		http.Error(
			writer,
			fmt.Sprintf("failed to encode request: %s", err),
			http.StatusInternalServerError,
		)
		return
	}

	proxyRequest, err := http.NewRequestWithContext(
		request.Context(),
		http.MethodPost,
		openRouterURL,
		bytes.NewReader(body),
	)
	if err != nil {
		http.Error(
			writer,
			fmt.Sprintf("failed to create OpenRouter request: %s", err),
			http.StatusInternalServerError,
		)
		return
	}

	proxyRequest.Header.Set(
		"Authorization",
		"Bearer "+config.OpenRouterKey,
	)

	proxyRequest.Header.Set(
		"Content-Type",
		"application/json",
	)

	response, err := client.Do(proxyRequest)
	if err != nil {
		http.Error(
			writer,
			fmt.Sprintf("OpenRouter request failed: %s", err),
			http.StatusBadGateway,
		)
		return
	}

	defer response.Body.Close()

	for key, values := range response.Header {
		for _, value := range values {
			writer.Header().Add(key, value)
		}
	}

	writer.WriteHeader(response.StatusCode)

	if _, err := io.Copy(writer, response.Body); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"failed to proxy OpenRouter response: %s\n",
			err,
		)
	}
}

func handleHealth(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodGet {
		http.Error(
			writer,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	writer.Header().Set("Content-Type", "application/json")

	_, err := writer.Write([]byte(`{"status":"ok"}`))
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"health response error: %s\n",
			err,
		)
	}
}
