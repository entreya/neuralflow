package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Global database handle, initialized in main().
var appDB *sql.DB

// ─── Log Broadcasting ───────────────────────────────────────────────

// LogEvent represents a structured log event for SSE streaming.
type LogEvent struct {
	Time    string         `json:"time"`
	Type    string         `json:"type"`
	Message string         `json:"message"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// logClients holds all active SSE client channels.
var (
	logClients   = make(map[chan LogEvent]struct{})
	logClientsMu sync.RWMutex
)

// Broadcast sends a LogEvent to all connected SSE clients and logs to terminal.
// Non-blocking: if a client's channel is full, the event is dropped for that client.
func Broadcast(eventType, message string, meta map[string]any) {
	log.Printf("[%s] %s", eventType, message)

	evt := LogEvent{
		Time:    time.Now().Format("15:04:05"),
		Type:    eventType,
		Message: message,
		Meta:    meta,
	}

	logClientsMu.RLock()
	defer logClientsMu.RUnlock()
	for ch := range logClients {
		select {
		case ch <- evt:
		default:
			// Client channel full — drop event to avoid blocking.
		}
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[main] starting NeuralFlow server...")

	// ── Initialize database ────────────────────────────────────
	db, err := InitDB()
	if err != nil {
		log.Fatalf("[main] database init failed: %v", err)
	}
	defer db.Close()
	appDB = db

	// ── Setup Gin router ───────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Serve static frontend.
	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	// API routes.
	api := r.Group("/api")
	{
		api.POST("/upload", handleUpload)
		api.POST("/run", handleRun)
		api.GET("/files", handleFiles)
		api.GET("/rules", handleRules)
		api.GET("/corrections", handleCorrections)
		api.GET("/logs", handleLogs)
	}
	r.GET("/health", handleHealth)

	log.Println("[main] NeuralFlow running on http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("[main] server failed: %v", err)
	}
}

// ─── GET /api/logs (SSE Broadcast Stream) ───────────────────────────

func handleLogs(c *gin.Context) {
	// Set SSE headers.
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, _ := c.Writer.(http.Flusher)

	// Register this client.
	ch := make(chan LogEvent, 256)
	logClientsMu.Lock()
	logClients[ch] = struct{}{}
	logClientsMu.Unlock()

	// Unregister on disconnect.
	defer func() {
		logClientsMu.Lock()
		delete(logClients, ch)
		logClientsMu.Unlock()
		close(ch)
	}()

	// Send initial heartbeat so client knows connection is live.
	fmt.Fprintf(c.Writer, ": heartbeat\n\n")
	if flusher != nil {
		flusher.Flush()
	}

	// Stream events until client disconnects.
	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(c.Writer, "event: log\ndata: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

// ─── POST /api/upload ───────────────────────────────────────────────

func handleUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": 400, "data": nil,
			"error": "missing file in form data: " + err.Error(),
		})
		return
	}
	defer file.Close()

	filename := header.Filename
	Broadcast("info", fmt.Sprintf("Upload started: %s (%d bytes)", filename, header.Size), nil)

	// Validate file extension.
	allowed := []string{".php", ".json", ".txt", ".csv", ".go", ".js", ".py"}
	ext := ""
	for _, a := range allowed {
		if strings.HasSuffix(strings.ToLower(filename), a) {
			ext = a
			break
		}
	}
	if ext == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": 400, "data": nil,
			"error": fmt.Sprintf("unsupported file type, allowed: %v", allowed),
		})
		return
	}

	// Read file content.
	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": 500, "data": nil,
			"error": "failed to read file: " + err.Error(),
		})
		return
	}

	// Run the upload pipeline (verbalization + QA generation).
	result, err := ProcessUpload(appDB, string(content), filename)
	if err != nil {
		Broadcast("error", fmt.Sprintf("Upload failed: %v", err), nil)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": 500, "data": nil,
			"error": "upload processing failed: " + err.Error(),
		})
		return
	}

	Broadcast("ok", fmt.Sprintf("Upload complete: %d functions, %d QA pairs, %d rules",
		result.Chunks, result.QAPairs, result.Rules), nil)

	c.JSON(http.StatusOK, gin.H{
		"status": 200,
		"data":   result,
		"error":  nil,
	})
}

// extractRules calls llama3 to extract validation rules from file content.
func extractRules(fileContent string) ([]ExtractedRule, error) {
	prompt := `You are a code analysis engine.
Read the code below and extract validation rules for its JSON output.
Return ONLY a raw JSON array. No markdown. No explanation. No preamble.

Each rule object:
{
  "id": "snake_case_unique_id",
  "description": "plain english",
  "type": "equality|range|presence|conditional|enum",
  "field": "field.path",
  "operator": "eq|neq|gt|gte|lt|lte|in|sum_of|not_null",
  "value": "<string|number|array>",
  "condition": { "field": "...", "operator": "...", "value": "..." },
  "severity": "error|warning"
}

Code:
` + fileContent

	response, err := Chat(prompt)
	if err != nil {
		return nil, fmt.Errorf("llm rule extraction: %w", err)
	}

	// Try to parse the response as a JSON array.
	trimmed := strings.TrimSpace(response)

	// Try to extract from potential markdown wrapping.
	extracted := extractJSON(trimmed)
	if extracted != "" {
		trimmed = extracted
	}

	// Handle the case where the LLM returns a JSON array inside markdown.
	if !strings.HasPrefix(trimmed, "[") {
		idx := strings.Index(trimmed, "[")
		if idx >= 0 {
			trimmed = trimmed[idx:]
		}
	}
	if lastIdx := strings.LastIndex(trimmed, "]"); lastIdx >= 0 {
		trimmed = trimmed[:lastIdx+1]
	}

	var rules []ExtractedRule
	if err := json.Unmarshal([]byte(trimmed), &rules); err != nil {
		return nil, fmt.Errorf("parse rules JSON: %w (raw: %.200s)", err, trimmed)
	}

	return rules, nil
}

// ─── POST /api/run (SSE Streaming) ──────────────────────────────────

type runRequest struct {
	Filename string `json:"filename" binding:"required"`
	Query    string `json:"query" binding:"required"`
}

// ginSSEWriter implements SSEWriter using Gin's response writer.
type ginSSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (g *ginSSEWriter) SendLog(msg string) {
	data, _ := json.Marshal(msg)
	fmt.Fprintf(g.w, "event: log\ndata: %s\n\n", data)
}

func (g *ginSSEWriter) SendToken(token string) {
	data, _ := json.Marshal(token)
	fmt.Fprintf(g.w, "event: token\ndata: %s\n\n", data)
}

func (g *ginSSEWriter) SendResult(result *PipelineResult) {
	data, _ := json.Marshal(result)
	fmt.Fprintf(g.w, "event: result\ndata: %s\n\n", data)
}

func (g *ginSSEWriter) Flush() {
	if g.flusher != nil {
		g.flusher.Flush()
	}
}

func handleRun(c *gin.Context) {
	var req runRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": 400, "data": nil,
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	log.Printf("[run] query='%s' file='%s'", req.Query, req.Filename)

	// Set SSE headers.
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, _ := c.Writer.(http.Flusher)
	sse := &ginSSEWriter{w: c.Writer, flusher: flusher}

	_, err := RunPipeline(appDB, req.Filename, req.Query, sse)
	if err != nil {
		// Send error as an SSE event.
		errData, _ := json.Marshal(err.Error())
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", errData)
		if flusher != nil {
			flusher.Flush()
		}
		return
	}

	// Signal completion.
	fmt.Fprintf(c.Writer, "event: done\ndata: {}\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

// ─── GET /api/files ─────────────────────────────────────────────────

func handleFiles(c *gin.Context) {
	files, err := GetFiles(appDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": 500, "data": nil,
			"error": "failed to list files: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": 200,
		"data":   files,
		"error":  nil,
	})
}

// ─── GET /api/rules ─────────────────────────────────────────────────

func handleRules(c *gin.Context) {
	filename := c.Query("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": 400, "data": nil,
			"error": "missing 'filename' query parameter",
		})
		return
	}

	rules, err := GetRules(appDB, filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": 500, "data": nil,
			"error": "failed to get rules: " + err.Error(),
		})
		return
	}

	// Convert to a frontend-friendly format.
	type ruleResp struct {
		ID          string `json:"id"`
		RuleID      string `json:"rule_id"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Field       string `json:"field"`
		Operator    string `json:"operator"`
		Severity    string `json:"severity"`
	}

	result := make([]ruleResp, 0, len(rules))
	for _, r := range rules {
		result = append(result, ruleResp{
			ID:          fmt.Sprintf("%d", r.ID),
			RuleID:      r.RuleID,
			Description: r.Description,
			Type:        r.RuleType,
			Field:       r.FieldPath,
			Operator:    r.Operator,
			Severity:    r.Severity,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": 200,
		"data":   result,
		"error":  nil,
	})
}

// ─── GET /api/corrections ───────────────────────────────────────────

func handleCorrections(c *gin.Context) {
	filename := c.Query("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": 400, "data": nil,
			"error": "missing 'filename' query parameter",
		})
		return
	}

	corrections, err := GetCorrections(appDB, filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": 500, "data": nil,
			"error": "failed to get corrections: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": 200,
		"data":   corrections,
		"error":  nil,
	})
}

// ─── GET /health ────────────────────────────────────────────────────

func handleHealth(c *gin.Context) {
	mysqlOK := appDB.Ping() == nil
	ollamaOK := CheckOllama()

	c.JSON(http.StatusOK, gin.H{
		"status": 200,
		"data": gin.H{
			"mysql":  mysqlOK,
			"ollama": ollamaOK,
		},
		"error": nil,
	})
}
