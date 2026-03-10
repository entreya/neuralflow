package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Global database handle, initialized in main().
var appDB *sql.DB

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
	}
	r.GET("/health", handleHealth)

	log.Println("[main] NeuralFlow running on http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("[main] server failed: %v", err)
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
	log.Printf("[upload] received file: %s (%d bytes)", filename, header.Size)

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
	fileContent := string(content)

	// Clear previous data for this file (re-upload support).
	if err := DeleteFileData(appDB, filename); err != nil {
		log.Printf("[upload] warning: failed to clear old data for %s: %v", filename, err)
	}

	// ── Step 1: Chunk the file ──────────────────────────────────
	chunks := chunkContent(fileContent, 512)
	log.Printf("[upload] split into %d chunks", len(chunks))

	// ── Step 2: Embed and store each chunk ──────────────────────
	for i, chunk := range chunks {
		embedding, err := Embed(chunk)
		if err != nil {
			log.Printf("[upload] warning: embed failed for chunk %d: %v", i, err)
			// Store without embedding.
			if err := InsertChunk(appDB, filename, i, chunk, nil); err != nil {
				log.Printf("[upload] error: insert chunk %d: %v", i, err)
			}
			continue
		}
		if err := InsertChunk(appDB, filename, i, chunk, embedding); err != nil {
			log.Printf("[upload] error: insert chunk %d: %v", i, err)
		}
	}

	// ── Step 3: Extract rules via LLM ───────────────────────────
	rulesCount := 0
	extractedRules, err := extractRules(fileContent)
	if err != nil {
		log.Printf("[upload] warning: rule extraction failed: %v", err)
	} else {
		for _, rule := range extractedRules {
			if err := InsertRule(appDB, filename, rule); err != nil {
				log.Printf("[upload] warning: insert rule failed: %v", err)
			} else {
				rulesCount++
			}
		}
	}

	log.Printf("[upload] done: %d chunks, %d rules for %s", len(chunks), rulesCount, filename)

	c.JSON(http.StatusOK, gin.H{
		"status": 200,
		"data": gin.H{
			"chunks":   len(chunks),
			"rules":    rulesCount,
			"filename": filename,
		},
		"error": nil,
	})
}

// chunkContent splits content into ~maxSize char chunks on function/class boundaries.
func chunkContent(content string, maxSize int) []string {
	lines := strings.Split(content, "\n")
	chunks := make([]string, 0, len(lines)/10+1)
	var current strings.Builder

	// Boundary markers for common languages.
	boundaries := []string{"function ", "func ", "class ", "public ", "private ", "protected ", "def ", "module "}

	for _, line := range lines {
		// Check if this line starts a new logical block and current chunk is large enough.
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

		// Hard split if chunk gets too large.
		if current.Len() >= maxSize {
			chunks = append(chunks, current.String())
			current.Reset()
		}
	}

	// Don't forget the last chunk.
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
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
		// Try to find the array start.
		idx := strings.Index(trimmed, "[")
		if idx >= 0 {
			trimmed = trimmed[idx:]
		}
	}
	// Find the array end.
	if lastIdx := strings.LastIndex(trimmed, "]"); lastIdx >= 0 {
		trimmed = trimmed[:lastIdx+1]
	}

	var rules []ExtractedRule
	if err := json.Unmarshal([]byte(trimmed), &rules); err != nil {
		return nil, fmt.Errorf("parse rules JSON: %w (raw: %.200s)", err, trimmed)
	}

	return rules, nil
}

// ─── POST /api/run ──────────────────────────────────────────────────

type runRequest struct {
	Filename string `json:"filename" binding:"required"`
	Query    string `json:"query" binding:"required"`
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

	result, err := RunPipeline(appDB, req.Filename, req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": 500, "data": nil,
			"error": "pipeline failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": 200,
		"data":   result,
		"error":  nil,
	})
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
