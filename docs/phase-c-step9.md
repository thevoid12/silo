# Phase C — Step 9: HTTP Gateway Server

## What's New

### `silo start`
Starts the gateway HTTP server in the background.

- Prompts for your vault password to load the gateway bearer token
- Spawns a detached background process and writes its PID to `~/.silo/silo.pid`
- Server listens on `127.0.0.1:5110` by default (configurable in `silo.toml`)

### `silo stop`
Sends SIGTERM to the running server and removes the PID file.

### `silo status`
Reports whether the server is running and on which address.

---

## Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | None | Liveness probe — always returns `{"status":"ok"}` |
| GET | `/silo/status` | Bearer token | Server info: uptime, version, host, port |

All protected routes require `Authorization: Bearer <token>` — the token is generated during `silo init` and stored in the vault under the key `gateway-token`.

---

## Configuration (`silo.toml`)

```toml
[gateway]
host = "127.0.0.1"
port = 5110
pid_file = "~/.silo/silo.pid"

[gateway.timeouts]
read  = "30s"
write = "60s"
idle  = "120s"
```

---

## Quick verification

```bash
./silo start
# -> prompts for vault password
# -> silo server started (PID 12345) listening on 127.0.0.1:5110

curl http://localhost:5110/health
# -> {"status":"ok"}

curl -H "Authorization: Bearer <token>" http://localhost:5110/silo/status
# -> {"status":"running","uptime":"5s","version":"0.0.0",...}

./silo status
# -> silo server: running (PID 12345) on 127.0.0.1:5110

./silo stop
# -> silo server stopped (PID 12345)
```
