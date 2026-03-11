# Memory & Knowledge System Specification

Memory is Silo's persistent knowledge layer — facts, preferences, and document chunks that span across sessions. **V0 uses ADK's `InMemoryMemoryService` (ephemeral, per-session). The full memory system described below is the V1 target.**

---

## 1. V0: ADK In-Memory (Ephemeral)

V0 uses ADK's built-in `InMemoryMemoryService`. This provides:
- Per-session ephemeral memory (cleared when session ends)
- No persistent cross-session knowledge
- No embeddings, no hybrid search, no document ingestion
- Zero configuration, zero additional dependencies

```go
import "google.golang.org/adk/memory"

memoryService := memory.NewInMemoryMemoryService()
```

This is sufficient for V0 — the agent has session history via ADK's `SessionService` and doesn't need cross-session memory to be useful.

---

## 2. V1: Full Memory System (Target)

The entire spec below describes the V1 persistent memory system. When implemented, it will replace `InMemoryMemoryService` with a custom ADK `MemoryService` backed by SQLite + FTS5 + vector embeddings.

### Memory vs. Sessions

| Concern | Sessions ([06_sessions.md](06_sessions.md)) | Memory (this spec) |
|---------|----------------------------------------------|---------------------|
| Scope | Single conversation (`session_id`) | Cross-session, global |
| Content | Message history (user/assistant turns) | Facts, preferences, document chunks |
| Lifetime | Expires after `retention_days` | Persists until explicitly deleted |
| Storage | `sessions.db` (ADK managed) | `memory.db` (custom) |

### Storage

Separate SQLite file: `~/.silo/memory.db`. Uses:
- SQLite + FTS5 for full-text search (BM25 ranking)
- BLOB columns for vector embeddings (f32 little-endian)
- Cosine similarity computed in Go
- No external vector DB dependency

---

## 3. V1 Data Model & SQLite Schema

Three tables plus one FTS5 virtual table, all in `memory.db`.

### `knowledge_entries` — one row per knowledge unit

```sql
CREATE TABLE knowledge_entries (
    id              TEXT PRIMARY KEY,           -- ULID
    content         TEXT NOT NULL,
    entry_type      TEXT NOT NULL,              -- 'fact' | 'document_chunk' | 'preference'
    source          TEXT NOT NULL,              -- 'user' | 'conversation' | 'ingestion'
    source_ref      TEXT,
    chunk_index     INTEGER,
    parent_id       TEXT,
    tags            TEXT NOT NULL DEFAULT '[]',
    token_count     INTEGER NOT NULL,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL,
    metadata        TEXT NOT NULL DEFAULT '{}'
);
```

### `embeddings` — 1:1 with knowledge_entries

```sql
CREATE TABLE embeddings (
    entry_id    TEXT PRIMARY KEY REFERENCES knowledge_entries(id) ON DELETE CASCADE,
    embedding   BLOB NOT NULL,
    model       TEXT NOT NULL,
    dimensions  INTEGER NOT NULL,
    created_at  INTEGER NOT NULL
);
```

### `knowledge_fts` — FTS5 external content table

```sql
CREATE VIRTUAL TABLE knowledge_fts USING fts5(
    content,
    tags,
    content=knowledge_entries,
    content_rowid=rowid
);
```

---

## 4. V1 Embedding Pipeline

### Embedder Interface

```go
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimensions() int
    ModelName() string
}
```

### V1: API Embedder

Calls the provider's embedding API (e.g., OpenAI `text-embedding-3-small`).

### V2: Local Embedder

ONNX runtime for offline/private embeddings.

---

## 5. V1 Hybrid Search

Three stages: FTS5/BM25, vector cosine similarity, and Reciprocal Rank Fusion (RRF).

### Stage 1 — FTS5/BM25

```sql
SELECT ke.id, ke.content, ke.entry_type, ke.source, ke.token_count,
       bm25(knowledge_fts) AS bm25_score
FROM knowledge_fts
JOIN knowledge_entries ke ON ke.rowid = knowledge_fts.rowid
WHERE knowledge_fts MATCH ?
ORDER BY bm25(knowledge_fts)
LIMIT ?
```

### Stage 2 — Vector Cosine Similarity

Embed the query, compute cosine similarity against all stored embeddings:

```go
func cosineSimilarity(a, b []float32) float32 {
    var dot, normA, normB float32
    for i := range a {
        dot += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }
    if normA == 0 || normB == 0 {
        return 0
    }
    return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
```

### Stage 3 — Reciprocal Rank Fusion (RRF)

```
RRF_score(entry) = Σ  1 / (k + rank_i)    for i ∈ {bm25, vector}
```

Where `k = 60` (standard constant).

### Type-Based Priority Bonus

| Entry Type | Multiplier |
|------------|-----------|
| `preference` | × 1.5 |
| `fact` | × 1.2 |
| `document_chunk` | × 1.0 |

---

## 6. V1 MemoryService Interface

Implements ADK's `MemoryService` interface backed by custom SQLite:

```go
type SiloMemoryService struct {
    db       *sqlx.DB
    embedder Embedder
}

// Implements adk.MemoryService
func (s *SiloMemoryService) Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryResult, error)
func (s *SiloMemoryService) Store(ctx context.Context, entry MemoryEntry) (string, error)
func (s *SiloMemoryService) Delete(ctx context.Context, entryID string) error
func (s *SiloMemoryService) List(ctx context.Context, opts ListOptions) ([]MemoryEntry, error)
func (s *SiloMemoryService) Ingest(ctx context.Context, doc DocumentInput) (IngestResult, error)
```

---

## 7. V1 Context Injection

Memory recall is injected into the LLM context during context assembly:

```
[Relevant knowledge from memory]
- Deploy key is stored in /etc/keys (fact, source: user)
- User prefers dark mode and verbose output (preference, source: user)
[End of memory recall]
```

Memory budget: `min(memory_max_tokens, history_budget × 0.25)`

---

## 8. V1 Document Chunking

Recursive character text splitter:
- `chunk_size`: 800 tokens (configurable)
- `chunk_overlap`: 120 tokens (15%)
- Separator hierarchy: `["\n\n", "\n", ". ", " ", ""]`

---

## 9. V1 Configuration

```toml
[memory]
enabled = true
db_path = "~/.silo/memory.db"
embedding_provider = "default"
embedding_model = "text-embedding-3-small"
embedding_dimensions = 1536
chunk_size = 800
chunk_overlap = 120
search_top_k = 10
memory_max_tokens = 2048
```

---

## 10. V1 CLI Commands

```
silo memory
├── search <query>       # Hybrid search
├── add <content>        # Store a fact or preference
├── list                 # List entries with filters
├── delete <entry_id>    # Delete a single entry
├── ingest <file>        # Ingest document (chunk + embed + store)
└── clear                # Delete all entries (with confirmation)
```

---

## 11. V1 API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/silo/vault/store` | Store a knowledge entry |
| POST | `/silo/vault/query` | Hybrid search |
| DELETE | `/silo/vault/memory/{entry_id}` | Delete entry |
| GET | `/silo/vault/memory` | List entries |
| POST | `/silo/vault/memory/ingest` | Ingest document |

---

## 12. Cross-References

| Spec | Interaction |
|------|-------------|
| [06_sessions.md](06_sessions.md) | Sessions are per-conversation; memory is cross-session |
| [11_agent.md](11_agent.md) | V1: Memory recall injected into agent context |
| [14_adk_integration.md](14_adk_integration.md) | V0 uses InMemoryMemoryService; V1 implements custom MemoryService |
