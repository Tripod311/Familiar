<p align="center">
  <img src="docs/assets/familiar.png" alt="Familiar" width="180">
</p>

# Familiar

Familiar is a lightweight runtime for building LLM-powered applications from independent modules.

Applications are composed from standalone subprocesses connected through a simple JSON-RPC protocol. Modules can provide UI, history, context management, tools, local storage, knowledge retrieval, remote service adapters, or any other functionality required by the application.

Familiar does not prescribe a fixed application architecture. A build may use only the components it needs, and modules may be implemented in any language as long as they follow the Familiar module protocol.

Inference is also replaceable. Familiar can work with local OpenAI-compatible servers such as `llama.cpp`, as well as proxy engines for remote providers.

The project is experimental and primarily intended for local applications, prototyping, and experimentation with modular LLM architectures.

[Demos](https://tripod311.github.io/Familiar/demos/)
[Documentation](https://tripod311.github.io/Familiar/)