# NeuralFlow

**Visual AI Pipeline Builder** — A working Go prototype that lets you upload code files, extract validation rules via LLM, run RAG queries against the code, and self-correct using an evaluator loop.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌───────────┐
│  Frontend    │────►│  Gin Server  │────►│  Ollama   │
│  (Vanilla JS)│    │  (Go :8080)  │     │  llama3   │
└─────────────┘     └──────┬───────┘     │  nomic    │
                           │             └───────────┘
                    ┌──────▼───────┐
                    │    MySQL     │
                    │  chunks      │
                    │  rules       │
                    │  corrections │
                    │  runs        │
                    └──────────────┘
```

## Prerequisites

- **Go 1.22+**
- **MySQL** running on `localhost:3306`
- **Ollama** running on `localhost:11434` with models: `llama3`, `nomic-embed-text`

## Quick Start

```bash
cd neuralflow
go mod tidy
mysql -u root -proot1234 < schema.sql
go run .
```

Open **http://localhost:8080** in your browser.

## Environment Variables

| Variable     | Default          | Description         |
|-------------|------------------|---------------------|
| `MYSQL_USER` | `root`           | MySQL username      |
| `MYSQL_PASS` | `root1234`       | MySQL password      |
| `MYSQL_HOST` | `localhost:3306` | MySQL host:port     |
| `MYSQL_DB`   | `neuralflow`     | Database name       |

## API Reference

| Method | Endpoint             | Description                          |
|--------|----------------------|--------------------------------------|
| POST   | `/api/upload`        | Upload a file (multipart form)       |
| POST   | `/api/run`           | Run a RAG pipeline query             |
| GET    | `/api/files`         | List uploaded files with stats       |
| GET    | `/api/rules`         | Get inferred rules for a file        |
| GET    | `/api/corrections`   | Get corrections for a file           |
| GET    | `/health`            | Check MySQL + Ollama connectivity    |

### POST /api/upload

```bash
curl -F "file=@ExamFee.php" http://localhost:8080/api/upload
```

### POST /api/run

```bash
curl -X POST http://localhost:8080/api/run \
  -H "Content-Type: application/json" \
  -d '{"filename":"ExamFee.php","query":"student with 3 appearing papers"}'
```

## Project Structure

```
neuralflow/
├── main.go        ← Gin server + API routes
├── db.go          ← MySQL connection + queries + vector search
├── ollama.go      ← Embed() and Chat() wrappers
├── pipeline.go    ← RAG pipeline + self-correction loop
├── evaluator.go   ← Two-tier rule engine (universal + inferred)
├── schema.sql     ← MySQL CREATE TABLE statements
├── go.mod / go.sum
└── static/
    └── index.html ← Complete single-file UI
```

## Dependencies

| Package                          | Purpose        |
|----------------------------------|----------------|
| `github.com/gin-gonic/gin`      | HTTP server    |
| `github.com/go-sql-driver/mysql` | MySQL driver   |
| `github.com/ollama/ollama`       | LLM API client |

## How It Works

1. **Upload** — File is chunked (~512 chars), each chunk embedded with `nomic-embed-text`, rules extracted via `llama3`
2. **Query** — Query is embedded, top 5 similar chunks retrieved via cosine similarity in Go
3. **Generate** — LLM generates JSON output using RAG context + rules
4. **Evaluate** — Two-tier scoring: universal checks (40%) + inferred rules (60%)
5. **Self-Correct** — If score < 0.75, corrections are stored and injected into retries (max 3)
