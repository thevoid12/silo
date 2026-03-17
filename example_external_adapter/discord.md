# Discord Adapter for Silo

Send commands to Silo via Discord. The bot relays your messages to the Silo gateway,
streams the response back, and handles tool approval requests inline.

---

## Prerequisites

- Silo binary built (`go build -o silo .`)
- Python 3.9+ installed
- `uv` installed (`curl -LsSf https://astral.sh/uv/install.sh | sh`)
- A Discord account

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

## Step 3 — Create a Discord Bot

1. Go to [discord.com/developers/applications](https://discord.com/developers/applications)
2. Click **New Application**, give it a name (e.g. `Silo`)
3. Go to **Bot** in the left sidebar
4. Click **Reset Token** and copy the token
5. Under **Privileged Gateway Intents**, enable **Message Content Intent**
6. Go to **OAuth2 → URL Generator**:
   - Scopes: `bot`
   - Bot Permissions: `Send Messages`, `Read Messages/View Channels`, `Read Message History`
7. Copy the generated URL, open it in your browser, and add the bot to your server

---

## Step 4 — Run the Adapter

Set your tokens as environment variables and run:

```bash
export DISCORD_BOT_TOKEN="your-discord-bot-token"
export SILO_TOKEN="<your-gateway-token-from-step-1>"
export SILO_URL="http://127.0.0.1:5110"   # default, change if needed

uv run example_external_adapter/discord_adapter.py
```

You should see:

```
Silo Discord adapter connected as YourBot#1234
```

---

## Step 5 — Talk to Silo via Discord

Send a message in any channel your bot has access to:

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
Tool approval required
Tool: shell
Command: rm old_file.txt

Reply yes or no.
```

Reply in Discord:
```
yes
```
or
```
no
```

Natural language works too — `go ahead`, `allow it`, `deny`, etc.

---

## Session Continuity

Each Discord channel maps to one persistent Silo session. Silo remembers the conversation
context across messages in the same channel. To start fresh:

```
!reset
```

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
| Bot not responding | Check `DISCORD_BOT_TOKEN` and `SILO_TOKEN` env vars are set |
| `401 Unauthorized` from Silo | Wrong `SILO_TOKEN` — re-run `./silo vault get gateway-token` |
| `Missing Message Content Intent` | Enable it in Discord Developer Portal → Bot → Privileged Gateway Intents |
| Tool approval timeout | Increase `tools.approval.timeout` in config (default 120s) |
| Bot only responds to DMs | Ensure the bot has `Read Messages` permission in the target channel |
