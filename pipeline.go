package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// ─── Types ──────────────────────────────────────────────────────────

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

// UploadResult is the response for the upload endpoint.
type UploadResult struct {
	Chunks   int    `json:"chunks"`
	QAPairs  int    `json:"qa_pairs"`
	Rules    int    `json:"rules"`
	Filename string `json:"filename"`
}

// PHPFunction represents a parsed PHP function.
type PHPFunction struct {
	Name string
	Body string
}

// SSEWriter abstracts SSE event writing for streaming pipeline results.
type SSEWriter interface {
	SendLog(msg string)
	SendToken(token string)
	SendResult(result *PipelineResult)
	Flush()
}

// ─── PHP Function Parser ────────────────────────────────────────────

// phpFuncRegex matches PHP function declarations.
var phpFuncRegex = regexp.MustCompile(`(?:public|protected|private)(?:\s+static)?\s+function\s+(\w+)`)

// parsePHPFunctions extracts functions from PHP source by matching
// the declaration and tracking brace depth to find the closing }.
func parsePHPFunctions(source string) []PHPFunction {
	matches := phpFuncRegex.FindAllStringSubmatchIndex(source, -1)
	if len(matches) == 0 {
		return nil
	}

	functions := make([]PHPFunction, 0, len(matches))

	for _, match := range matches {
		name := source[match[2]:match[3]]

		// Find the opening brace after the function declaration.
		rest := source[match[0]:]
		braceIdx := strings.Index(rest, "{")
		if braceIdx < 0 {
			continue
		}

		// Track brace depth to find the matching closing brace.
		depth := 0
		endIdx := -1
		for i := braceIdx; i < len(rest); i++ {
			switch rest[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					endIdx = i + 1
				}
			}
			if endIdx >= 0 {
				break
			}
		}

		if endIdx < 0 {
			// No matching brace found — take everything until next function or EOF.
			endIdx = len(rest)
		}

		body := rest[:endIdx]
		functions = append(functions, PHPFunction{Name: name, Body: body})
	}

	return functions
}

// ─── Upload Processing (Code Verbalization Pipeline) ────────────────

// ProcessUpload handles the full upload flow: parse → verbalize → embed → QA.
func ProcessUpload(db *sql.DB, fileContent, filename string) (*UploadResult, error) {
	// Clear previous data for this file.
	if err := DeleteFileData(db, filename); err != nil {
		Broadcast("warn", fmt.Sprintf("Failed to clear old data for %s: %v", filename, err), nil)
	}

	// Step 1 — Parse PHP into functions (or fall back to fixed chunks).
	functions := parsePHPFunctions(fileContent)

	var chunks []string
	if len(functions) == 0 {
		Broadcast("info", "No PHP functions found, falling back to fixed 512-char chunks", nil)
		chunks = chunkContentFixed(fileContent, 512)
	} else {
		Broadcast("info", fmt.Sprintf("Parsing PHP... found %d functions", len(functions)), nil)
	}

	totalQAPairs := 0

	if len(functions) > 0 {
		// Step 2 — Process each function through the verbalization pipeline.
		for i, fn := range functions {
			Broadcast("progress", fmt.Sprintf("%s()", fn.Name), map[string]any{
				"current": i + 1,
				"total":   len(functions),
			})

			// 2a. Save the raw function body as a chunk.
			chunkID, err := InsertChunkReturningID(db, filename, i, fn.Body, nil)
			if err != nil {
				Broadcast("error", fmt.Sprintf("Insert chunk failed for %s: %v", fn.Name, err), nil)
				continue
			}

			// 2b. Verbalize the function.
			Broadcast("info", fmt.Sprintf("Verbalizing %s()...", fn.Name), nil)
			verbalization, err := Verbalize(fn.Body)
			if err != nil {
				Broadcast("warn", fmt.Sprintf("Verbalize failed for %s, using raw body", fn.Name), nil)
				verbalization = fn.Body // Fallback to raw body.
			}

			// 2c. Embed the verbalization and save it.
			verbEmb, err := Embed(verbalization)
			if err != nil {
				Broadcast("warn", fmt.Sprintf("Embed verbalization failed for %s: %v", fn.Name, err), nil)
			}
			if err := SaveVerbalization(db, chunkID, verbalization, verbEmb); err != nil {
				Broadcast("error", fmt.Sprintf("Save verbalization failed for %s: %v", fn.Name, err), nil)
			} else {
				Broadcast("ok", fmt.Sprintf("Verbalized %s()", fn.Name), nil)
			}

			// 2d. Generate Q&A pairs from the verbalization.
			Broadcast("info", fmt.Sprintf("Generating QA for %s()...", fn.Name), nil)
			qaPairs, err := GenerateQA(verbalization)
			if err != nil {
				Broadcast("warn", fmt.Sprintf("GenerateQA failed for %s: %v", fn.Name, err), nil)
				qaPairs = nil
			}
			fnQACount := 0
			for _, pair := range qaPairs {
				qEmb, embErr := Embed(pair.Question)
				if embErr != nil {
					log.Printf("[upload] warning: embed QA question failed: %v", embErr)
					qEmb = nil
				}
				if saveErr := SaveQAPair(db, filename, fn.Name, pair.Question, pair.Answer, qEmb); saveErr != nil {
					log.Printf("[upload] warning: save QA pair failed: %v", saveErr)
				} else {
					totalQAPairs++
					fnQACount++
				}
			}

			Broadcast("data", fmt.Sprintf("%d QA pairs saved for %s()", fnQACount, fn.Name), nil)
		}
	} else {
		// Fallback path: embed fixed chunks without verbalization.
		for i, chunk := range chunks {
			embedding, err := Embed(chunk)
			if err != nil {
				Broadcast("warn", fmt.Sprintf("Embed failed for chunk %d: %v", i, err), nil)
				embedding = nil
			}
			if err := InsertChunk(db, filename, i, chunk, embedding); err != nil {
				Broadcast("error", fmt.Sprintf("Insert chunk %d failed: %v", i, err), nil)
			}
		}
	}

	// Step 3 — Extract rules via LLM (unchanged from original).
	Broadcast("info", "Extracting validation rules...", nil)
	rulesCount := 0
	extractedRules, err := extractRules(fileContent)
	if err != nil {
		Broadcast("warn", fmt.Sprintf("Rule extraction failed: %v", err), nil)
	} else {
		for _, rule := range extractedRules {
			if err := InsertRule(db, filename, rule); err != nil {
				log.Printf("[upload] warning: insert rule failed: %v", err)
			} else {
				rulesCount++
			}
		}
		Broadcast("data", fmt.Sprintf("%d rules extracted", rulesCount), nil)
	}

	chunkCount := len(functions)
	if chunkCount == 0 {
		chunkCount = len(chunks)
	}

	Broadcast("ok", fmt.Sprintf("Upload complete: %d functions, %d QA pairs, %d rules",
		chunkCount, totalQAPairs, rulesCount), map[string]any{
		"chunks":   chunkCount,
		"qa_pairs": totalQAPairs,
		"rules":    rulesCount,
	})

	return &UploadResult{
		Chunks:   chunkCount,
		QAPairs:  totalQAPairs,
		Rules:    rulesCount,
		Filename: filename,
	}, nil
}

// chunkContentFixed splits content into fixed-size chunks (fallback for non-PHP files).
func chunkContentFixed(content string, maxSize int) []string {
	lines := strings.Split(content, "\n")
	chunks := make([]string, 0, len(lines)/10+1)
	var current strings.Builder

	boundaries := []string{"function ", "func ", "class ", "public ", "private ", "protected ", "def ", "module "}

	for _, line := range lines {
		if current.Len() > maxSize/2 {
			trimmedLine := strings.TrimSpace(line)
			isBoundary := false
			for _, b := range boundaries {
				if strings.HasPrefix(trimmedLine, b) {
					isBoundary = true
					break
				}
			}
			if isBoundary && current.Len() > 0 {
				chunks = append(chunks, current.String())
				current.Reset()
			}
		}

		current.WriteString(line)
		current.WriteString("\n")

		if current.Len() >= maxSize {
			chunks = append(chunks, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}

// ─── Pipeline Run (SSE-streaming) ───────────────────────────────────

// RunPipeline orchestrates a full RAG + evaluation + self-correction loop.
// If an SSEWriter is provided, events are streamed in real time.
func RunPipeline(db *sql.DB, filename, query string, sse SSEWriter) (*PipelineResult, error) {
	plog := make([]string, 0, 16)

	emit := func(msg string) {
		plog = append(plog, msg)
		if sse != nil {
			sse.SendLog(msg)
			sse.Flush()
		}
	}

	emit(fmt.Sprintf("Starting pipeline for file=%s", filename))

	// 1. Embed the query.
	emit("Embedding query with nomic-embed-text...")
	Broadcast("info", "Embedding query...", nil)
	queryEmbedding, err := Embed(query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	emit(fmt.Sprintf("Query embedded (%d dimensions)", len(queryEmbedding)))

	// 2. Retrieve top 3 similar chunks (using verbalization embeddings).
	emit("Searching for similar functions (verbalization cosine similarity)...")
	chunks, err := QuerySimilar(db, filename, queryEmbedding, 3)
	if err != nil {
		return nil, fmt.Errorf("query similar: %w", err)
	}
	emit(fmt.Sprintf("Found %d relevant functions", len(chunks)))
	Broadcast("data", fmt.Sprintf("Retrieved %d relevant functions", len(chunks)), nil)

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks found for file '%s'", filename)
	}

	// 3. Retrieve top 3 similar Q&A examples.
	emit("Searching for similar Q&A examples...")
	qaPairs, err := GetQASimilar(db, filename, queryEmbedding, 3)
	if err != nil {
		log.Printf("[pipeline] warning: QA search failed: %v", err)
		qaPairs = nil
	}
	emit(fmt.Sprintf("Found %d relevant Q&A examples", len(qaPairs)))
	Broadcast("data", fmt.Sprintf("Retrieved %d QA examples", len(qaPairs)), nil)

	// 4. Load rules for this file.
	rules, err := GetRules(db, filename)
	if err != nil {
		return nil, fmt.Errorf("get rules: %w", err)
	}
	emit(fmt.Sprintf("Loaded %d validation rules", len(rules)))

	// 5. Build the enriched context.
	contextStr := buildEnrichedContext(chunks, qaPairs)

	// Build rules summary for prompt injection.
	rulesSummary := buildRulesSummary(rules)

	// 6. Create run record.
	graphJSON, _ := json.Marshal(map[string]interface{}{
		"filename": filename,
		"query":    query,
	})
	runID, err := InsertRun(db, string(graphJSON))
	if err != nil {
		log.Printf("[pipeline] warning: failed to insert run: %v", err)
	}

	// 7. Self-correction loop.
	var (
		correctionText string
		lastOutput     string
		lastResult     EvalResult
		retries        int
	)

	const maxRetries = 3

	for retries = 0; retries < maxRetries; retries++ {
		if retries > 0 {
			emit(fmt.Sprintf("Retry %d/%d — injecting correction...", retries, maxRetries))
			Broadcast("warn", fmt.Sprintf("Score %.2f below threshold — retry %d/%d", lastResult.Score, retries, maxRetries), nil)
		}

		// Build the prompt.
		prompt := buildPrompt(contextStr, rulesSummary, query, correctionText)

		emit("Calling llama3...")
		Broadcast("info", "Calling llama3...", nil)

		// Stream tokens via SSE if available, else use regular Chat.
		var response string
		if sse != nil {
			response, err = ChatStream(prompt, func(token string) {
				sse.SendToken(token)
				sse.Flush()
			})
		} else {
			response, err = Chat(prompt)
		}
		if err != nil {
			return nil, fmt.Errorf("chat (attempt %d): %w", retries+1, err)
		}

		lastOutput = response
		emit(fmt.Sprintf("Received response (%d chars)", len(response)))

		// 8. Evaluate.
		emit("Evaluating output against rules...")
		lastResult = Evaluate(response, rules)
		emit(fmt.Sprintf("Score: %.2f (T1=%.2f, T2=%.2f) | Passed: %v",
			lastResult.Score, lastResult.Tier1Score, lastResult.Tier2Score, lastResult.Passed))

		if lastResult.Passed {
			emit("✓ Output passed evaluation!")
			Broadcast("ok", fmt.Sprintf("Score: %.2f — passed", lastResult.Score), nil)
			break
		}

		// 9. Build correction and store it.
		errorsStr := strings.Join(lastResult.Errors, "; ")
		correctionText = fmt.Sprintf(
			"Previous attempt failed with score %.2f. Errors: %s. Fix these in your next response. Return ONLY valid JSON with no markdown wrapping.",
			lastResult.Score, errorsStr,
		)

		emit(fmt.Sprintf("✗ Failed evaluation (%d errors), storing correction...", len(lastResult.Errors)))

		corrEmb, embErr := Embed(correctionText)
		if embErr != nil {
			log.Printf("[pipeline] warning: failed to embed correction: %v", embErr)
			corrEmb = nil
		}
		if storeErr := InsertCorrection(db, filename, query, lastOutput, errorsStr, correctionText, corrEmb); storeErr != nil {
			log.Printf("[pipeline] warning: failed to store correction: %v", storeErr)
		}
	}

	// 10. Parse output for the response.
	var parsedOutput interface{}
	trimmed := strings.TrimSpace(lastOutput)
	extracted := extractJSON(trimmed)
	if extracted != "" {
		trimmed = extracted
	}
	if err := json.Unmarshal([]byte(trimmed), &parsedOutput); err != nil {
		parsedOutput = lastOutput // Fall back to raw string.
	}

	// 11. Update run record.
	status := "passed"
	if !lastResult.Passed {
		status = "failed"
		Broadcast("warn", "Max retries reached — showing best result", nil)
	} else {
		Broadcast("ok", "Output ready ✓", nil)
	}
	outputJSON, _ := json.Marshal(parsedOutput)
	if updateErr := UpdateRun(db, runID, status, lastResult.Score, retries, string(outputJSON)); updateErr != nil {
		log.Printf("[pipeline] warning: failed to update run: %v", updateErr)
	}

	emit(fmt.Sprintf("Pipeline complete — status=%s, retries=%d", status, retries))

	result := &PipelineResult{
		Output:  parsedOutput,
		Score:   lastResult.Score,
		Retries: retries,
		Passed:  lastResult.Passed,
		Errors:  lastResult.Errors,
		RunID:   runID,
		Log:     plog,
	}

	// Send final result via SSE if available.
	if sse != nil {
		sse.SendResult(result)
		sse.Flush()
	}

	return result, nil
}

// ─── Prompt Construction ────────────────────────────────────────────

// buildEnrichedContext creates a prompt context from code chunks, QA examples,
// and past corrections.
func buildEnrichedContext(chunks []string, qaPairs []QAPair) string {
	var sb strings.Builder

	sb.WriteString("RELEVANT CODE:\n")
	for i, c := range chunks {
		sb.WriteString(fmt.Sprintf("--- Function %d ---\n%s\n", i+1, c))
	}

	if len(qaPairs) > 0 {
		sb.WriteString("\nEXAMPLES OF CORRECT OUTPUT FORMAT:\n")
		for _, qa := range qaPairs {
			sb.WriteString(fmt.Sprintf("Q: %s\nA: %s\n\n", qa.Question, string(qa.AnswerJSON)))
		}
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

	sb.WriteString(context)
	sb.WriteString("\nQUERY: ")
	sb.WriteString(query)
	sb.WriteString("\n\nRespond with ONLY a JSON object:")

	return sb.String()
}
