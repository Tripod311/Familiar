# Building an Application

A Familiar application is assembled from an inference engine, a main module, optional helper modules, model files, and a configuration file.

There is intentionally very little packaging logic involved. Building an application mostly means placing the required components into directories and describing how they should be connected.

Familiar does not attempt to verify whether modules are semantically compatible with each other. The person assembling the application is responsible for understanding what each module expects and what it provides.

## Application layout

A typical application directory may look like this:

```text
familiar/
├── familiar
├── configuration.json
├── engines/
│   └── ubuntu-vulkan-llama/
│       ├── llama-server
│       └── manifest.json
├── models/
│   └── Qwen3-1.7B-Q8_0.gguf
└── modules/
    ├── ui/
    │   ├── familiar_ui
    │   └── manifest.json
    ├── history/
    │   ├── history
    │   └── manifest.json
    ├── context/
    │   ├── context
    │   └── manifest.json
    └── records/
        ├── records
        ├── manifest.json
        └── schema.json
```

The exact contents depend on the application.

A very small application may contain only a main module and an inference engine. Other applications may use several helper modules for history, context management, records, knowledge retrieval, or other tasks.

## Main configuration

The application is configured through a JSON file.

A relatively complete example looks like this:

```json
{
    "enginesDir": "./engines",
    "modelsDir": "./models",
    "modulesDir": "./modules",

    "app": {
        "port": 14000,
        "verbose": true,
        "startTimeout": 60,
        "temperature": 0.1,
        "top_p": 0.8,
        "max_tokens": 512,

        "engine": "ubuntu-vulkan-llama",
        "model": "Qwen3-1.7B-Q8_0.gguf",

        "main": {
            "name": "ui",
            "configuration": {
                "port": 8080,
                "history": "short_history",
                "context": "context"
            }
        },

        "helpers": {
            "short_history": {
                "name": "history",
                "configuration": {
                    "size": 20,
                    "store": true
                }
            },

            "context": {
                "name": "context",
                "configuration": {
                    "store": true,
                    "useMemory": false
                }
            },

            "records": {
                "name": "records",
                "configuration": {}
            },

            "datetime": {
                "name": "datetime",
                "configuration": {}
            }
        }
    }
}
```

## Directory settings

The top-level directory fields tell Familiar where to find application components.

### `enginesDir`

Directory containing inference engines.

Each engine is stored in its own subdirectory together with a manifest describing how it should be launched.

Example:

```text
engines/
├── ubuntu-vulkan-llama/
├── ubuntu-cpu-llama/
└── openrouter/
```

### `modelsDir`

Directory containing model files.

Model files do not require manifests.

For a `llama.cpp` based engine, this directory may contain GGUF files:

```text
models/
├── Qwen3-1.7B-Q8_0.gguf
└── another-model.gguf
```

Some inference engines do not use local model files at all. For example, an OpenRouter-backed engine may select its model through its own configuration.

### `modulesDir`

Directory containing application modules.

Each module lives in its own directory and has a manifest describing how Familiar should start it.

Example:

```text
modules/
├── ui/
├── history/
├── context/
├── records/
└── datetime/
```

## Application settings

The `app` object describes the current application build.

### `port`

```json
"port": 14000
```

Internal port assigned to the inference engine.

Familiar starts the configured engine as a subprocess and passes this port to it.

The engine is expected to expose its HTTP API on this port.

### `verbose`

```json
"verbose": true
```

Controls whether inference engine output is shown in the Familiar output.

This is useful while developing or debugging an engine. It can be disabled when engine logs are not needed.

### `startTimeout`

```json
"startTimeout": 60
```

Maximum number of seconds Familiar waits for the inference engine to become ready.

During startup, Familiar checks the engine's health endpoint.

If the engine does not become available before the timeout expires, application startup fails.

This is particularly relevant for local models, where loading weights may take some time.

### Inference parameters

The following fields are passed as inference parameters:

```json
"temperature": 0.1,
"top_p": 0.8,
"max_tokens": 512
```

These correspond to the usual LLM inference settings.

Their exact effect may depend on the selected inference engine and model.

### `engine`

```json
"engine": "ubuntu-vulkan-llama"
```

Selects an engine from `enginesDir`.

The value refers to the engine directory name.

For example:

```text
engines/
└── ubuntu-vulkan-llama/
```

is selected with:

```json
"engine": "ubuntu-vulkan-llama"
```

### `model`

```json
"model": "Qwen3-1.7B-Q8_0.gguf"
```

Selects a model file from `modelsDir`.

This is mainly used by local inference engines such as `llama.cpp`.

For an engine that manages model selection itself, such as an OpenRouter proxy, this field may be left empty:

```json
"model": ""
```

The meaning of the field ultimately depends on the engine manifest and its arguments.

## Main module

Every Familiar application has one main module.

Example:

```json
"main": {
    "name": "ui",
    "configuration": {
        "port": 8080,
        "history": "short_history",
        "context": "context"
    }
}
```

The `name` field selects a module from `modulesDir`.

In this example:

```text
modules/
└── ui/
```

is used as the main module.

The `configuration` object is passed directly to the module during its `load` phase.

Familiar itself does not interpret this configuration.

Its structure is defined entirely by the module.

For example, the UI module above happens to expect:

```json
{
    "port": 8080,
    "history": "short_history",
    "context": "context"
}
```

Another main module may expect completely different settings.

The main module is usually responsible for the primary application workflow. It may expose a UI, an HTTP API, a CLI, or any other application entry point.

## Helper modules

Additional modules are configured under `helpers`.

Example:

```json
"helpers": {
    "short_history": {
        "name": "history",
        "configuration": {
            "size": 20,
            "store": true
        }
    },

    "records": {
        "name": "records",
        "configuration": {}
    }
}
```

Each helper entry has two names with different purposes.

In:

```json
"short_history": {
    "name": "history"
}
```

`short_history` is the identifier used by this application.

`history` is the module directory that should be loaded from `modulesDir`.

This allows the same module implementation to potentially be used more than once with different configurations.

For example, an application could theoretically contain:

```json
"helpers": {
    "short_history": {
        "name": "history",
        "configuration": {
            "size": 20
        }
    },

    "long_history": {
        "name": "history",
        "configuration": {
            "size": 200
        }
    }
}
```

Whether such a configuration is useful depends on the modules involved.

Like the main module configuration, helper `configuration` objects are passed directly to the corresponding module.

Familiar does not interpret their contents.

## Inference engines

An inference engine is a subprocess that exposes an OpenAI-compatible HTTP interface.

At minimum, Familiar needs to know:

* how to start the engine;
* where its health endpoint is;
* where its chat completion endpoint is.

Each engine lives in its own directory under `enginesDir`.

Example:

```text
engines/
└── ubuntu-vulkan-llama/
    ├── llama-server
    └── manifest.json
```

An engine manifest may look like this:

```json
{
    "name": "llama.cpp CPU",
    "description": "CPU version",
    "exec": "llama-server",

    "apiPaths": {
        "health": "/health",
        "request": "/v1/chat/completions"
    },

    "args": [
        "-m",
        "%MODEL%",
        "--host",
        "127.0.0.1",
        "--port",
        "%PORT%",
        "--no-ui"
    ]
}
```

### Engine manifest fields

#### `name`

Human-readable engine name.

```json
"name": "llama.cpp CPU"
```

It is primarily descriptive.

#### `description`

Optional human-readable description of the engine.

```json
"description": "CPU version"
```

#### `exec`

Executable that should be started.

```json
"exec": "llama-server"
```

The executable is resolved relative to the engine directory.

An engine does not have to be `llama.cpp`. It may be any program capable of exposing the expected HTTP endpoints.

For example, an engine may also be a small proxy to a remote inference service.

#### `apiPaths`

Defines HTTP paths used by Familiar.

```json
"apiPaths": {
    "health": "/health",
    "request": "/v1/chat/completions"
}
```

`health` is used during startup to determine whether the engine is ready.

`request` is used for inference requests.

#### `args`

Command-line arguments passed to the engine.

```json
"args": [
    "-m",
    "%MODEL%",
    "--host",
    "127.0.0.1",
    "--port",
    "%PORT%",
    "--no-ui"
]
```

Familiar substitutes special placeholders before starting the process.

`%PORT%` is replaced with the application inference port.

`%MODEL%` is replaced with the selected model path.

This makes it possible to describe different engine launch commands without adding engine-specific behavior to the Familiar core.

## Remote inference engines

An inference engine does not need to perform inference locally.

For example, an engine can be a small proxy that listens on the local port expected by Familiar and forwards requests to OpenRouter or another remote API.

From the core runtime's perspective, both cases look similar:

```text
Familiar
    |
    v
Local inference engine
    |
    v
Local model
```

or:

```text
Familiar
    |
    v
Local proxy engine
    |
    v
Remote LLM provider
```

This allows the rest of the application to remain unchanged when switching inference backends.

Local modules such as history, records, and knowledge storage may still remain local even when inference itself is remote.

## Modules

A module is a standalone subprocess.

Each module lives in its own directory under `modulesDir`.

Example:

```text
modules/
└── ui/
    ├── familiar_ui
    └── manifest.json
```

A module manifest may look like this:

```json
{
    "name": "ui",
    "description": "Simple ui",
    "exec": "./familiar_ui",
    "args": []
}
```

### Module manifest fields

#### `name`

Human-readable module name.

```json
"name": "ui"
```

#### `description`

Optional description of the module.

```json
"description": "Simple ui"
```

#### `exec`

Program that should be started.

```json
"exec": "./familiar_ui"
```

This may be a native executable, but it does not need to be one.

For example, a Python module could be launched through a bundled virtual environment.

Conceptually:

```json
{
    "exec": "./venv/bin/python",
    "args": [
        "my_module.py"
    ]
}
```

A module may therefore be implemented in any language or runtime that can communicate using the Familiar module protocol.

#### `args`

Additional command-line arguments passed to the module.

```json
"args": []
```

Arguments are passed in the order specified by the manifest.

## Module working directory

A module is started with its own module directory as the current working directory.

For example:

```text
modules/
└── history/
    ├── history
    ├── manifest.json
    └── dump.json
```

When the `history` module is running, its working directory is:

```text
modules/history/
```

This is intentional.

It allows a small module to keep its own files next to its executable without requiring additional path configuration.

A module may therefore use simple relative paths such as:

```text
dump.json
schema.json
docs/
cache/
configuration.json
```

This also makes modules easier to move together with their data.

For example, copying a module directory may copy both the module implementation and its persistent local state.

The exact storage strategy remains up to the module.

## Model files

Model weights are simpler than engines and modules.

They are placed directly under `modelsDir` and do not require manifests.

Example:

```text
models/
├── Qwen3-1.7B-Q8_0.gguf
├── Qwen3-4B-Q5_K_M.gguf
└── tiny-test-model.gguf
```

The configured engine determines how the model file is used.

For a `llama.cpp` engine, the selected path may be substituted into `%MODEL%`.

For an engine that does not use local weights, the model configuration may be ignored or left empty.

## Building an application

There is no dedicated package format required for a Familiar application.

A basic build process is:

1. Create directories for engines, models, and modules.
2. Place the required inference engine into `enginesDir`.
3. Add an engine manifest.
4. Place any required model weights into `modelsDir`.
5. Place the main module and helper modules into `modulesDir`.
6. Add a manifest for each module.
7. Create the main Familiar configuration.
8. Start Familiar.

For example:

```text
my-app/
├── familiar
├── configuration.json
├── engines/
├── models/
└── modules/
```

The application configuration then connects these components together.

## Module compatibility

Familiar intentionally performs only limited orchestration.

It does not attempt to determine whether two modules are logically compatible.

For example, a UI module may expect helper modules named:

```text
history
context
```

while another main module may expect completely different RPC methods or helper names.

It is therefore the responsibility of the application author to know:

* which RPC methods a module exposes;
* which helper modules it expects;
* which tools it provides;
* what configuration format it accepts;
* what persistent files it requires;
* whether it expects another module with a particular behavior.

The reference modules included with Familiar are examples and reusable building blocks, not a universal compatibility standard.

It is completely valid to build a different set of modules that cooperate with each other but are not compatible with the reference modules.

## Application-specific modules

A Familiar build does not need to use generic modules.

For a particular application, it may be simpler to write several modules specifically designed to work together.

For example:

```text
modules/
├── customer_api/
├── customer_memory/
├── report_generator/
└── company_database/
```

These modules may use their own conventions and internal RPC methods.

Familiar only provides the process orchestration and common protocol mechanisms.

The architecture inside the application remains intentionally flexible.

## Trust and security

Modules are ordinary subprocesses.

They run with the same operating-system permissions as the user running Familiar.

A module can therefore potentially:

* read and write accessible files;
* access the network;
* start other processes;
* modify its own files;
* interact with other system resources available to the user.

Familiar does not sandbox modules or attempt to restrict their behavior.

For this reason, modules and prebuilt Familiar distributions should only be used from sources you trust.

The same applies to inference engines: they are also executable programs launched by Familiar.

This trust model is intentional and keeps the runtime relatively small, but it means Familiar should not be treated as a secure environment for executing untrusted plugins.

## A note on portability

Because engines and modules are external processes, application bundles may need platform-specific executables.

For example, a build intended for Linux and Windows may contain different engine binaries and module binaries for each operating system.

The module protocol itself is not tied to a particular language or runtime, but the programs implementing it must of course be executable on the target system.

This is one reason engine and module directories are kept separate from the Familiar core: different application distributions can package whatever implementations are appropriate for their target environment.

## Summary

A Familiar application is mostly a directory structure plus configuration:

```text
configuration
+
main module
+
optional helper modules
+
inference engine
+
optional local model
```

The core runtime connects these pieces but does not prescribe what the application should look like internally.

This makes it possible to build very small applications with only a few components, or experimental applications composed from several specialized modules, without requiring every build to adopt the same architecture.
