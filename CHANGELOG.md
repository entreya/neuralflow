# Changelog

All notable changes to **NeuralFlow** will be documented in this file.

## [Unreleased]

### Added
- [FEAT] MySQL schema with 4 tables: `chunks`, `rules`, `corrections`, `runs`
- [FEAT] Gin HTTP server with 6 API endpoints (`/api/upload`, `/api/run`, `/api/files`, `/api/rules`, `/api/corrections`, `/health`)
- [FEAT] Ollama integration: `Embed()` via nomic-embed-text, `Chat()` via llama3
- [FEAT] In-Go cosine similarity vector search (no ChromaDB needed)
- [FEAT] Two-tier evaluator: universal JSON checks (40%) + inferred rules (60%)
- [FEAT] Self-correction loop with up to 3 retries, storing corrections with embeddings
- [FEAT] LLM-powered rule extraction from uploaded code files
- [FEAT] Smart file chunking on function/class boundaries (~512 chars)
- [FEAT] Single-file vanilla JS frontend with 3-column layout
- [FEAT] JSON syntax highlighting in output display
- [FEAT] Real-time health check indicators for MySQL and Ollama
- [FEAT] Drag-and-drop file upload with progress bar
- [FEAT] Rules and corrections tabs in right panel
- [FEAT] Pipeline log console showing step-by-step execution
- [FEAT] Typewriter effect for JSON output with blinking cursor and staggered console log reveal
- [FEAT] Code verbalization pipeline — PHP functions → plain English → embedding → QA pairs
- [FEAT] SSE streaming for `/api/run` — real-time LLM tokens, log events, and results
- [FEAT] `qa_pairs` table for storing generated Q&A test cases per function
- [FEAT] PHP function parser with regex + brace-depth matching
- [FEAT] `Verbalize()` and `GenerateQA()` Ollama wrapper functions
- [FEAT] `ChatStream()` for streaming LLM output token-by-token
- [FEAT] Verbalization-based similarity search (falls back to raw embeddings)
- [FEAT] `ProcessUpload()` — centralized upload flow with per-function progress logging
- [PERF] Enriched RAG context: code chunks + QA examples + corrections in prompt
- [FEAT] Console tab in right panel with real-time SSE log streaming via `/api/logs`
- [FEAT] `Broadcast()` function — fan-out log events to all connected SSE clients
- [FEAT] Color-coded log lines (info/ok/warn/error/data/progress) with mini progress bars
- [FEAT] Unread badge on Console tab, autoscroll toggle, 500-line pruning, re-connect with retry
