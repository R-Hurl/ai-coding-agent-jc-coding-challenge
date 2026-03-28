package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
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

		reply, updatedHistory, err := runAgent(http.DefaultClient, apiKey, history)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			// Remove the user message we just added so history stays consistent
			history = history[:len(history)-1]
			continue
		}
		history = updatedHistory

		fmt.Println(reply)
	}
}
