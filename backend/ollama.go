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
func Embed(ctx context.Context, text string) ([]float32, error) {
	client, err := newOllamaClient()
	if err != nil {
		return nil, err
	}

	req := &api.EmbedRequest{
		Model: GetActiveEmbedModel(),
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
func Chat(ctx context.Context, prompt string) (string, error) {
	return ChatWithSystem(ctx, "", prompt)
}

// ChatWithSystem sends a prompt to llama3 with an optional system message.
func ChatWithSystem(ctx context.Context, system, user string) (string, error) {
	client, err := newOllamaClient()
	if err != nil {
		return "", err
	}

	messages := make([]api.Message, 0, 2)
	if system != "" {
		messages = append(messages, api.Message{Role: "system", Content: system})
	}
	messages = append(messages, api.Message{Role: "user", Content: user})

	req := &api.ChatRequest{
		Model:    GetActiveModel(),
		Messages: messages,
		Stream:   new(bool), // false = non-streaming
		Options:  map[string]any{"temperature": GetActiveTemperature()},
	}

	// Thinking/reasoning: prepend a system directive if enabled.
	if GetActiveThinking() {
		req.Messages = append([]api.Message{
			{Role: "system", Content: "Enable extended thinking. Reason step-by-step before answering."},
		}, req.Messages...)
	}

	var response strings.Builder

	err = client.Chat(ctx, req, func(cr api.ChatResponse) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			response.WriteString(cr.Message.Content)
			return nil
		}
	})
	if err != nil {
		return "", fmt.Errorf("ollama chat: %w", err)
	}

	return response.String(), nil
}

// ChatStream sends a prompt to llama3 and calls onToken for each streamed chunk.
// Returns the full assembled response when complete.
func ChatStream(ctx context.Context, prompt string, onToken func(token string)) (string, error) {
	client, err := newOllamaClient()
	if err != nil {
		return "", err
	}

	stream := true
	req := &api.ChatRequest{
		Model: GetActiveModel(),
		Messages: []api.Message{
			{Role: "user", Content: prompt},
		},
		Stream:  &stream, // true = streaming
		Options: map[string]any{"temperature": GetActiveTemperature()},
	}

	// Thinking/reasoning: prepend a system directive if enabled.
	if GetActiveThinking() {
		req.Messages = append([]api.Message{
			{Role: "system", Content: "Enable extended thinking. Reason step-by-step before answering."},
		}, req.Messages...)
	}

	var full strings.Builder

	err = client.Chat(ctx, req, func(cr api.ChatResponse) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			token := cr.Message.Content
			full.WriteString(token)
			if onToken != nil {
				onToken(token)
			}
			return nil
		}
	})
	if err != nil {
		return "", fmt.Errorf("ollama chat stream: %w", err)
	}

	return full.String(), nil
}

// ─── Code Verbalization ─────────────────────────────────────────────

// Verbalize converts a PHP function body into precise plain-English description.
func Verbalize(ctx context.Context, functionBody string) (string, error) {
	system := `You are a code documentation engine. Convert the PHP function below into
precise plain English. Describe: every condition and what triggers it,
every fee type handled and when it applies, every input parameter and
what it controls, every output key produced and its constraints.
Be exhaustive. No code syntax. No markdown. Plain paragraphs only.`

	result, err := ChatWithSystem(ctx, system, functionBody)
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
func GenerateQA(ctx context.Context, verbalization string) ([]QAPairData, error) {
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

	response, err := ChatWithSystem(ctx, system, user)
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

// ListOllamaModels returns the names of all models available on the Ollama server.
func ListOllamaModels() ([]string, error) {
	resp, err := http.Get(ollamaBaseURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	names := make([]string, len(result.Models))
	for i, m := range result.Models {
		names[i] = m.Name
	}
	return names, nil
}
