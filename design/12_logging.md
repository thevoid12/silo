# Logging & Telemetry Specification (V0)

This spec defines Silo's unified operational logging system. V0 uses `go.uber.org/zap` for structured logging. Cost tracking and usage analytics are deferred to V1.

**Key boundary:** Operational logging (this spec) and security audit ([07_security.md §8](07_security.md)) are **separate concerns** — security events are logged via the same `zap` infrastructure but tagged with security-specific fields.

---

## 1. Overview & Architecture

| Concern | V0 Implementation | V1 Enhancement |
|---------|------------------|----------------|
| Operational Logging | `zap` → file + stdout | + OTEL exporter |
| Security Audit | `zap` with security tags | + Signed event trail |
| Cost Tracking | Not implemented | `usage.db` SQLite |

### Data Flow

```
Gateway (request_id) ──► Agent / Shell / Vault ──► zap logger
     │                                                    │
     v                                                    v
request_id propagation                           ~/.silo/logs/silo.log (V0)
via context.Context                              OTEL exporter (V1)
```

`zap` is the single instrumentation API. All subsystems use `logger.Info()`, `logger.Debug()`, etc.

---

## 2. Logging Stack

### Zap (`go.uber.org/zap`)

| Component | Purpose |
|-----------|---------|
| `go.uber.org/zap` | Structured logging API (high-performance) |
| `lumberjack` | Log rotation (file size + age) |
| `go-systemd/journal` | Journald output (V1) |

### Logger Setup

```go
func initLogging(config LoggingConfig, mode RunMode) *zap.Logger {
    level := parseLevel(config.Level)

    switch mode {
    case Foreground:
        // Colored text to stdout
        cfg := zap.NewDevelopmentConfig()
        cfg.Level = zap.NewAtomicLevelAt(level)
        cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
        logger, _ := cfg.Build()
        return logger

    case Daemon, Headless:
        // JSON to file with rotation
        writer := zapcore.AddSync(&lumberjack.Logger{
            Filename:   filepath.Join(config.LogDir, "silo.log"),
            MaxSize:    100, // MB
            MaxBackups: 7,
            MaxAge:     7,   // days
        })
        core := zapcore.NewCore(
            zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
            writer,
            level,
        )
        return zap.New(core)
    }

    return zap.NewNop()
}
```

### Mode Detection

| Mode | Detection | Destination | Format |
|------|-----------|-------------|--------|
| Foreground | `silo chat` or `silo start` without `--headless` | stdout | Colored text |
| Headless | `silo start --headless` | `~/.silo/logs/silo.log` | JSON-per-line (JSONL) |

---

## 3. Structured Log Format

### JSON-per-Line (JSONL)

Every log entry in daemon/headless mode is a single JSON object on one line:

```json
{
  "time": "2025-03-15T14:23:01.456789Z",
  "level": "INFO",
  "msg": "LLM call completed",
  "request_id": "req_a1b2c3d4e5f6a7b8",
  "session_id": "tg-12345-67890",
  "provider": "gemini",
  "model": "gemini-2.0-flash",
  "input_tokens": 1200,
  "output_tokens": 340,
  "latency_ms": 2100
}
```

### Text Format (Foreground)

```
2025-03-15T14:23:01.456Z  INFO  LLM call completed  request_id=req_a1b2  provider=gemini  model=gemini-2.0-flash  latency=2100ms
```

---

## 4. Request-Level Tracing & Correlation

### Request ID Generation

Every incoming HTTP request (headless mode) gets a `request_id`:

```go
func generateRequestID() string {
    id := uuid.New()
    return "req_" + id.String()[:16]
}
```

The `request_id` is:
1. Added to the `context.Context` for the request
2. Propagated to all downstream `zap` calls via `logger.With()`
3. Returned in the `X-Request-Id` HTTP response header

### Context Propagation

```go
// In middleware
ctx := context.WithValue(r.Context(), requestIDKey, requestID)
logger := zap.L().With(zap.String("request_id", requestID))
ctx = context.WithValue(ctx, loggerKey, logger)

// In any handler or service
logger := loggerFromContext(ctx)
logger.Info("tool executed", zap.String("tool", toolName), zap.Int64("duration_ms", duration))
```

---

## 5. Event Catalog

### Gateway Events

| Event | Level | Key Fields |
|-------|-------|------------|
| `request_received` | INFO | `method`, `path`, `request_id` |
| `request_completed` | INFO | `request_id`, `status`, `latency_ms` |
| `auth_rejected` | WARN | `request_id`, `reason` |

### Agent Events

| Event | Level | Key Fields |
|-------|-------|------------|
| `session_created` | INFO | `session_id` |
| `session_resumed` | INFO | `session_id`, `message_count` |
| `llm_call_start` | INFO | `request_id`, `provider`, `model` |
| `llm_call_complete` | INFO | `request_id`, `provider`, `model`, `input_tokens`, `output_tokens`, `latency_ms` |
| `llm_call_error` | ERROR | `request_id`, `provider`, `error` |
| `tool_requested` | INFO | `request_id`, `tool`, `call_id` |
| `tool_approved` | INFO | `request_id`, `tool`, `call_id` |
| `tool_denied` | INFO | `request_id`, `tool`, `call_id`, `reason` |
| `tool_executed` | INFO | `request_id`, `tool`, `call_id`, `duration_ms`, `success` |

### Vault Events

| Event | Level | Key Fields |
|-------|-------|------------|
| `vault_access` | INFO | `operation`, `key` |

### Lifecycle Events

| Event | Level | Key Fields |
|-------|-------|------------|
| `silo_started` | INFO | `version`, `mode`, `host`, `port` |
| `silo_stopping` | INFO | `reason` |
| `silo_stopped` | INFO | `uptime_secs` |

---

## 6. Log Levels

| Level | Semantics | Example |
|-------|-----------|---------|
| ERROR | Unrecoverable failures | LLM unreachable after retries, vault corruption |
| WARN | Degraded but recoverable | Rate limit hit, tool timeout |
| INFO | Normal operations | Request complete, tool executed, session created |
| DEBUG | Internal details | Context assembly, memory search, shell command |

### Default Level

**INFO** — captures all normal operations without noise.

### Override Hierarchy

```
CLI -v flag  >  SILO_LOGGING_LEVEL env  >  logging.level config  >  INFO default
```

| Source | Effect |
|--------|--------|
| Default | INFO |
| `logging.level = "debug"` | All at DEBUG |
| `silo chat -v` | DEBUG |
| `silo chat -vv` | Full DEBUG with all fields |

---

## 7. Configuration

```toml
[logging]
level = "info"                    # "debug", "info", "warn", "error"
format = "auto"                   # "auto", "json", "text"
log_dir = "~/.silo/logs"         # log file directory
```

---

## 8. V0/V1 Scoping Summary

### V0 Ships

| Feature | Section |
|---------|---------|
| `zap` structured logging (text to stdout, JSON to file) | §2 |
| Request correlation IDs via `context.Context` | §4 |
| `X-Request-Id` response header | §4 |
| Event catalog (~15 events) | §5 |
| Log levels with override hierarchy | §6 |
| `[logging]` config section | §7 |
| Log rotation via `lumberjack` | §2 |

### V1 Deferred

| Feature | Notes |
|---------|-------|
| Cost tracking (`usage.db`, `UsageTracker`, `silo usage` commands) | Logging sufficient for V0 |
| `silo logs tail/search/show/export` CLI commands | Read log files directly in V0 |
| `silo usage show/detail/export` CLI commands | No cost tracking in V0 |
| OTEL exporter | |
| ELK/Filebeat integration | JSONL format is already compatible |
| Distributed tracing (W3C Trace Context) | |
| Prometheus metrics endpoint | |
| Cost analytics API | |
| Web dashboard | |
| Journald integration (`go-systemd/journal`) | |
| Runtime log level change | Restart to change in V0 |

---

## 9. Cross-References

| Spec | Interaction |
|------|-------------|
| [03_gateway.md §7](03_gateway.md) | Request ID middleware, `X-Request-Id` header |
| [07_security.md §8](07_security.md) | Security events logged via same `zap` infrastructure |
| [08_cli.md](08_cli.md) | `-v` / `--quiet` flags, V1 `silo logs` commands |
| [09_configuration.md](09_configuration.md) | `[logging]` config keys |
| [11_agent.md](11_agent.md) | Agent events (llm_call, tool execution) |

---

## 10. Verification Walkthrough

### Single Chat Request — Log Trail

```
14:23:01.456  INFO  request_received       request_id=req_a1b2 method=POST path=/silo/brain/chat
14:23:01.458  INFO  session_resumed        request_id=req_a1b2 session_id=tg-12345 message_count=8
14:23:01.464  INFO  llm_call_start         request_id=req_a1b2 provider=gemini model=gemini-2.0-flash
14:23:03.556  INFO  llm_call_complete      request_id=req_a1b2 provider=gemini input_tokens=1200 output_tokens=340 latency_ms=2092
14:23:03.558  INFO  tool_requested         request_id=req_a1b2 tool=bash call_id=call_xyz
14:23:03.560  INFO  tool_approved          request_id=req_a1b2 tool=bash call_id=call_xyz
14:23:04.010  INFO  tool_executed          request_id=req_a1b2 tool=bash call_id=call_xyz duration_ms=450 success=true
14:23:07.414  INFO  request_completed      request_id=req_a1b2 status=200 latency_ms=5958
```

Every line shares `request_id=req_a1b2` for instant correlation.
