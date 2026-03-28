# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

This is a step-by-step coding challenge to build a simplified AI coding agent CLI in Go — similar to Claude Code, Codex, or AmpCode. The agent calls the OpenAI REST API directly (no SDK) and will progressively gain capabilities: chat, file reading, code editing, shell execution, and codebase search.

## Commands

```bash
# Run the agent
go run .

# Run the playground test project
go run playground/main.go

# Build the agent binary
go build -o ai-coding-agent .

# Add a dependency
go get <package>
```

## Architecture

**`main.go`** — entry point. Contains:
- `main()` — loads `.env` via `godotenv`, reads `OPENAI_API_KEY`, runs the interactive REPL loop

**`types.go`** — all OpenAI API structs:
- `Message`, `ChatRequest`, `ChatResponse`, `ChatResponseChoice`
- `ToolCall`, `FunctionCall`, `Tool`, `ToolFunction`

**`agent.go`** — agent logic and API calls:
- `chatOnce(apiKey, messages, tools)` — single non-streaming POST to OpenAI, returns full response
- `runAgent(apiKey, history)` — agentic loop: calls `chatOnce`, executes tool calls, repeats until `finish_reason == "stop"`
- `readFile(path)` — executes the `read_file` tool; returns file contents or an error string
- `createToolList()` — returns the slice of tools available to the model

**`playground/`** — a small multi-package Go app used as a test target for the agent (reading, editing, searching files). Not part of the agent itself.

## API Key

Stored in `.env` at the repo root (gitignored). `godotenv.Load()` is called at startup; falls back to shell environment if `.env` is absent.

```
OPENAI_API_KEY=sk-...
```

## OpenAI API

Model: `gpt-4o-mini`. Endpoint: `POST https://api.openai.com/v1/chat/completions`. The response struct only captures `choices[0].message.content` and `finish_reason` — other fields in the response are intentionally omitted.
