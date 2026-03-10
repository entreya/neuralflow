package main

import (
	"context"
	"encoding/json"
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
	return ChatWithSystem("", prompt)
}

// ChatWithSystem sends a prompt to llama3 with an optional system message.
func ChatWithSystem(system, user string) (string, error) {
	client, err := newOllamaClient()
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	messages := make([]api.Message, 0, 2)
	if system != "" {
		messages = append(messages, api.Message{Role: "system", Content: system})
	}
	messages = append(messages, api.Message{Role: "user", Content: user})

	req := &api.ChatRequest{
		Model:    "llama3",
		Messages: messages,
		Stream:   new(bool), // false = non-streaming
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

// ChatStream sends a prompt to llama3 and calls onToken for each streamed chunk.
// Returns the full assembled response when complete.
func ChatStream(prompt string, onToken func(token string)) (string, error) {
	client, err := newOllamaClient()
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	stream := true
	req := &api.ChatRequest{
		Model: "llama3",
		Messages: []api.Message{
			{Role: "user", Content: prompt},
		},
		Stream: &stream, // true = streaming
	}

	var full strings.Builder

	err = client.Chat(ctx, req, func(cr api.ChatResponse) error {
		token := cr.Message.Content
		full.WriteString(token)
		if onToken != nil {
			onToken(token)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("ollama chat stream: %w", err)
	}

	return full.String(), nil
}

// ─── Code Verbalization ─────────────────────────────────────────────

// Verbalize converts a PHP function body into precise plain-English description.
func Verbalize(functionBody string) (string, error) {
	system := `You are a code documentation engine. Convert the PHP function below into
precise plain English. Describe: every condition and what triggers it,
every fee type handled and when it applies, every input parameter and
what it controls, every output key produced and its constraints.
Be exhaustive. No code syntax. No markdown. Plain paragraphs only.`

	result, err := ChatWithSystem(system, functionBody)
	if err != nil {
		return "", fmt.Errorf("verbalize: %w", err)
	}
	return strings.TrimSpace(result), nil
}

// ─── QA Generation ──────────────────────────────────────────────────

// QAPairData is the JSON shape returned by the GenerateQA prompt.
type QAPairData struct {
	Question string          `json:"question"`
	Answer   json.RawMessage `json:"answer"`
}

// GenerateQA produces test Q&A pairs from a function verbalization.
// If the LLM returns invalid JSON, returns an empty slice (no error).
func GenerateQA(verbalization string) ([]QAPairData, error) {
	system := `You are a test case generator. Return ONLY a raw JSON array. No markdown.
No explanation. Each item: {"question": "...", "answer": {...}}
The answer must follow this schema exactly:
{"status":1,"total":0,"currency":"INR","exempted":0,
 "components":[{"amount":0,"currency":"INR","message":"..."}]}`

	user := `Generate 6 test cases for this function covering:
1. Normal appearing papers only
2. Mixed appearing + failed papers
3. PWD exemption (total must be 0, exempted must be 1)
4. Late fee triggered (submission after deadline)
5. Custom fee enabled (single component only)
6. Re-course papers (failed/improvement excluded)

Function description:
` + verbalization

	response, err := ChatWithSystem(system, user)
	if err != nil {
		return nil, fmt.Errorf("generate qa: %w", err)
	}

	// Parse the response — tolerant of markdown wrapping.
	trimmed := strings.TrimSpace(response)

	// Try to extract from code fences.
	extracted := extractJSONArray(trimmed)
	if extracted != "" {
		trimmed = extracted
	}

	// Find array boundaries.
	if !strings.HasPrefix(trimmed, "[") {
		if idx := strings.Index(trimmed, "["); idx >= 0 {
			trimmed = trimmed[idx:]
		}
	}
	if lastIdx := strings.LastIndex(trimmed, "]"); lastIdx >= 0 {
		trimmed = trimmed[:lastIdx+1]
	}

	var pairs []QAPairData
	if err := json.Unmarshal([]byte(trimmed), &pairs); err != nil {
		log.Printf("[ollama] warning: GenerateQA returned invalid JSON, skipping: %v (first 200 chars: %.200s)", err, trimmed)
		return nil, nil // Do not crash — return empty.
	}

	return pairs, nil
}

// extractJSONArray tries to pull a JSON array out of markdown code fences.
func extractJSONArray(s string) string {
	if idx := strings.Index(s, "```json"); idx >= 0 {
		start := idx + 7
		end := strings.Index(s[start:], "```")
		if end >= 0 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	if idx := strings.Index(s, "```"); idx >= 0 {
		start := idx + 3
		if nlIdx := strings.Index(s[start:], "\n"); nlIdx >= 0 {
			start += nlIdx + 1
		}
		end := strings.Index(s[start:], "```")
		if end >= 0 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	return ""
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
