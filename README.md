# AI Coding Agent

A simplified AI coding agent CLI built in Go, inspired by tools like Claude Code, Codex, and AmpCode. Built as part of John Crickett's Coding Challenge #112 - AI Coding Agent.

## What it does

The agent is an interactive CLI that connects to the OpenAI API. It maintains conversation history across turns and can use tools to interact with your codebase — currently supporting file reading, with more capabilities planned.

## Getting started

1. Add your OpenAI API key to a `.env` file at the repo root:
   ```
   OPENAI_API_KEY=sk-...
   ```

2. Run the agent:
   ```bash
   go run .
   ```

## Example usage

```
> What's in playground/main.go?
[tool: read_file("playground/main.go")]
The file contains a Go program that performs basic arithmetic...

> What is a binary tree?
A binary tree is a data structure where each node has at most two children...
```

## Project structure

| File | Purpose |
|------|---------|
| `main.go` | Interactive REPL loop |
| `agent.go` | Agentic loop, API calls, tool execution |
| `types.go` | OpenAI API request/response structs |
| `playground/` | Sample Go project used for testing the agent |
