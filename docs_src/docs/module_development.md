# Developing a Module

Familiar itself is written in Go and provides a small Go SDK for implementing modules.

Several modules included in the repository are intentionally simple and can be used as examples when writing new ones.

A Familiar module is a standalone subprocess. It communicates with the Familiar core using JSON-RPC messages over standard input and standard output.

The module does not need to be written in Go. The Go SDK is only a convenience layer around the protocol.

## Module process model

When Familiar starts a module, it launches the command defined in the module manifest and connects to its:

* `stdin`
* `stdout`

These streams are used for JSON-RPC communication.

Conceptually:

```text
Familiar core
    |
    | JSON-RPC
    v
module process
```

The process may internally do anything appropriate for the application:

```text
module
├── local files
├── database
├── HTTP API
├── another subprocess
├── remote service
└── its own LLM requests
```

Familiar does not inspect or control the internal architecture of a module.

!!! warning

```
Standard output is used by the RPC protocol.

A module should not write ordinary logs or debugging output to `stdout`, as this may corrupt the RPC stream.

Use `stderr` for module logs instead.
```

## JSON-RPC protocol

Communication uses JSON-RPC 2.0-style packets.

A request looks approximately like this:

```json
{
    "jsonrpc": "2.0",
    "id": "_12",
    "method": "get_context",
    "params": {}
}
```

A successful response:

```json
{
    "jsonrpc": "2.0",
    "id": "_12",
    "result": {}
}
```

An error response:

```json
{
    "jsonrpc": "2.0",
    "id": "_12",
    "error": {
        "code": 1,
        "message": "Something went wrong"
    }
}
```

The Go SDK provides `RPCConnector` and `ExternalModule`, so modules written in Go normally do not need to work with these packets directly.

## Using the Go SDK

A minimal Go module looks like this:

```go
package main

import (
    "encoding/json"

    sdk "tripod311/familiar-sdk"
)

var module *sdk.ExternalModule

func main() {
    module = sdk.NewExternalModule()

    module.LoadHandle = Setup
    module.UnloadHandle = Shutdown
    module.MethodHandle = ProcessPacket
    module.GatherFunctions = GatherFunctions
    module.CallFunction = CallFunction

    module.Start()
}
```

`ExternalModule` creates the RPC connection over `stdin` and `stdout`, handles reserved Familiar methods, and dispatches application-specific requests to the provided handlers.

Not every handler is required.

A very small module may only implement `LoadHandle` and `MethodHandle`, while a module exposing LLM tools may also implement `GatherFunctions` and `CallFunction`.

## Module lifecycle

Familiar uses several reserved methods for module lifecycle and tool integration.

The most important are:

```text
load
unload
gatherFunctions
callFunction
```

These names are handled by the SDK and should not normally be implemented through `MethodHandle`.

### `load`

`load` is sent after the module process has started.

The module receives the `configuration` object specified for it in the Familiar application configuration.

For example:

```json
{
    "short_history": {
        "name": "history",
        "configuration": {
            "size": 20,
            "store": true
        }
    }
}
```

The history module receives:

```json
{
    "size": 20,
    "store": true
}
```

A Go module handles this using `LoadHandle`:

```go
func Setup(params json.RawMessage) (json.RawMessage, error) {
    if err := json.Unmarshal(params, &config); err != nil {
        return nil, err
    }

    return nil, nil
}
```

`load` is the normal place to:

* parse configuration;
* load persistent state;
* read prompts;
* open local files;
* prepare databases;
* initialize internal state.

Returning an error causes application startup to fail.

### `unload`

`unload` is sent when Familiar shuts the application down.

A module may use it to persist state or perform cleanup.

Example:

```go
func Shutdown(params json.RawMessage) (json.RawMessage, error) {
    if config.Store {
        Dump()
    }

    return nil, nil
}
```

For data that must survive unexpected process termination, it is usually better not to rely exclusively on `unload`. Persisting important state when it changes may be more appropriate.

### `gatherFunctions`

`gatherFunctions` asks the module which LLM tools it currently exposes.

In Go this is represented by:

```go
func GatherFunctions() ([]sdk.ToolDescription, error)
```

For example:

```go
func GatherFunctions() ([]sdk.ToolDescription, error) {
    return []sdk.ToolDescription{
        {
            Name:        "update_memory",
            Description: "Updates the persistent memory state.",
            Strict:      true,
            Parameters:  memorySchema,
        },
    }, nil
}
```

A tool description contains:

```go
type ToolDescription struct {
    Name        string
    Description string
    Strict      bool
    Parameters  json.RawMessage
}
```

`Parameters` contains the JSON schema used for the tool arguments.

The returned tools become available to the appropriate model requests.

A module may also return an empty list:

```go
return []sdk.ToolDescription{}, nil
```

if it does not expose any LLM tools.

### `callFunction`

When the model requests a tool exposed by the module, Familiar routes the call back to that module through `callFunction`.

The Go SDK exposes it as:

```go
func CallFunction(
    name string,
    arguments string,
) (json.RawMessage, error)
```

Example:

```go
func CallFunction(
    name string,
    arguments string,
) (json.RawMessage, error) {
    switch name {
    case "update_memory":
        memoryState = []byte(arguments)

        return nil, nil

    default:
        return nil, fmt.Errorf(
            "unknown function: %s",
            name,
        )
    }
}
```

`arguments` contains the JSON arguments generated by the model.

The module is responsible for parsing and validating them.

## Custom RPC methods

Modules can expose their own RPC methods in addition to LLM tools.

In the Go SDK these requests are handled by:

```go
MethodHandle func(
    method string,
    params json.RawMessage,
) (json.RawMessage, error)
```

For example, a context module may expose:

```go
func ProcessPacket(
    method string,
    params json.RawMessage,
) (json.RawMessage, error) {
    switch method {
    case "get_context":
        // Build context here.

        return result, nil

    default:
        return nil, fmt.Errorf(
            "unknown method: %s",
            method,
        )
    }
}
```

These methods are application-specific.

Familiar does not define what names such as:

```text
get_context
append
search
save
reset
```

mean.

Their meaning is part of the contract between the modules used by a particular application.

## Main modules and helper modules

Familiar distinguishes between one **main module** and optional **helper modules**.

They use the same subprocess protocol, but their role in the application is different.

## Main module

The main module controls the primary application workflow.

It may, for example:

* provide the user interface;
* expose an HTTP API;
* read context from helper modules;
* read and update history;
* build a model request;
* return the final model response to the user.

The main module has access to two core methods:

```text
moduleRequest
modelRequest
```

### `moduleRequest`

`moduleRequest` allows the main module to call an application-specific RPC method on a helper.

The request uses:

```go
type ModuleRequest struct {
    Module string          `json:"module"`
    Method string          `json:"method"`
    Params json.RawMessage `json:"params,omitempty"`
}
```

For example, a main module may append new messages to a history helper:

```go
func AppendHistory(messages []*sdk.Message) error {
    if len(config.History) == 0 {
        return nil
    }

    msgBytes, err := json.Marshal(messages)
    if err != nil {
        return err
    }

    request := sdk.ModuleRequest{
        Module: config.History,
        Method: "append",
        Params: msgBytes,
    }

    data, err := json.Marshal(request)
    if err != nil {
        return err
    }

    _, err = module.Send(
        "moduleRequest",
        data,
    )

    return err
}
```

Here:

```text
config.History
```

contains the helper identifier from the application configuration.

For example:

```json
{
    "helpers": {
        "short_history": {
            "name": "history",
            "configuration": {}
        }
    }
}
```

The module identifier is:

```text
short_history
```

The request:

```go
sdk.ModuleRequest{
    Module: "short_history",
    Method: "append",
    Params: ...
}
```

is routed by Familiar to that helper.

Conceptually:

```text
Main module
    |
    | moduleRequest
    v
Familiar
    |
    | append
    v
History helper
```

The helper does not need to know which module sent the request. It simply handles its own `append` method.

## Main module model requests

The main module can request inference using:

```text
modelRequest
```

For the main module, the request body is a JSON array of `sdk.Message`.

A typical workflow may look like:

```text
fetch context
    |
fetch history
    |
append user message
    |
modelRequest
    |
receive assistant message
    |
append conversation to history
```

For example:

```go
func SendRequest(message string) (string, error) {
    req, err := FetchContext()
    if err != nil {
        return "", err
    }

    history, err := FetchHistory()
    if err != nil {
        return "", err
    }

    req = append(req, history...)

    userMessage := sdk.Message{
        Role:    sdk.RoleUser,
        Content: message,
    }

    req = append(req, userMessage)

    data, err := json.Marshal(req)
    if err != nil {
        return "", err
    }

    response, err := module.Send(
        "modelRequest",
        data,
    )
    if err != nil {
        return "", err
    }

    var assistantMessage sdk.Message

    if err := json.Unmarshal(
        response,
        &assistantMessage,
    ); err != nil {
        return "", err
    }

    return assistantMessage.Content, nil
}
```

Familiar takes these messages, adds the tools exposed by the main module and helper modules, sends the resulting request to the configured inference engine, handles tool calls, and returns the final assistant message.

The main module therefore does not need to communicate directly with the inference server.

## Messages

The SDK uses an OpenAI-compatible message representation:

```go
type Message struct {
    Role       Role       `json:"role"`
    Content    string     `json:"content,omitempty"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID string     `json:"tool_call_id,omitempty"`
}
```

Available roles are:

```go
RoleSystem
RoleUser
RoleAssistant
RoleTool
```

A simple request may therefore be represented as:

```go
messages := []sdk.Message{
    {
        Role:    sdk.RoleSystem,
        Content: "You are a helpful assistant.",
    },
    {
        Role:    sdk.RoleUser,
        Content: "Hello.",
    },
}
```

Tool calls use the usual OpenAI-style structures exposed by the SDK.

## Helper modules

A helper module is intended to be an autonomous application component.

It may expose:

* custom RPC methods;
* LLM tools;
* persistent local state;
* its own internal logic.

Unlike the main module, a helper does not use `moduleRequest` to communicate with other helpers.

This keeps helpers independent from each other.

A helper can still use the inference engine itself.

## Helper model requests

A helper may send its own:

```text
modelRequest
```

Unlike the main module, a helper sends a `ServerRequest`.

For helper requests, the most important fields are:

```go
type ServerRequest struct {
    Tools    []Tool
    Messages []Message
}
```

A helper can therefore create an isolated model interaction with its own messages and its own tools.

Conceptually:

```text
Helper
   |
   | modelRequest
   v
Familiar
   |
   v
Inference engine
```

This request does **not** automatically inherit the tools or internal state of the main application.

For example, a document-search helper could provide only:

```text
search_documents
read_document
```

to its own LLM request.

The main application's tools remain unavailable unless the helper explicitly provides equivalent capabilities itself.

This allows a helper to behave as a small specialized agent while remaining isolated from the main agent.

A possible structure is:

```text
Main agent
    |
    | tool call
    v
Records helper
    |
    | isolated modelRequest
    v
Records search agent
```

The helper may use the same underlying inference engine as the main application, but the interaction has its own messages and capabilities.

## Example helper module

A context helper may combine several Familiar mechanisms.

For example:

```go
func main() {
    module = sdk.NewExternalModule()

    module.LoadHandle = Setup
    module.UnloadHandle = Shutdown
    module.MethodHandle = ProcessPacket
    module.GatherFunctions = GatherFunctions
    module.CallFunction = CallFunction

    module.Start()
}
```

Its `Setup` handler may load:

```text
prompt.md
memory.json
dump.json
```

Its custom RPC method:

```text
get_context
```

may return a system message to the main module.

At the same time, `GatherFunctions` may expose:

```text
update_memory
```

as an LLM tool.

The same module can therefore provide both:

```text
RPC service for the main module
+
LLM tool for the main agent
```

These mechanisms are independent.

A module may use either one or both.

## Failure model

Familiar currently uses a simple **all-or-nothing** process model.

If any critical application component stops unexpectedly, Familiar stops the whole application.

This includes:

* the inference engine;
* the main module;
* any configured helper module.

Conceptually:

```text
one required process fails
        |
        v
application stops
```

Familiar does not currently attempt production-style recovery such as:

* restarting individual modules;
* retrying module initialization;
* reconstructing partially failed application state;
* transparently replacing failed components.

This behavior is intentional for the current scope of the project.

Familiar is primarily intended for local and experimental applications, where stopping the application on component failure keeps lifecycle behavior relatively simple and predictable.

Applications that require more sophisticated recovery can implement additional behavior in their own components or extend the runtime.

## Implementing modules in other languages

The Go SDK is optional.

A module written in another language only needs to implement the same process protocol.

At a high level, it needs to:

1. Read newline-separated JSON-RPC packets from `stdin`.
2. Handle reserved Familiar methods as necessary.
3. Write JSON-RPC responses to `stdout`.
4. Keep ordinary logging on `stderr`.
5. Send requests such as `modelRequest` when supported by its role.

For example, a Python module may be launched using a bundled virtual environment:

```json
{
    "name": "python-module",
    "description": "Example Python module",

    "exec": "./venv/bin/python",

    "args": [
        "module.py"
    ]
}
```

Its directory might contain:

```text
python-module/
├── manifest.json
├── module.py
├── venv/
└── data/
```

Familiar does not care whether the process is implemented in Go, Python, Node.js, Rust, C++, or another language.

It only sees the RPC protocol.

This also means a module does not necessarily need to implement its functionality locally. It may simply act as an adapter to another process or network service:

```text
Familiar
    |
    v
module subprocess
    |
    v
remote service
```

## Choosing module boundaries

There is no required architecture for splitting an application into modules.

The reference modules demonstrate one possible approach:

```text
UI
History
Context
Knowledge
Records
Datetime
```

A different application may use completely different boundaries:

```text
HTTP API
Customer database
Report generator
Search service
```

Modules do not need to be compatible with the reference module set.

For application-specific builds, it may be more convenient to create several modules designed specifically to cooperate with each other.

The Familiar core only provides orchestration and communication. The architecture above that layer belongs to the application.

## Summary

A Familiar module is fundamentally:

```text
standalone process
+
JSON-RPC protocol
+
optional custom methods
+
optional LLM tools
+
optional model access
```

The Go SDK provides a convenient implementation of this protocol, but it is not required.

The main module coordinates the application and may communicate with helpers through `moduleRequest`.

Helpers are independent components. They may provide RPC methods and tools, and may create isolated LLM requests with their own messages and capabilities.

Familiar intentionally keeps the contract between these components small, leaving the internal design of each module up to its author.