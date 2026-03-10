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

## [Unreleased]
## [Unreleased]
- [FEAT] Rewrote entire React 18 UI utilizing Vite, Material UI 5, Zustand, and React Query inside local `frontend/` directory.
- [FEAT] Implemented granular MethodSelection GUI allowing parsing of PHP functions prior to training.
- [FEAT] Replaced monolithic `/api/upload` route with `/api/parse` (extraction only) and `/api/train` (queue-based training).
- [FEAT] Integrated Server-Sent Events (SSE) into a terminal-style Log Console Panel tracking async pipeline progress.
- [FEAT] Implemented strict layout constraints via TopBar, ModelPicker, UploadDropzone, MethodAccordion, and Output panels.
- [FEAT] Added React Query pooling on `/health`, `/files`, `/rules`, and `/corrections` to fetch immediately on app focus.
- [REFACTOR] Removed temporary `neuralflow-ui` and static HTML builds. Project is now strictly `backend/` and `frontend/`.
- [FEAT] Global "Stop Training" button with context.Context cancellation across all Ollama HTTP calls.
- [FEAT] `POST /api/training/stop` route — cancels all in-flight training immediately.
- [FEAT] `GET /api/training/status` route — returns `{"active": true/false}` for frontend polling.
- [FEAT] Pulsing red "Stop Training" button in TopBar with indeterminate LinearProgress bar.
- [REFACTOR] All Ollama functions (`Chat`, `Embed`, `Verbalize`, `GenerateQA`) now accept `context.Context` for graceful cancellation.
- [FEAT] Training context manager (`StartTraining`, `StopTraining`, `MarkTrainingDone`, `IsTraining`) in `main.go`.
- [STYLE] Switched global typography from IBM Plex Mono to Inter sans-serif.
- [FIX] SSE log stream now wired globally via `useLogs()` in `App.jsx` — logs persist across tab switches.
- [FIX] `ConsolePanel` reads from Zustand store instead of local `useState` — no more lost logs on unmount.
- [FEAT] Added `logs`, `logConnected`, `unreadLogs`, `consoleVisible` state to `uploadStore.js`.
- [FEAT] Reconnect with exponential backoff (1s → 15s, 5 attempts) on SSE disconnect.
- [FIX] Bug 1: Added `/health` to Vite proxy; removed `baseURL: ''` override in TopBar health query.
- [FEAT] Bug 2: Added `GET /api/models`, `GET /api/config`, `POST /api/config` routes + handlers + in-memory `ModelConfig` state.
- [FIX] Bug 3: Replaced hardcoded `"llama3"` with `GetActiveModel()` in `ChatWithSystem` and `ChatStream`. Added `ListOllamaModels()`.
- [FIX] Bug 4: `handleParse` now uses `filepath.Base()` for disk storage, avoiding subdirectory creation failures.
- [FIX] Bug 5: `updateFromSSE` now parses function names from message text via regex when `meta.fnName` is absent.
- [FIX] Bug 6: Added `onSuccess` handler to `useTrainMethods` documenting async training lifecycle.
- [FIX] Bug 7: File dropzone hidden `<input>` always rendered in DOM so `openFileDialog()` works from card view.
- [FIX] Bug 8: Added `setFileSelection` batch action; `MethodAccordion` multi-select now registers all rows atomically.
- [FIX] T1-CRITICAL: `ProcessUpload` + `RetrainMethod` now check `ctx.Done()` at loop top and after every Ollama call; `context.Canceled` exits immediately instead of falling back.
- [FIX] T2-CRITICAL: `handleTrain` DELETE now uses `WHERE function_name = ?` instead of broken self-referencing MySQL subquery that nuked all chunks.
- [FIX] T3-CRITICAL: `RetrainMethod` uses `InsertChunkWithFunction` with `chunk_index=0` instead of accumulating `chunk_index=-1` duplicates.
- [FEAT] T4: Added `function_name` column to `chunks` table with compound index; auto-migration for existing DBs; new `InsertChunkWithFunction` in `db.go`.
- [FEAT] T5: `PHPFunction` now captures visibility, line count, and static modifier; `ParsedMethod` includes `visibility`, `is_static`, `lines`, `qa_pairs`, `trained_at`.
- [FIX] T6: Embed failure no longer broadcasts misleading "ok" — warns that verbalization is saved but NOT retrievable.
- [FIX] T7: All pipeline Broadcasts now include `fnName` and `stage` in meta, removing need for frontend regex parsing.
- [FIX] `Embed()` now uses `GetActiveEmbedModel()` instead of hardcoded `"nomic-embed-text"` — embed model switching works.
- [FIX] `ChatWithSystem()` + `ChatStream()` now pass `temperature` from config via `Options` map — temperature slider works.
- [FEAT] Thinking mode prepends a step-by-step reasoning system directive when enabled.
