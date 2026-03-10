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
