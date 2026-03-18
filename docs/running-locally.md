# Running silo locally

---

## Option A — Electron desktop app (packaged)

The packaged app is self-contained. No CLI setup needed.

### Build

```bash
make desktop
```

Builds the Go binary, bundles it inside the Electron app, and produces an installer in `desktop/dist/`.

### Launch

Run the installer or launch the app from `desktop/dist/`. On first launch:

1. **Setup screen** — choose a vault password, pick Gemini or OpenAI, paste your API key → "Create vault"
2. **Chat** — the app spawns the silo server internally, auto-unlocks with the password you just set, and opens the chat view

On subsequent launches:

1. **Unlock screen** — enter your vault password → "Unlock"
2. **Chat** — ready to go

The app manages the silo process lifecycle — no terminal needed.

---

## Option B — Electron dev mode

Dev mode connects to an already-running silo server. Useful for iterating on the UI without rebuilding the binary.

### Step 1 — start the backend

```bash
make build
SILO_VAULT_PASSWORD=<your-password> ./silo start
# prints: token:<token>   ← copy this
```

If you haven't created a vault yet:

```bash
./silo vault init
./silo vault set GEMINI_API_KEY=<your-key>
```

### Step 2 — start Electron in dev mode

```bash
SILO_DEV_PORT=5110 SILO_DEV_TOKEN=<paste-token> make desktop-dev
```

The app skips the unlock screen and connects directly.

---

## Option C — headless server only

No desktop app, just the HTTP API. Useful for scripting or remote access.

```bash
make build
SILO_VAULT_PASSWORD=<your-password> ./silo start
```

### Test with curl

```bash
TOKEN=<paste-token>

# List sessions
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:5110/silo/vault/sessions | jq

# Chat (streaming)
curl -s -N \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message":"hello"}' \
  http://127.0.0.1:5110/silo/brain/chat
```

---

## Go tests

```bash
make test
# single package:
go test ./pkg/core/...
```

---

## Playwright tests (Electron UI)

Single command — builds everything, starts the server, runs all tests, stops the server:

```bash
make test-playwright
```

Vault password `thisisvoid` is hardcoded in `scripts/test-e2e.sh`. The vault must already exist with that password (run `silo vault init` once if it doesn't).

Tests live in `desktop/playwright_tests/`:
- `app.spec.ts` — app launches, unlock screen visible
- `chat.spec.ts` — shell renders, send button state, message send/receive, approval modal idle state

---

## Port / config reference

| Setting | Default | Override |
|---------|---------|----------|
| Server port | `5110` | `--port` flag or `SILO_DEV_PORT` env var |
| DB path | `~/.silo/silo.db` | `~/.silo/silo.toml` |

Project-wide config: `config/silo.toml`. User overrides: `~/.silo/silo.toml`.
