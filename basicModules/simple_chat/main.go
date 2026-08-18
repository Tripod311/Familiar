package simple_chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var server *Server

func main() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req RPCRequest

		if err := decoder.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			fmt.Fprintln(os.Stderr, "decode error:", err)
			continue
		}

		switch req.Method {
		case "load":
			server = Build(req.Params, SendRequest)
			server.Start()
			encoder.Encode(RPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"success": true,
				},
			})
		case "unload":
			server.Stop()
			encoder.Encode(RPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"success": true,
				},
			})
		case "response":
			// pass response back
		default:
			encoder.Encode(RPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &RPCError{
					Code:    -32601,
					Message: "method not found",
				},
			})
		}
	}
}

func SendRequest(message string) {

}
