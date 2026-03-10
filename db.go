package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// InitDB connects to MySQL and ensures the schema exists.
func InitDB() (*sql.DB, error) {
	user := getEnvOrDefault("MYSQL_USER", "root")
	pass := getEnvOrDefault("MYSQL_PASS", "root1234")
	host := getEnvOrDefault("MYSQL_HOST", "localhost:3306")
	dbName := getEnvOrDefault("MYSQL_DB", "neuralflow")

	// First connect without a database to create it if needed.
	dsnNoDB := fmt.Sprintf("%s:%s@tcp(%s)/?parseTime=true&multiStatements=true", user, pass, host)
	tmpDB, err := sql.Open("mysql", dsnNoDB)
	if err != nil {
		return nil, fmt.Errorf("db open (no db): %w", err)
	}
	if _, err = tmpDB.Exec("CREATE DATABASE IF NOT EXISTS " + dbName); err != nil {
		tmpDB.Close()
		return nil, fmt.Errorf("create database: %w", err)
	}
	tmpDB.Close()

	// Now connect to the actual database.
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&multiStatements=true", user, pass, host, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Auto-run schema if tables don't exist.
	if err = ensureSchema(db); err != nil {
		return nil, fmt.Errorf("schema init: %w", err)
	}

	log.Println("[db] connected to MySQL successfully")
	return db, nil
}

// ensureSchema checks if tables exist and runs schema.sql if not.
func ensureSchema(db *sql.DB) error {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = DATABASE()
		AND table_name IN ('chunks','rules','corrections','runs')
	`).Scan(&count)
	if err != nil {
		return err
	}
	if count >= 4 {
		return nil // All tables exist.
	}

	schemaSQL, err := os.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema.sql: %w", err)
	}
	if _, err = db.Exec(string(schemaSQL)); err != nil {
		return fmt.Errorf("exec schema.sql: %w", err)
	}
	log.Println("[db] schema initialized from schema.sql")
	return nil
}

// ─── Chunk Operations ───────────────────────────────────────────────

// InsertChunk stores a chunk with its embedding into MySQL.
func InsertChunk(db *sql.DB, filename string, chunkIndex int, content string, embedding []float32) error {
	embJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("marshal embedding: %w", err)
	}
	_, err = db.Exec(
		`INSERT INTO chunks (filename, chunk_index, content, embedding) VALUES (?, ?, ?, ?)`,
		filename, chunkIndex, content, string(embJSON),
	)
	return err
}

// ─── Rule Operations ────────────────────────────────────────────────

// Rule represents a validation rule extracted from code.
type Rule struct {
	ID            int
	Filename      string
	RuleID        string
	Description   string
	RuleType      string
	FieldPath     string
	Operator      string
	ValueJSON     string
	ConditionJSON string
	Severity      string
}

// InsertRule stores an inferred rule into MySQL.
func InsertRule(db *sql.DB, filename string, r ExtractedRule) error {
	valJSON, err := json.Marshal(r.Value)
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}
	condJSON, err := json.Marshal(r.Condition)
	if err != nil {
		return fmt.Errorf("marshal condition: %w", err)
	}
	_, err = db.Exec(
		`INSERT INTO rules (filename, rule_id, description, rule_type, field_path, operator, value_json, condition_json, severity)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		filename, r.ID, r.Description, r.Type, r.Field, r.Operator,
		string(valJSON), string(condJSON), r.Severity,
	)
	return err
}

// ExtractedRule is the JSON shape returned by the LLM rule extraction prompt.
type ExtractedRule struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Type        string      `json:"type"`
	Field       string      `json:"field"`
	Operator    string      `json:"operator"`
	Value       interface{} `json:"value"`
	Condition   interface{} `json:"condition"`
	Severity    string      `json:"severity"`
}

// GetRules returns all rules for a given filename.
func GetRules(db *sql.DB, filename string) ([]Rule, error) {
	rows, err := db.Query(
		`SELECT id, filename, rule_id, description, rule_type, field_path, operator,
		        COALESCE(value_json, '{}'), COALESCE(condition_json, '{}'), severity
		 FROM rules WHERE filename = ? ORDER BY id`,
		filename,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]Rule, 0, 16)
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.Filename, &r.RuleID, &r.Description,
			&r.RuleType, &r.FieldPath, &r.Operator, &r.ValueJSON,
			&r.ConditionJSON, &r.Severity); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// ─── Correction Operations ──────────────────────────────────────────

// Correction represents a stored correction from a failed run.
type Correction struct {
	ID        int    `json:"id"`
	Filename  string `json:"filename"`
	Query     string `json:"query"`
	BadOutput string `json:"bad_output"`
	Errors    string `json:"errors"`
	CorrText  string `json:"correction"`
	CreatedAt string `json:"created_at"`
}

// InsertCorrection stores a correction with its embedding.
func InsertCorrection(db *sql.DB, filename, query, badOutput, errors, correction string, embedding []float32) error {
	embJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("marshal embedding: %w", err)
	}
	_, err = db.Exec(
		`INSERT INTO corrections (filename, query, bad_output, errors, correction, embedding)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		filename, query, badOutput, errors, correction, string(embJSON),
	)
	return err
}

// GetCorrections returns recent corrections for a given filename.
func GetCorrections(db *sql.DB, filename string) ([]Correction, error) {
	rows, err := db.Query(
		`SELECT id, filename, query, COALESCE(bad_output,''), COALESCE(errors,''),
		        COALESCE(correction,''), created_at
		 FROM corrections WHERE filename = ? ORDER BY id DESC LIMIT 20`,
		filename,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	corrections := make([]Correction, 0, 8)
	for rows.Next() {
		var c Correction
		if err := rows.Scan(&c.ID, &c.Filename, &c.Query, &c.BadOutput,
			&c.Errors, &c.CorrText, &c.CreatedAt); err != nil {
			return nil, err
		}
		corrections = append(corrections, c)
	}
	return corrections, rows.Err()
}

// ─── Run Operations ─────────────────────────────────────────────────

// InsertRun creates a new run record and returns its ID.
func InsertRun(db *sql.DB, graphJSON string) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO runs (graph_json, status, score, retries) VALUES (?, 'running', 0, 0)`,
		graphJSON,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateRun updates a run with its final state.
func UpdateRun(db *sql.DB, runID int64, status string, score float64, retries int, outputJSON string) error {
	_, err := db.Exec(
		`UPDATE runs SET status = ?, score = ?, retries = ?, output_json = ? WHERE id = ?`,
		status, score, retries, outputJSON, runID,
	)
	return err
}

// ─── File Listing ───────────────────────────────────────────────────

// FileInfo holds file metadata for the API response.
type FileInfo struct {
	Filename   string `json:"filename"`
	ChunkCount int    `json:"chunk_count"`
	RuleCount  int    `json:"rule_count"`
}

// GetFiles returns all uploaded filenames with chunk and rule counts.
func GetFiles(db *sql.DB) ([]FileInfo, error) {
	rows, err := db.Query(`
		SELECT c.filename,
		       COUNT(DISTINCT c.id) AS chunk_count,
		       COALESCE(r.rule_count, 0) AS rule_count
		FROM chunks c
		LEFT JOIN (
			SELECT filename, COUNT(*) AS rule_count
			FROM rules GROUP BY filename
		) r ON c.filename = r.filename
		GROUP BY c.filename
		ORDER BY c.filename
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]FileInfo, 0, 8)
	for rows.Next() {
		var f FileInfo
		if err := rows.Scan(&f.Filename, &f.ChunkCount, &f.RuleCount); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// ─── Vector Search (Cosine Similarity in Go) ────────────────────────

// chunkWithEmbedding is an internal struct for vector search.
type chunkWithEmbedding struct {
	content    string
	embedding  []float32
	similarity float64
}

// QuerySimilar retrieves the top K most similar chunks for a filename.
func QuerySimilar(db *sql.DB, filename string, queryEmbedding []float32, topK int) ([]string, error) {
	rows, err := db.Query(
		`SELECT content, embedding FROM chunks WHERE filename = ?`,
		filename,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := make([]chunkWithEmbedding, 0, 64)
	for rows.Next() {
		var content, embStr string
		if err := rows.Scan(&content, &embStr); err != nil {
			return nil, err
		}
		var emb []float32
		if err := json.Unmarshal([]byte(embStr), &emb); err != nil {
			log.Printf("[db] warning: failed to parse embedding for chunk, skipping: %v", err)
			continue
		}
		sim := cosineSimilarity(queryEmbedding, emb)
		chunks = append(chunks, chunkWithEmbedding{
			content:    content,
			embedding:  emb,
			similarity: sim,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Sort by similarity descending.
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].similarity > chunks[j].similarity
	})

	// Return top K contents.
	results := make([]string, 0, topK)
	for i := 0; i < len(chunks) && i < topK; i++ {
		results = append(results, chunks[i].content)
	}
	return results, nil
}

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns 0.0 if either vector is zero-length or all zeros.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, magA, magB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		magA += float64(a[i]) * float64(a[i])
		magB += float64(b[i]) * float64(b[i])
	}

	denom := math.Sqrt(magA) * math.Sqrt(magB)
	if denom == 0 {
		return 0.0
	}
	return dotProduct / denom
}

// DeleteFileData removes all chunks and rules for a filename (for re-upload).
func DeleteFileData(db *sql.DB, filename string) error {
	if _, err := db.Exec(`DELETE FROM chunks WHERE filename = ?`, filename); err != nil {
		return err
	}
	if _, err := db.Exec(`DELETE FROM rules WHERE filename = ?`, filename); err != nil {
		return err
	}
	return nil
}

// ─── Helpers ────────────────────────────────────────────────────────

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
