# NeuralFlow

**Visual AI Pipeline Builder** — A Go prototype that uploads code files, verbalizes PHP functions into plain English, generates Q&A test pairs, extracts validation rules via LLM, and runs RAG queries with SSE-streamed output and self-correction.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌───────────┐
│  Frontend    │◄SSE─│  Gin Server  │────►│  Ollama   │
│  (React 18)  │────►│  (Go :8080)  │     │  llama3   │
└─────────────┘     └──────┬───────┘     │  nomic    │
                           │             └───────────┘
                    ┌──────▼───────┐
                    │    MySQL     │
                    │  chunks      │
                    │  rules       │
                    │  corrections │
                    │  runs        │
                    │  qa_pairs    │
                    └──────────────┘
```

## Prerequisites

- **Go 1.22+**
- **Node.js & npm** (for the React UI)
- **MySQL** running on `localhost:3306`
- **Ollama** running on `localhost:11434` with models: `llama3`, `nomic-embed-text`

## Quick Start

1. **Clone & Database:**
```bash
git clone https://github.com/entreya/neuralflow.git
cd neuralflow
go mod tidy
mysql -u root -proot1234 < schema.sql
cp .env.example .env
```

2. **Start the Go Backend:**
```bash
cd backend
go run .
# Starts on http://localhost:8080
```

3. **Start the React Frontend:**
```bash
cd frontend
npm install
npm run dev
# Starts on http://localhost:3000
```

Open **http://localhost:3000** in your browser.

## Environment Variables

| Variable     | Default          | Description         |
|-------------|------------------|---------------------|
| `MYSQL_USER` | `root`           | MySQL username      |
| `MYSQL_PASS` | `root1234`       | MySQL password      |
| `MYSQL_HOST` | `localhost:3306` | MySQL host:port     |
| `MYSQL_DB`   | `neuralflow`     | Database name       |

## API Reference

| Method | Endpoint                     | Description                              |
|--------|------------------------------|------------------------------------------|
| POST   | `/api/parse`                 | Upload a file and extract function names |
| POST   | `/api/train`                 | Train selected parsed methods            |
| POST   | `/api/training/stop`         | Cancel all in-flight training instantly  |
| GET    | `/api/training/status`       | Check if training is active              |
| GET    | `/api/models`                | List available Ollama models             |
| GET    | `/api/config`                | Get current model configuration          |
| POST   | `/api/config`                | Update model, temp, thinking settings    |
| POST   | `/api/run`                   | Run a RAG pipeline query (SSE stream)    |
| GET    | `/api/files`                 | List uploaded files with stats           |
| GET    | `/api/rules`                 | Get inferred rules for a file            |
| GET    | `/api/corrections`           | Get corrections for a file               |
| GET    | `/api/logs`                  | Stream backend execution logs via SSE    |
| GET    | `/health`                    | Check MySQL + Ollama connectivity        |

### POST /api/parse

```bash
curl -F "files=@ExamFee.php" http://localhost:8080/api/parse
```

### POST /api/run (SSE)

The response is a `text/event-stream` with events: `log`, `token`, `result`, `error`, `done`.

```bash
curl -N -X POST http://localhost:8080/api/run \
  -H "Content-Type: application/json" \
  -d '{"filename":"ExamFee.php","query":"student with 3 appearing papers"}'
```

## Project Structure

```
neuralflow/
├── backend/
│   ├── main.go        ← Gin server + API routes + SSE handler
│   ├── db.go          ← MySQL connection + queries + vector search + QA pairs
│   ├── ollama.go      ← Embed, Chat, ChatStream, Verbalize, GenerateQA
│   ├── pipeline.go    ← ProcessUpload + RAG pipeline + self-correction loop
│   ├── evaluator.go   ← Two-tier rule engine (universal + inferred)
│   ├── schema.sql     ← MySQL CREATE TABLE statements (5 tables)
│   └── go.mod / go.sum
└── frontend/
    ├── src/           ← React 18, Vite, Material UI 5, Zustand, React Query
    └── package.json
```

## Dependencies

| Package                          | Purpose        |
|----------------------------------|----------------|
| `github.com/gin-gonic/gin`      | HTTP server    |
| `github.com/go-sql-driver/mysql` | MySQL driver   |
| `github.com/ollama/ollama`       | LLM API client |

## How It Works

1. **Upload** — PHP functions are parsed (regex + brace matching), each is verbalized by `llama3` into plain English, the verbalization is embedded and stored. 6 Q&A test pairs are generated per function. Rules are extracted via LLM.
2. **Query** — Query is embedded, top 3 similar functions retrieved via verbalization cosine similarity + top 3 Q&A examples
3. **Generate** — LLM generates JSON output using enriched context (code + QA examples + rules), streamed via SSE
4. **Evaluate** — Two-tier scoring: universal checks (40%) + inferred rules (60%)
5. **Self-Correct** — If score < 0.75, corrections are stored and injected into retries (max 3)

