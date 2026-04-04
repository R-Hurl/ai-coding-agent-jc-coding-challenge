package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	openAIURL                    = "https://api.openai.com/v1/chat/completions"
	openAIModel                  = "gpt-4o-mini"
	finishReasonStop             = "stop"
	finishReasonToolCalls        = "tool_calls"
	toolCallFunctionType         = "function"
	toolCallReadFileFunctionName = "read_file"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// chatOnce sends a non-streaming request to OpenAI and returns the full response.
func chatOnce(client HTTPClient, url string, apiKey string, messages []Message, tools []Tool) (ChatResponse, error) {
	reqBody := ChatRequest{
		Model:    openAIModel,
		Messages: messages,
		Tools:    tools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBytes, _ := io.ReadAll(resp.Body)
		return ChatResponse{}, fmt.Errorf("API error (status %d): %s", resp.StatusCode, errBytes)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("read response: %w", err)
	}

	var response ChatResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return ChatResponse{}, fmt.Errorf("decode response: %w", err)
	}

	return response, nil
}

// readFile returns the contents of path, or an error string if it can't be read.
// Errors are returned as strings (not Go errors) so the model can reason about them.
func readFile(path string) string {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("error reading file: %v", err)
	}
	return string(fileContent)
}

// runAgent runs the agentic loop: sends the conversation to the model, executes any
// tool calls, and repeats until the model produces a final text response.
// Returns the reply text and the updated history (including all tool call/result turns).
func runAgent(client HTTPClient, apiKey string, history []Message) (string, []Message, error) {
	tools := createToolList()
	for {
		chatResponse, err := chatOnce(client, openAIURL, apiKey, history, tools)
		if err != nil {
			return "", history, err
		}

		choice := chatResponse.Choices[0]
		history = append(history, choice.Message)

		if choice.FinishReason == finishReasonStop {
			return choice.Message.Content, history, nil
		}

		if choice.FinishReason == finishReasonToolCalls {
			for _, toolCall := range choice.Message.ToolCalls {
				if toolCall.Type == toolCallFunctionType && toolCall.FunctionCall.Name == toolCallReadFileFunctionName {
					var args struct {
						Path string `json:"path"`
					}
					if err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &args); err != nil {
						return "", history, err
					}
					fmt.Printf("[tool: read_file(%q)]\n", args.Path)
					fileContents := readFile(args.Path)
					history = append(history, Message{
						Role:       "tool",
						Content:    fileContents,
						ToolCallID: toolCall.Id,
					})
				}
			}
		}
	}
}

// createToolList returns the tools available to the agent.
func createToolList() []Tool {
	return []Tool{
		{
			Type: toolCallFunctionType,
			Function: ToolFunction{
				Name:        toolCallReadFileFunctionName,
				Description: "Read the contents of a file at the given path",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path": {
							"type": "string",
							"description": "The path to the file to read"
						}
					},
					"required": ["path"]
				}`),
			}},
	}
}
