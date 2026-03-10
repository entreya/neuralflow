package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ─── Model Config ───────────────────────────────────────────────────

type ModelConfig struct {
	ChatModel   string  `json:"chat_model"`
	EmbedModel  string  `json:"embed_model"`
	Thinking    bool    `json:"thinking"`
	Temperature float64 `json:"temperature"`
}

var (
	activeModelConfig = ModelConfig{
		ChatModel:   "llama3",
		EmbedModel:  "nomic-embed-text",
		Thinking:    false,
		Temperature: 0.0,
	}
	modelConfigMu sync.RWMutex
)

// GetActiveModel returns the currently configured chat model name.
func GetActiveModel() string {
	modelConfigMu.RLock()
	defer modelConfigMu.RUnlock()
	return activeModelConfig.ChatModel
}

// GetActiveEmbedModel returns the currently configured embedding model name.
func GetActiveEmbedModel() string {
	modelConfigMu.RLock()
	defer modelConfigMu.RUnlock()
	return activeModelConfig.EmbedModel
}

// GetActiveTemperature returns the currently configured temperature.
func GetActiveTemperature() float64 {
	modelConfigMu.RLock()
	defer modelConfigMu.RUnlock()
	return activeModelConfig.Temperature
}

// GetActiveThinking returns whether thinking/reasoning mode is enabled.
func GetActiveThinking() bool {
	modelConfigMu.RLock()
	defer modelConfigMu.RUnlock()
	return activeModelConfig.Thinking
}

// Global training state
var (
	trainingCtx    context.Context
	trainingCancel context.CancelFunc
	trainingMu     sync.Mutex
	trainingActive bool
)

func init() {
	// start with a cancelled context so IsTraining() = false initially
	trainingCtx, trainingCancel = context.WithCancel(context.Background())
	trainingCancel() // cancelled = no training running
}

// StartTraining creates a fresh cancellable context for a new training run.
// Returns false if training is already in progress.
func StartTraining() (context.Context, bool) {
	trainingMu.Lock()
	defer trainingMu.Unlock()
	if trainingActive {
		return nil, false
	}
	trainingCtx, trainingCancel = context.WithCancel(context.Background())
	trainingActive = true
	return trainingCtx, true
}

// StopTraining cancels the active training context immediately.
func StopTraining() {
	trainingMu.Lock()
	defer trainingMu.Unlock()
	if trainingCancel != nil {
		trainingCancel()
	}
	trainingActive = false
	Broadcast("warn", "Training stopped by user — all Ollama requests cancelled", nil)
}

// MarkTrainingDone is called when training finishes naturally.
func MarkTrainingDone() {
	trainingMu.Lock()
	defer trainingMu.Unlock()
	trainingActive = false
	Broadcast("ok", "Training complete", nil)
}

// IsTraining returns whether training is currently running.
func IsTraining() bool {
	trainingMu.Lock()
	defer trainingMu.Unlock()
	return trainingActive
}

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
		api.POST("/parse", handleParse)
		api.POST("/train", handleTrain)
		api.POST("/training/stop", handleTrainingStop)
		api.GET("/training/status", handleTrainingStatus)
		api.POST("/run", handleRun)
		api.GET("/files", handleFiles)
		api.GET("/rules", handleRules)
		api.GET("/corrections", handleCorrections)
		api.GET("/logs", handleLogs)
		api.GET("/models", handleGetModels)
		api.GET("/config", handleGetConfig)
		api.POST("/config", handleSetConfig)
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

// ─── POST /api/parse ───────────────────────────────────────────────

type ParsedMethod struct {
	Name       string `json:"name"`
	Visibility string `json:"visibility"` // "public" | "protected" | "private"
	IsStatic   bool   `json:"is_static"`
	Lines      int    `json:"lines"`
	QAPairs    int    `json:"qa_pairs"`
	TrainedAt  string `json:"trained_at"`
	Status     string `json:"status"` // 'trained' or 'not_trained'
}

type ParsedFile struct {
	Path     string         `json:"path"`
	Filename string         `json:"filename"`
	Methods  []ParsedMethod `json:"methods"`
}

func handleParse(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form: " + err.Error()})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}

	// Ensure uploads directory exists
	os.MkdirAll("uploads", 0755)

	var resultFiles []ParsedFile

	for _, fileHeader := range files {
		// Get the original path as passed by the client (used as logical ID)
		originalPath := fileHeader.Filename // e.g. "src/modules/ExaminationFee.php"

		// Use only the base name for actual disk storage
		baseName := filepath.Base(originalPath)
		diskPath := "uploads/" + baseName

		if err := c.SaveUploadedFile(fileHeader, diskPath); err != nil {
			log.Printf("[parse] warning: failed to save %s: %v", baseName, err)
			continue
		}

		content, err := os.ReadFile(diskPath)
		if err != nil {
			continue
		}

		funcs := parsePHPFunctions(string(content))
		if len(funcs) == 0 {
			continue
		}

		// Query DB for trained methods with counts and timestamps
		type methodInfo struct {
			count     int
			trainedAt string
		}
		trainedMethods := make(map[string]methodInfo)

		rows, err := appDB.Query(
			`SELECT function_name, COUNT(*) as cnt, MAX(created_at) as ts
			 FROM qa_pairs WHERE filename = ?
			 GROUP BY function_name`,
			diskPath,
		)
		if err == nil {
			for rows.Next() {
				var fn string
				var cnt int
				var ts string
				if err := rows.Scan(&fn, &cnt, &ts); err == nil {
					trainedMethods[fn] = methodInfo{count: cnt, trainedAt: ts}
				}
			}
			rows.Close()
		}

		var methods []ParsedMethod
		for _, fn := range funcs {
			info, isTrained := trainedMethods[fn.Name]
			status := "not_trained"
			if isTrained {
				status = "trained"
			}
			methods = append(methods, ParsedMethod{
				Name:       fn.Name,
				Visibility: fn.Visibility,
				IsStatic:   fn.IsStatic,
				Lines:      fn.Lines,
				QAPairs:    info.count,
				TrainedAt:  info.trainedAt,
				Status:     status,
			})
		}

		resultFiles = append(resultFiles, ParsedFile{
			Path:     diskPath, // used for DB queries and RetrainMethod
			Filename: baseName, // display name
			Methods:  methods,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"files": resultFiles,
	})
}

// ─── POST /api/train ───────────────────────────────────────────────

type TrainPayload struct {
	Files []struct {
		Filename string   `json:"filename"`
		Path     string   `json:"path"`
		Methods  []string `json:"methods"`
	} `json:"files"`
}

func handleTrain(c *gin.Context) {
	var payload TrainPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON mapping"})
		return
	}

	trainCtx, ok := StartTraining()
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": "Training already in progress"})
		return
	}

	// Calculate total methods to queue
	totalMethods := 0
	for _, f := range payload.Files {
		totalMethods += len(f.Methods)
	}

	if totalMethods == 0 {
		MarkTrainingDone()
		c.JSON(http.StatusOK, gin.H{"queued": 0})
		return
	}

	go func() {
		defer MarkTrainingDone()
		for _, f := range payload.Files {
			for _, m := range f.Methods {
				// Check for cancellation before starting the next method
				select {
				case <-trainCtx.Done():
					return
				default:
				}

				Broadcast("info", fmt.Sprintf("Processing queued method: %s", m), map[string]interface{}{
					"queued": true, "fnName": m,
				})

				appDB.Exec("DELETE FROM qa_pairs WHERE filename = ? AND function_name = ?", f.Path, m)
				appDB.Exec("DELETE FROM chunks WHERE filename = ? AND function_name = ?", f.Path, m)

				err := RetrainMethod(trainCtx, appDB, f.Path, m)
				if err != nil {
					// Only broadcast error if it wasn't a cancellation
					if err != context.Canceled {
						Broadcast("error", fmt.Sprintf("Train failed for %s", m), map[string]interface{}{"fnName": m})
					}
				}
			}
		}
	}()

	c.JSON(http.StatusOK, gin.H{"queued": totalMethods})
}

func handleTrainingStop(c *gin.Context) {
	StopTraining()
	c.JSON(http.StatusOK, gin.H{"stopped": true, "message": "All training cancelled"})
}

func handleTrainingStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"active": IsTraining()})
}

// extractRules calls llama3 to extract validation rules from file content.
func extractRules(ctx context.Context, fileContent string) ([]ExtractedRule, error) {
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

	response, err := Chat(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm rule extraction: %w", err)
	}

	trimmed := strings.TrimSpace(response)
	extracted := extractJSON(trimmed)
	if extracted != "" {
		trimmed = extracted
	}

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, _ := c.Writer.(http.Flusher)
	sse := &ginSSEWriter{w: c.Writer, flusher: flusher}

	_, err := RunPipeline(c.Request.Context(), appDB, req.Filename, req.Query, sse)
	if err != nil {
		errData, _ := json.Marshal(err.Error())
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", errData)
		if flusher != nil {
			flusher.Flush()
		}
		return
	}

	fmt.Fprintf(c.Writer, "event: done\ndata: {}\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func handleFiles(c *gin.Context) {
	files, err := GetFiles(appDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list files: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": files})
}

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

// ─── Model Endpoints ────────────────────────────────────────────────

func handleGetModels(c *gin.Context) {
	models, err := ListOllamaModels()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"models": []string{"llama3", "qwen3:4b", "qwen3:8b"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func handleGetConfig(c *gin.Context) {
	modelConfigMu.RLock()
	defer modelConfigMu.RUnlock()
	c.JSON(http.StatusOK, activeModelConfig)
}

func handleSetConfig(c *gin.Context) {
	var cfg ModelConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if cfg.ChatModel == "" {
		cfg.ChatModel = "llama3"
	}
	if cfg.EmbedModel == "" {
		cfg.EmbedModel = "nomic-embed-text"
	}

	modelConfigMu.Lock()
	activeModelConfig = cfg
	modelConfigMu.Unlock()

	Broadcast("info", fmt.Sprintf("Model switched to %s (thinking=%v, temp=%.1f)",
		cfg.ChatModel, cfg.Thinking, cfg.Temperature), nil)
	c.JSON(http.StatusOK, activeModelConfig)
}
