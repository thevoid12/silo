# Step 13 — Desktop App Scaffold

## What changed

Added the `desktop/` Electron app scaffold and small Go changes to support desktop mode.

## New: `desktop/` directory

The Electron app lives at `desktop/` inside the silo repo. It is a completely separate build — `go build` ignores it. Two executables come out of one repo:

| Command | Output |
|---|---|
| `make build` | `./silo` — the CLI + gateway binary |
| `make desktop` | `desktop/dist/` — the Electron installer with silo bundled inside |

## How the desktop app connects to silo

1. User launches the app → sees **Unlock Screen** (vault password prompt)
2. User enters vault password → Electron spawns `silo start --serve --desktop-mode --port {random_port}` with `SILO_VAULT_PASSWORD` in the environment
3. silo unlocks the vault, reads the gateway token and API key, prints `token:{token}` to stdout, then starts listening
4. Electron reads the token from stdout, polls `/health` until the server is ready
5. App transitions to the **Shell** (three-pane layout)

In dev mode, set `SILO_DEV_PORT` and `SILO_DEV_TOKEN` — Electron skips spawning and connects directly.

## Go changes

- `pkg/cli/start.go` — `runServe()` now supports:
  - `--port` flag overrides the config port
  - `--desktop-mode` forces bind to `127.0.0.1`, loads credentials from vault via `SILO_VAULT_PASSWORD`, prints `token:{token}` to stdout before the server starts

## Dev workflow

```bash
# Terminal 1 — run silo backend
go run . start --serve --port 5110

# Terminal 2 — run Electron UI
cd desktop
SILO_DEV_PORT=5110 SILO_DEV_TOKEN=$(./silo vault get gateway-token) npm run dev
```
