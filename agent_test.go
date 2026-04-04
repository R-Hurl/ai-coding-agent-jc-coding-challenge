package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeHttpClient struct {
	response *http.Response
	err      error
}

func (f *fakeHttpClient) Do(req *http.Request) (*http.Response, error) {
	return f.response, f.err
}

// -- Tests using fakeHttpClient --

func TestChatOnce_Success(t *testing.T) {
	want := ChatResponse{
		Choices: []ChatResponseChoice{
			{Message: Message{Role: "assistant", Content: "Hello!"}, FinishReason: "stop"},
		},
	}
	body, _ := json.Marshal(want)

	client := &fakeHttpClient{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(body))),
		},
	}

	got, err := chatOnce(client, "http://fake-url", "test-key", []Message{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Choices[0].Message.Content != "Hello!" {
		t.Errorf("got content %q, want %q", got.Choices[0].Message.Content, "Hello!")
	}
}

func TestChatOnce_NonOKStatus(t *testing.T) {
	client := &fakeHttpClient{
		response: &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("internal server error")),
		},
	}

	_, err := chatOnce(client, "http://fake-url", "test-key", []Message{}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain status code 500, got: %v", err)
	}
}

// TODO(human): Implement TestChatOnce_NetworkError.
// This test should verify that when the fakeHttpClient's Do method returns an error
// (and a nil response), chatOnce propagates that error back to the caller.
// Hint: set the `err` field on fakeHttpClient instead of the `response` field.
// Use fmt.Errorf to create a test error, then assert err != nil after calling chatOnce.
func TestChatOnce_NetworkError(t *testing.T) {
	testError := fmt.Errorf("Error occurred calling API")
	client := &fakeHttpClient{
		response: nil,
		err: testError,
	}

	_, err := chatOnce(client, "http://fake-url", "test-key", []Message{}, nil)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

// -- Test using httptest.NewServer --

func TestChatOnce_AuthHeader_HTTPTest(t *testing.T) {
	var capturedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		resp := ChatResponse{
			Choices: []ChatResponseChoice{
				{Message: Message{Role: "assistant", Content: "ok"}, FinishReason: "stop"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	_, err := chatOnce(http.DefaultClient, server.URL, "test-key", []Message{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "Bearer test-key"
	if capturedAuth != want {
		t.Errorf("Authorization header = %q, want %q", capturedAuth, want)
	}
}

