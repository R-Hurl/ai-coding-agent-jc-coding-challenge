package main

import "encoding/json"

// Message represents a single turn in the conversation.
// Content uses omitempty because assistant tool_call messages have null content.
// ToolCalls is set on assistant messages that request tool executions.
// ToolCallID is set on role:"tool" result messages to match them to their call.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall is one entry in an assistant message's tool_calls array.
type ToolCall struct {
	Id           string       `json:"id"`
	Type         string       `json:"type"`
	FunctionCall FunctionCall `json:"function"`
}

// FunctionCall holds the name and JSON-encoded arguments for a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatRequest is the body sent to POST /v1/chat/completions.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
	Tools    []Tool    `json:"tools,omitempty"`
}

// ChatResponse is the non-streaming response from /v1/chat/completions.
type ChatResponse struct {
	Choices []ChatResponseChoice `json:"choices"`
}

// ChatResponseChoice is one entry in the choices array.
type ChatResponseChoice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// PermissionMode controls whether a tool executes automatically, requires user confirmation, or is blocked.
type PermissionMode string

const (
	PermissionAllow  PermissionMode = "allow"
	PermissionPrompt PermissionMode = "prompt"
	PermissionDeny   PermissionMode = "deny"
)

// Tool describes a function the model can call.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction holds the name, description, and JSON Schema for a tool's parameters.
// Parameters is json.RawMessage so we can embed an arbitrary schema without defining a struct for it.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}
