# Demo bundles

Familiar demo bundles are preconfigured applications built from the same runtime and module system.

They are intended to show different ways Familiar can be composed without requiring you to assemble an application manually.

## Basic local assistant

A minimal general-purpose assistant with:

- short message history;
- persistent context memory;
- system date/time tool;
- local inference.

This bundle is completely self-contained. It does not require an API key, external service, or additional model download.

The bundled model is:

`Qwen3-0.6B-Q8_0.gguf`

It is intentionally very small so the demo remains reasonably lightweight and can run on modest hardware.

The model is suitable for testing the runtime, history, memory, tool calls, and local inference, but it is extremely limited compared to larger models. Do not expect consistently strong answers, reasoning, or reliable tool usage from it.

Three local inference engine variants are included. Vulkan is selected by default.

If Vulkan does not work on your system, switch the engine in the main configuration to the CPU version.

For better response quality, you can either:

- use the included OpenRouter proxy engine;
- download a stronger GGUF model from Hugging Face and configure Familiar to use it locally.

When using OpenRouter, select the `openRouter` engine in the main configuration and provide your API key and model name in:

```text
engines/openRouter/configuration.json
```

OpenRouter registration and provider-specific setup are outside the scope of this documentation.

When using a larger local model, keep in mind that hardware requirements increase quickly with model size.

### Downloads

[Download Linux x64](https://github.com/Tripod311/Familiar/releases/latest/download/assistant_linux_64.zip)

[Download Windows x64](https://github.com/Tripod311/Familiar/releases/latest/download/assistant_win_64.zip)

---

## Job application tracker

A small practical agent for tracking job applications through normal conversation.

This demo uses a different module composition from the basic assistant.

It does not keep conversation history and persistent context memory is disabled.

Instead, it uses the `records` module as structured persistent storage.

The agent can:

- create application records from natural-language messages;
- search previous applications;
- update records when something changes;
- generate summaries and status reports;
- compare stored information;
- remove outdated records when explicitly requested.

For example, you can tell the agent where you applied, what position it was for, and what you thought about the opportunity.

Later, you can provide updates such as recruiter responses, test assignments, interviews, rejections, or changes in your estimate of the application.

The agent stores this information as small structured records instead of keeping the entire conversation in context.

No local model is bundled with this demo.

It is configured to use the OpenRouter proxy engine by default.

### Downloads

[Download Linux x64](https://github.com/Tripod311/Familiar/releases/latest/download/job_tracker_linux_64.zip)

[Download Windows x64](https://github.com/Tripod311/Familiar/releases/latest/download/job_tracker_win_64.zip)

---

## Puppy guide

A small knowledge-base assistant for questions about dog care.

The current demo knowledge base contains material about transitioning a dog to natural feeding. The source content was prepared from a document provided by an experienced dog trainer.

The application uses:

- short message history;
- persistent memory for basic information about the user and their dogs;
- a searchable knowledge base;
- an LLM that turns retrieved document fragments into natural-language answers.

The knowledge base included in the demo is intentionally small, but the same module can be used with a much larger set of documents.

The important distinction in this demo is that domain knowledge is not stored permanently in the model context.

Instead, the agent searches the knowledge base when needed and receives only the relevant fragments for the current request.

The persistent memory is used separately for user-specific information, such as dog names, breeds, age, feeding details, and other stable context.

No local model is bundled with this demo.

It is configured to use the OpenRouter proxy engine by default.

### Downloads

[Download Linux x64](https://github.com/Tripod311/Familiar/releases/latest/download/puppy_guide_linux_64.zip)

[Download Windows x64](https://github.com/Tripod311/Familiar/releases/latest/download/puppy_guide_win_64.zip)
