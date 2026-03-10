-- NeuralFlow Schema
-- Run: mysql -u root -proot1234 < schema.sql

CREATE DATABASE IF NOT EXISTS neuralflow;
USE neuralflow;

-- Stores uploaded file chunks + their embeddings
CREATE TABLE IF NOT EXISTS chunks (
  id                       INT AUTO_INCREMENT PRIMARY KEY,
  filename                 VARCHAR(255) NOT NULL,
  chunk_index              INT NOT NULL,
  function_name            VARCHAR(255),
  content                  TEXT NOT NULL,
  embedding                JSON,
  verbalization            TEXT,
  verbalization_embedding  JSON,
  created_at               TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_chunks_filename (filename),
  INDEX idx_chunks_function (filename, function_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Stores inferred validation rules per file
CREATE TABLE IF NOT EXISTS rules (
  id             INT AUTO_INCREMENT PRIMARY KEY,
  filename       VARCHAR(255) NOT NULL,
  rule_id        VARCHAR(100) NOT NULL,
  description    TEXT,
  rule_type      VARCHAR(50) NOT NULL,
  field_path     VARCHAR(255),
  operator       VARCHAR(50),
  value_json     JSON,
  condition_json JSON,
  severity       VARCHAR(20) DEFAULT 'error',
  created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_rules_filename (filename)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Stores corrections from failed runs
CREATE TABLE IF NOT EXISTS corrections (
  id         INT AUTO_INCREMENT PRIMARY KEY,
  filename   VARCHAR(255) NOT NULL,
  query      TEXT,
  bad_output TEXT,
  errors     TEXT,
  correction TEXT,
  embedding  JSON,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_corrections_filename (filename)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Stores pipeline run history
CREATE TABLE IF NOT EXISTS runs (
  id          INT AUTO_INCREMENT PRIMARY KEY,
  graph_json  JSON,
  status      VARCHAR(20) NOT NULL,
  score       FLOAT DEFAULT 0,
  retries     INT DEFAULT 0,
  output_json JSON,
  created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Stores Q&A pairs generated from code verbalizations
CREATE TABLE IF NOT EXISTS qa_pairs (
  id            INT AUTO_INCREMENT PRIMARY KEY,
  filename      VARCHAR(255) NOT NULL,
  function_name VARCHAR(255),
  question      TEXT,
  answer_json   JSON,
  embedding     JSON,
  created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_qa_filename (filename)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

