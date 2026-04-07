package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	openAIURL                     = "https://api.openai.com/v1/chat/completions"
	openAIModel                   = "gpt-4o-mini"
	finishReasonStop              = "stop"
	finishReasonToolCalls         = "tool_calls"
	toolCallFunctionType          = "function"
	toolCallReadFileFunctionName  = "read_file"
	toolCallEditFileFunctionName  = "edit_file"
	toolCallWriteFileFunctionName = "write_file"
	dirPermissions                = fs.FileMode(0755)
	filePermissions               = fs.FileMode(0644)
	toolCallGlobFunctionName      = "glob"
	toolCallGrepFunctionName      = "grep"
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

// editFile replaces the first occurrence of oldText in the file at path with newText.
// Returns an error string if the file can't be read/written or oldText is not found.
func editFile(path, oldText, newText string) string {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("error reading file: %v", err)
	}

	textExits := strings.Contains(string(fileContent), oldText)
	if !textExits {
		return fmt.Sprintf("oldText not found in file: %v", oldText)
	}

	newContent := strings.Replace(string(fileContent), oldText, newText, 1)
	if err := os.WriteFile(path, []byte(newContent), filePermissions); err != nil {
		return fmt.Sprintf("error writing file: %v", err)
	}

	return "ok"
}

// writeFile creates or overwrites the file at path with content.
// Intermediate directories are created if needed.
func writeFile(path, content string) string {
	if err := os.MkdirAll(filepath.Dir(path), dirPermissions); err != nil {
		return fmt.Sprintf("error creating directories: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), filePermissions); err != nil {
		return fmt.Sprintf("error writing file: %v", err)
	}
	return "ok"
}

// globFiles returns a newline-joined list of file paths matching pattern.
// Uses filepath.Walk for recursion so most patterns work even without native ** support.
// ** is stripped from the pattern since Walk already handles recursive traversal.
func globFiles(pattern string) string {
	// filepath.Match doesn't support **, so normalize it out.
	// Walk handles recursion, so "src/**/*.go" becomes "src/*.go" which matches correctly.
	normalized := strings.ReplaceAll(pattern, "**/", "")
	var matches []string
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if ok, _ := filepath.Match(normalized, path); ok {
			matches = append(matches, path)
			return nil
		}
		if ok, _ := filepath.Match(normalized, filepath.Base(path)); ok {
			matches = append(matches, path)
		}
		return nil
	})
	if len(matches) == 0 {
		return "no files found"
	}
	return strings.Join(matches, "\n")
}

// grepFiles searches all files under "." for lines matching pattern and returns
// matching lines formatted as "filepath:linenum: line".
func grepFiles(pattern string) string {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Sprintf("invalid regex pattern: %v", err)
	}

	var matches []string
	filepath.Walk(".", func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if regex.MatchString(line) {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", path, lineNum, line))
			}
		}
		return nil
	})

	if len(matches) == 0 {
		return "no matches found"
	}

	return strings.Join(matches, "\n")
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
				if toolCall.Type != toolCallFunctionType {
					continue
				}
				var result string
				switch toolCall.FunctionCall.Name {
				case toolCallReadFileFunctionName:
					var args struct {
						Path string `json:"path"`
					}
					if err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &args); err != nil {
						return "", history, err
					}
					fmt.Printf("[tool: read_file(%q)]\n", args.Path)
					result = readFile(args.Path)
				case toolCallEditFileFunctionName:
					var args struct {
						Path    string `json:"path"`
						OldText string `json:"old_text"`
						NewText string `json:"new_text"`
					}
					if err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &args); err != nil {
						return "", history, err
					}
					fmt.Printf("[tool: edit_file(%q)]\n", args.Path)
					result = editFile(args.Path, args.OldText, args.NewText)
				case toolCallWriteFileFunctionName:
					var args struct {
						Path    string `json:"path"`
						Content string `json:"content"`
					}
					if err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &args); err != nil {
						return "", history, err
					}
					fmt.Printf("[tool: write_file(%q)]\n", args.Path)
					result = writeFile(args.Path, args.Content)
				case toolCallGlobFunctionName:
					var args struct {
						Pattern string `json:"pattern"`
					}
					if err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &args); err != nil {
						return "", history, err
					}
					fmt.Printf("[tool: glob(%q)]\n", args.Pattern)
					result = globFiles(args.Pattern)
				case toolCallGrepFunctionName:
					var args struct {
						Pattern string `json:"pattern"`
					}
					if err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &args); err != nil {
						return "", history, err
					}
					fmt.Printf("[tool: grep(%q)]\n", args.Pattern)
					result = grepFiles(args.Pattern)
				}
				if result != "" {
					history = append(history, Message{
						Role:       "tool",
						Content:    result,
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
						"path": {"type": "string", "description": "Path to the file to read"}
					},
					"required": ["path"]
				}`),
			},
		},
		{
			Type: toolCallFunctionType,
			Function: ToolFunction{
				Name:        toolCallEditFileFunctionName,
				Description: "Replace the first occurrence of old_text with new_text in a file",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path":     {"type": "string", "description": "Path to the file to edit"},
						"old_text": {"type": "string", "description": "Exact text to find and replace"},
						"new_text": {"type": "string", "description": "Text to replace it with"}
					},
					"required": ["path", "old_text", "new_text"]
				}`),
			},
		},
		{
			Type: toolCallFunctionType,
			Function: ToolFunction{
				Name:        toolCallWriteFileFunctionName,
				Description: "Create or overwrite a file with the given content",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path":    {"type": "string", "description": "Path to the file to write"},
						"content": {"type": "string", "description": "Full content to write to the file"}
					},
					"required": ["path", "content"]
				}`),
			},
		},
		{
			Type: toolCallFunctionType,
			Function: ToolFunction{
				Name:        toolCallGlobFunctionName,
				Description: "Find files matching a glob pattern (e.g. **/*.go, src/**/*.ts)",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"pattern": {"type": "string", "description": "Glob pattern to match files against"}
					},
					"required": ["pattern"]
				}`),
			},
		},
		{
			Type: toolCallFunctionType,
			Function: ToolFunction{
				Name:        toolCallGrepFunctionName,
				Description: "Search file contents for a regex pattern; returns matching lines with file path and line number",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"pattern": {"type": "string", "description": "Regex pattern to search for"}
					},
					"required": ["pattern"]
				}`),
			},
		},
	}
}
