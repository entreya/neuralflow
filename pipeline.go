package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// PipelineResult is the final output returned to the API caller.
type PipelineResult struct {
	Output  interface{} `json:"output"`
	Score   float64     `json:"score"`
	Retries int         `json:"retries"`
	Passed  bool        `json:"passed"`
	Errors  []string    `json:"errors,omitempty"`
	RunID   int64       `json:"run_id"`
	Log     []string    `json:"log"`
}

// RunPipeline orchestrates a full RAG + evaluation + self-correction loop.
func RunPipeline(db *sql.DB, filename, query string) (*PipelineResult, error) {
	plog := make([]string, 0, 16) // Pipeline log for the UI console.
	plog = append(plog, fmt.Sprintf("Starting pipeline for file=%s", filename))

	// 1. Embed the query.
	plog = append(plog, "Embedding query with nomic-embed-text...")
	queryEmbedding, err := Embed(query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	plog = append(plog, fmt.Sprintf("Query embedded (%d dimensions)", len(queryEmbedding)))

	// 2. Retrieve top 5 similar chunks.
	plog = append(plog, "Searching for similar chunks (cosine similarity)...")
	chunks, err := QuerySimilar(db, filename, queryEmbedding, 5)
	if err != nil {
		return nil, fmt.Errorf("query similar: %w", err)
	}
	plog = append(plog, fmt.Sprintf("Found %d relevant chunks", len(chunks)))

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks found for file '%s'", filename)
	}

	// 3. Load rules for this file.
	rules, err := GetRules(db, filename)
	if err != nil {
		return nil, fmt.Errorf("get rules: %w", err)
	}
	plog = append(plog, fmt.Sprintf("Loaded %d validation rules", len(rules)))

	// 4. Build the context string.
	context := buildContext(chunks)

	// Build rules summary for prompt injection.
	rulesSummary := buildRulesSummary(rules)

	// 5. Create run record.
	graphJSON, _ := json.Marshal(map[string]interface{}{
		"filename": filename,
		"query":    query,
	})
	runID, err := InsertRun(db, string(graphJSON))
	if err != nil {
		log.Printf("[pipeline] warning: failed to insert run: %v", err)
	}

	// 6. Self-correction loop.
	var (
		correctionText string
		lastOutput     string
		lastResult     EvalResult
		retries        int
	)

	const maxRetries = 3

	for retries = 0; retries < maxRetries; retries++ {
		if retries > 0 {
			plog = append(plog, fmt.Sprintf("Retry %d/%d — injecting correction...", retries, maxRetries))
		}

		// Build the prompt.
		prompt := buildPrompt(context, rulesSummary, query, correctionText)

		plog = append(plog, "Calling llama3...")
		response, err := Chat(prompt)
		if err != nil {
			return nil, fmt.Errorf("chat (attempt %d): %w", retries+1, err)
		}
		lastOutput = response
		plog = append(plog, fmt.Sprintf("Received response (%d chars)", len(response)))

		// 7. Evaluate.
		plog = append(plog, "Evaluating output against rules...")
		lastResult = Evaluate(response, rules)
		plog = append(plog, fmt.Sprintf("Score: %.2f (T1=%.2f, T2=%.2f) | Passed: %v",
			lastResult.Score, lastResult.Tier1Score, lastResult.Tier2Score, lastResult.Passed))

		if lastResult.Passed {
			plog = append(plog, "✓ Output passed evaluation!")
			break
		}

		// 8. Build correction and store it.
		errorsStr := strings.Join(lastResult.Errors, "; ")
		correctionText = fmt.Sprintf(
			"Previous attempt failed with score %.2f. Errors: %s. Fix these in your next response. Return ONLY valid JSON with no markdown wrapping.",
			lastResult.Score, errorsStr,
		)

		plog = append(plog, fmt.Sprintf("✗ Failed evaluation (%d errors), storing correction...", len(lastResult.Errors)))

		// Embed and store the correction.
		corrEmb, embErr := Embed(correctionText)
		if embErr != nil {
			log.Printf("[pipeline] warning: failed to embed correction: %v", embErr)
			corrEmb = nil
		}
		if storeErr := InsertCorrection(db, filename, query, lastOutput, errorsStr, correctionText, corrEmb); storeErr != nil {
			log.Printf("[pipeline] warning: failed to store correction: %v", storeErr)
		}
	}

	// 9. Parse output for the response.
	var parsedOutput interface{}
	trimmed := strings.TrimSpace(lastOutput)
	extracted := extractJSON(trimmed)
	if extracted != "" {
		trimmed = extracted
	}
	if err := json.Unmarshal([]byte(trimmed), &parsedOutput); err != nil {
		parsedOutput = lastOutput // Fall back to raw string.
	}

	// 10. Update run record.
	status := "passed"
	if !lastResult.Passed {
		status = "failed"
	}
	outputJSON, _ := json.Marshal(parsedOutput)
	if updateErr := UpdateRun(db, runID, status, lastResult.Score, retries, string(outputJSON)); updateErr != nil {
		log.Printf("[pipeline] warning: failed to update run: %v", updateErr)
	}

	plog = append(plog, fmt.Sprintf("Pipeline complete — status=%s, retries=%d", status, retries))

	return &PipelineResult{
		Output:  parsedOutput,
		Score:   lastResult.Score,
		Retries: retries,
		Passed:  lastResult.Passed,
		Errors:  lastResult.Errors,
		RunID:   runID,
		Log:     plog,
	}, nil
}

// ─── Prompt Construction ────────────────────────────────────────────

func buildContext(chunks []string) string {
	var sb strings.Builder
	for i, c := range chunks {
		sb.WriteString(fmt.Sprintf("--- Chunk %d ---\n%s\n\n", i+1, c))
	}
	return sb.String()
}

func buildRulesSummary(rules []Rule) string {
	if len(rules) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("The output must satisfy these validation rules:\n")
	for _, r := range rules {
		sb.WriteString(fmt.Sprintf("- [%s] %s (field: %s, operator: %s)\n",
			r.RuleType, r.Description, r.FieldPath, r.Operator))
	}
	return sb.String()
}

func buildPrompt(context, rulesSummary, query, correction string) string {
	var sb strings.Builder
	sb.WriteString("You are a code analysis assistant. Based on the code context below, answer the query.\n")
	sb.WriteString("Return ONLY a valid JSON object. No markdown. No explanation. No preamble.\n\n")

	if rulesSummary != "" {
		sb.WriteString(rulesSummary)
		sb.WriteString("\n")
	}

	if correction != "" {
		sb.WriteString("IMPORTANT CORRECTION:\n")
		sb.WriteString(correction)
		sb.WriteString("\n\n")
	}

	sb.WriteString("CODE CONTEXT:\n")
	sb.WriteString(context)
	sb.WriteString("\nQUERY: ")
	sb.WriteString(query)
	sb.WriteString("\n\nRespond with ONLY a JSON object:")

	return sb.String()
}
