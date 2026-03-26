package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/joho/godotenv"
)

const openAIURL = "https://api.openai.com/v1/chat/completions"

// --- Request types ---

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"` // omitempty: false → field omitted, keeps non-streaming callers compatible
}

// --- Streaming response types ---

// StreamDelta holds the partial content for one SSE chunk.
type StreamDelta struct {
	Content string `json:"content"`
}

// StreamChoice is one entry in the choices array of a streaming chunk.
// FinishReason is a pointer because OpenAI sends null during streaming and "stop" on the final chunk —
// a plain string can't distinguish between an empty string and an absent field.
type StreamChoice struct {
	Delta        StreamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// StreamChunk is the top-level struct for one parsed SSE data line.
// Wire format: data: {"choices":[{"delta":{"content":"Hi"},"finish_reason":null}]}
//
//	→ StreamChunk.Choices[0].Delta.Content == "Hi"
type StreamChunk struct {
	Choices []StreamChoice `json:"choices"`
}


func streamChat(apiKey string, messages []Message) (string, error) {
	// Build the request body with streaming enabled
	reqBody := ChatRequest{
		Model:    "gpt-4o-mini",
		Messages: messages,
		Stream:   true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// Create and send the HTTP request — same pattern as chat()
	req, err := http.NewRequest("POST", openAIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// On error status, read the full body and return it as an error message
	if resp.StatusCode != http.StatusOK {
		errBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, errBytes)
	}

	// Wrap the response body in a buffered reader to read SSE lines one at a time
	reader := bufio.NewReader(resp.Body)
	var builder strings.Builder

	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		// Process whatever we read before handling the error
		if line == "" {
			if err != nil {
				break
			}
			continue
		}

		// End-of-stream sentinel
		if line == "data: [DONE]" {
			break
		}

		// Skip non-data lines (e.g. "event: ..." or comments)
		if !strings.HasPrefix(line, "data: ") {
			if err != nil {
				break
			}
			continue
		}

		// Strip the "data: " prefix and parse the JSON chunk
		jsonData := strings.TrimPrefix(line, "data: ")
		var chunk StreamChunk
		if err := json.Unmarshal([]byte(jsonData), &chunk); err != nil {
			return "", fmt.Errorf("decode stream chunk: %w", err)
		}

		// Print the token immediately and accumulate for history
		if len(chunk.Choices) > 0 {
			token := chunk.Choices[0].Delta.Content
			fmt.Print(token)
			builder.WriteString(token)
		}

		if err != nil {
			break
		}
	}

	fmt.Println() // end the streamed line
	return builder.String(), nil
}


func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, falling back to environment variables")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is not set — add it to .env or export it in your shell")
	}

	// Buffer of 1 so the signal isn't dropped if it arrives before we reach the select
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	history := []Message{}
	stdinReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")

		input, err := stdinReader.ReadString('\n')
		input = strings.TrimSpace(input)

		// Check for Ctrl+C (non-blocking)
		select {
		case <-sigCh:
			fmt.Println("\nGoodbye!")
			return
		default:
		}

		// Ctrl+D or closed stdin
		if err == io.EOF {
			fmt.Println("\nGoodbye!")
			return
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}

		if input == "" {
			continue
		}

		history = append(history, Message{Role: "user", Content: input})

		reply, err := streamChat(apiKey, history)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			// Remove the user message we just added so history stays consistent
			history = history[:len(history)-1]
			continue
		}

		history = append(history, Message{Role: "assistant", Content: reply})
	}
}
