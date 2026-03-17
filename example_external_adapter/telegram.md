# Telegram Adapter for Silo

Send commands to Silo via Telegram. The bot relays your messages to the Silo gateway,
streams the response back, and handles tool approval requests inline.

---

## Prerequisites

- Silo binary built (`go build -o silo .`)
- Python 3.9+ installed
- `uv` installed (`curl -LsSf https://astral.sh/uv/install.sh | sh`)
- A Telegram account

---

## Step 1 — Initialize Silo

If you have not initialized Silo yet:

```bash
./silo init
```

Follow the interactive prompts:
- Set a vault password (min 8 chars)
- Choose provider: `gemini`, `openai`, or `anthropic`
- Enter your API key for that provider

At the end you will see:

```
Gateway token: <your-token>
```

**Copy that token. You will need it.**

If you lost it:

```bash
./silo vault get gateway-token
```

---

## Step 2 — Start Silo

```bash
./silo start
```

Enter your vault password when prompted. You should see:

```
silo server started (PID xxxxx) listening on 127.0.0.1:5110
```

Verify it is running:

```bash
curl http://localhost:5110/health
# {"status":"ok"}
```

---

## Step 3 — Create a Telegram Bot

1. Open Telegram, search for **@BotFather**
2. Send `/newbot`
3. Choose a name (e.g. `My Silo Bot`)
4. Choose a username ending in `bot` (e.g. `my_silo_bot`)
5. BotFather replies with your **bot token**:
   ```
   Use this token to access the HTTP API:
   7123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw
   ```

Keep this token — it goes in the adapter config below.

---

## Step 4 — Run the Adapter

Save the following as `example_external_adapter/telegram_adapter.py` (already present in the repo).

Set your two tokens as environment variables and run:

```bash
export TELEGRAM_BOT_TOKEN="7123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"
export SILO_TOKEN="<your-gateway-token-from-step-1>"
export SILO_URL="http://127.0.0.1:5110"   # default, change if needed


uv run example_external_adapter/telegram_adapter.py
```

---

## Step 5 — Talk to Silo via Telegram

Open Telegram and send your bot a message:

```
List files in my home directory
```

The bot will:
1. Forward the message to Silo
2. Stream back the response as it arrives
3. Notify you if a tool needs approval (see below)

---

## Tool Approval Flow

When Silo needs to run a command that requires approval, the bot sends:

```
⚠️ Tool approval required
Tool: shell
Command: rm old_file.txt

Reply /approve <request_id> or /deny <request_id>
```

Reply in Telegram:
```
/approve abc123
```
or
```
/deny abc123
```

---

## Session Continuity

Each Telegram chat (by chat ID) maps to one persistent Silo session.
Silo remembers the conversation context across messages in the same chat.
To start fresh, send `/reset` to the bot.

---

## Stopping Silo

```bash
./silo stop
```

---

## Troubleshooting

| Problem | Fix |
|---|---|
| `curl: Failed to connect` | `silo start` was not run or crashed — check `./silo status` |
| Bot not responding | Check `TELEGRAM_BOT_TOKEN` and `SILO_TOKEN` env vars are set |
| `401 Unauthorized` from Silo | Wrong `SILO_TOKEN` — re-run `./silo vault get gateway-token` |
| Agent gives wrong API key error | Re-run `./silo init` after deleting `~/.silo/vault.enc` |
| Tool approval timeout | Default 30s — approve faster or increase `tools.approval.timeout` in `~/.silo/silo.toml` |
