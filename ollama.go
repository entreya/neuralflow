package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/ollama/ollama/api"
)

// ollamaBaseURL is the default Ollama server address.
const ollamaBaseURL = "http://localhost:11434"

// newOllamaClient creates an Ollama API client.
func newOllamaClient() (*api.Client, error) {
	u, err := url.Parse(ollamaBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse ollama url: %w", err)
	}
	return api.NewClient(u, http.DefaultClient), nil
}

// Embed generates an embedding for the given text using nomic-embed-text.
func Embed(text string) ([]float32, error) {
	client, err := newOllamaClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	req := &api.EmbedRequest{
		Model: "nomic-embed-text",
		Input: text,
	}

	resp, err := client.Embed(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("ollama embed: empty response")
	}

	// Convert []float64 to []float32 for storage efficiency.
	raw := resp.Embeddings[0]
	embedding := make([]float32, len(raw))
	for i, v := range raw {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// Chat sends a prompt to llama3 and returns the full response text.
func Chat(prompt string) (string, error) {
	client, err := newOllamaClient()
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	req := &api.ChatRequest{
		Model: "llama3",
		Messages: []api.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: new(bool), // false = non-streaming
	}

	var response strings.Builder

	err = client.Chat(ctx, req, func(cr api.ChatResponse) error {
		response.WriteString(cr.Message.Content)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("ollama chat: %w", err)
	}

	return response.String(), nil
}

// CheckOllama verifies that the Ollama server is reachable.
func CheckOllama() bool {
	resp, err := http.Get(ollamaBaseURL + "/api/tags")
	if err != nil {
		log.Printf("[ollama] health check failed: %v", err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
