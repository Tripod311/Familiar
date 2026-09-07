<p align="center">
  <img src="./assets/familiar.png" width="160">
</p>

# Familiar

Familiar is a lightweight runtime for building LLM-powered applications from independent modules.

Instead of providing a large agent framework with a predefined architecture, Familiar focuses on a small orchestration layer. Applications are assembled from standalone processes that communicate through a simple RPC protocol.

A module may provide a user interface, history storage, context management, persistent records, knowledge retrieval, system utilities, or any other functionality required by the application.

The inference backend is also replaceable. A Familiar application may use a local model server, a remote OpenAI-compatible API, or a custom inference adapter.

## Design goals

Familiar is designed around a few simple principles:

* Keep the core runtime small.
* Keep application components independent.
* Do not prescribe how modules should be implemented internally.
* Allow modules to be written in any language.
* Allow each application to use only the components it actually needs.
* Keep persistent application data local when possible.
* Make experimental LLM applications quick to assemble and modify.

Familiar is primarily intended as a flexible environment for experimentation and personal applications rather than as a production application framework.

## Architecture

A Familiar application consists of:

* one **main module**;
* one **inference engine**;
* zero or more **helper modules**.

The main module is responsible for the primary application workflow.

Helper modules provide additional capabilities. They may expose tools to the LLM, provide services directly to the main module, or use the inference engine themselves for isolated LLM tasks.

The exact structure is application-specific.

Familiar does not require applications to use history, context management, a UI, a knowledge base, or any other particular component.

## Modules

Modules are standalone processes launched by Familiar.

Communication between Familiar and a module happens through JSON RPC messages over the module's standard input and output streams.

Because modules communicate through a process boundary rather than through a language-specific API, their implementation language is not important.

For example, a module may be:

* a Go executable;
* a Python application running inside its own virtual environment;
* a Node.js program;
* a Rust or C++ binary;
* a proxy to an external HTTP service;
* an adapter around an existing command-line application.

As long as the process implements the Familiar module protocol, the core runtime does not need to know how it works internally.

## Main and helper modules

The main module is the central application component.

It may:

* communicate with the user or an external API;
* send requests to the inference engine;
* call helper modules;
* expose its own tools to the LLM;
* combine results from multiple components.

Helper modules are optional components that provide additional functionality.

A helper may expose LLM tools, such as:

```text
records_search
records_create
system_datetime
knowledge_search
```

A helper may also expose regular RPC methods intended for use by the main module.

Some helpers can make their own requests to the inference engine. These requests are isolated from the main agent and may use their own prompts and tools.

This allows a helper to act as a small specialized agent without gaining access to the main application's complete state or tool set.

## Inference engines

Inference is separated from the rest of the application.

The Familiar core communicates with an inference engine through an OpenAI-compatible chat completions interface.

This makes it possible to switch between different backends without changing application modules.

Examples include:

* a local `llama-server`;
* a local OpenAI-compatible model server;
* an OpenRouter proxy;
* another remote OpenAI-compatible provider;
* a custom inference adapter.

Application state and inference do not need to live in the same place.

For example, history, structured records, and knowledge files may remain on the local machine while only the context required for a specific request is sent to a remote model.

## Reference modules

The Familiar repository includes a set of small modules intended primarily as examples and reusable building blocks.

Examples include:

* simple chat UI;
* conversation history;
* context management;
* knowledge storage and retrieval;
* structured records;
* date and time utilities.

These modules are not part of a mandatory application architecture.

They are deliberately small and may be replaced, modified, combined differently, or ignored entirely.

An application that only needs to expose an HTTP endpoint and generate LLM responses does not need to load history, context, UI, or knowledge modules at all.

## Process-based composition

Familiar treats a module as a complete program rather than as a library loaded into the main process.

This approach has several useful properties:

* modules may use different languages and dependency ecosystems;
* modules can use completely different internal architectures;
* functionality can often be added without modifying the core runtime;
* dependencies remain isolated between components;
* existing programs and network services can be wrapped as Familiar modules.

The tradeoff is that modules communicate through RPC rather than direct function calls, and debugging multiple processes may require more effort than debugging a monolithic application.

For LLM applications, IPC overhead is generally insignificant compared with model inference and network latency.

## Trust model

Familiar does not sandbox modules.

A module is an ordinary process and runs with the permissions of the user who started Familiar.

Therefore, loading a module is equivalent to running any other executable on the system.

Only use modules and prebuilt Familiar distributions from sources you trust.

The Familiar runtime does not attempt to validate or restrict what third-party modules may do.

## What Familiar is not

Familiar is not intended to provide:

* a universal agent architecture;
* a secure plugin sandbox;
* a production workflow engine;
* a distributed execution platform;
* a complete memory or RAG framework;
* a mandatory abstraction for every LLM application.

Its purpose is smaller:

> Provide a lightweight runtime and protocol for composing LLM applications from independent processes.

The rest of the architecture belongs to the application.