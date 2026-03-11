# Channel & Adapter Specification (V0)

Channels are how users and external programs talk to Silo. They sit on the upstream side — between the client and the core. Silo does **not** build integrations. Instead, it exposes a clean API and lets anyone build adapters in any language.

---

## 1. Philosophy

- A channel is any external program that speaks HTTP + SSE to Silo's gateway (headless mode), or a frontend that communicates via IPC (desktop mode).
- **Silo does NOT build integrations** — no Telegram bot, no Slack app, no WhatsApp connector lives inside Silo. You can build whatever you like and our ultimate aim is to provide the customizable interface.
- **Three built-in channels:**
  - **Desktop app** (Electron) — the primary consumer experience, ships with the distribution
  - **CLI** (`silo chat`) — readline interactive loop, SSH/terminal fallback
  - **Headless gateway** — HTTP+SSE server for external adapters
- Anyone can build an adapter in any language — Python, Go, Node, whatever. Just speak HTTP to the gateway.

---

## 2. Inbuilt CLI (`silo chat`)

The simplest built-in channel. Same binary, subcommand launch.

### Basics

- **Launch:** `silo chat` starts the readline loop.
- **Library:** Go `bufio.Scanner` or `github.com/chzyer/readline` for line editing.
- **Connection:** In-process — calls Go core functions directly. No HTTP hop.
- **API:** Uses the same ADK agent loop as desktop and headless modes. Events are rendered to stdout.

### V0 Scope (Minimal, Functional)

| Feature                          | Description                                              |
|----------------------------------|----------------------------------------------------------|
| Text input prompt                | User types messages, sends to ADK agent                  |
| Streaming response display       | Token-by-token rendering to stdout as events arrive      |
| Tool call visibility             | Shows what tools the agent wants to run (name + args)    |
| Human-in-the-loop tool approval  | `[y/n]` prompt on stdin before execution                 |
| Current session only             | No conversation history persistence — fresh on each launch |

### V1 Scope (Deferred)

- Conversation history and session persistence
- Provider switcher (change model mid-conversation)
- Rich markdown rendering
- File attachment support
- Session picker (`--session <id>`, `--list`)

---

## 3. Desktop App (Electron)

The primary consumer experience. See [13_desktop_app.md](13_desktop_app.md) for the full specification.

### Summary

- Electron shell communicates with Go backend via IPC (local HTTP on random port)
- Chat view with streaming markdown rendering
- Tool approval modal (approve/deny buttons)
- Settings view (provider config, vault management)
- Session sidebar (list of past conversations)
- Launched via `silo start` (or double-click the desktop app)

### Desktop as a Channel

The desktop app is a "first-class adapter" shipped in the same distribution. It follows the same adapter contract as external adapters (§5) but communicates via IPC instead of the headless gateway:

```
Go Core  ←── IPC (local HTTP + SSE) ──→  Electron Frontend
```

Tool approval in desktop mode: Electron shows a modal dialog → user clicks approve/deny → IPC message back to Go core.

---

## 4. Tool Approval Protocol (V0)

This is a key design piece. The approval protocol works across all three modes (CLI, Desktop, Headless).

### Flow

```
1. Client sends message (via CLI stdin / Desktop IPC / HTTP POST)

2. ADK agent starts processing, decides to call a tool

3. BeforeToolCallback checks tool's approval config (from silo.toml)

4. If approval required:
   Agent PAUSES execution
   Event emitted: tool_pending
   data: { "call_id": "abc123", "tool": "bash", "args": { "cmd": "rm -rf /tmp/old" } }

5. Client displays tool call to user, asks approve/deny:
   - CLI: [y/n] prompt on stdin
   - Desktop: modal dialog via IPC
   - Headless: SSE event, adapter POSTs approval

6. Approval response sent back:
   - CLI: synchronous channel send
   - Desktop: IPC message → Go channel
   - Headless: POST /silo/brain/tool-approval → Go channel

7. Agent resumes:
   - If approved → executes tool → emits: tool_result
   - If denied  → skips tool   → emits: tool_denied

8. Agent continues (may call more tools or produce final response)
```

### Go Implementation

```go
type PendingApproval struct {
    CallID  string
    Tool    string
    Args    map[string]interface{}
    Result  chan bool        // approval response channel
}

// Stored in sync.Map keyed by call_id
var pendingApprovals sync.Map
```

Approval timeout uses `context.WithTimeout` or `time.After`:

```go
select {
case approved := <-pending.Result:
    if approved {
        // execute tool
    } else {
        // emit tool_denied
    }
case <-time.After(timeout):
    // auto-deny on timeout
}
```

### Gateway Endpoint (Headless Mode)

| Method | Path                          | Description                          |
|--------|-------------------------------|--------------------------------------|
| POST   | `/silo/brain/tool-approval`   | Approve or deny a pending tool call  |

**Request:**
```json
{
  "call_id": "abc123",
  "approved": true
}
```

**Response:**
```json
{
  "status": "accepted"
}
```

If `call_id` is unknown or expired:
```json
{
  "type": "silo/invalid-call-id",
  "title": "Invalid Call ID",
  "status": 404,
  "detail": "No pending tool call with id 'abc123'"
}
```

### SSE Event Types

| Event Type          | Payload Description                                        |
|---------------------|------------------------------------------------------------|
| `event: tool_pending` | Tool awaiting human approval (includes `call_id`, `tool`, `args`) |
| `event: tool_denied`  | Tool was denied by user (includes `call_id`, `tool`)        |

### Tool Approval Config in `silo.toml`

```toml
[tools.approval]
mode = "per-tool"       # "all", "none", or "per-tool"
timeout = 30            # seconds before auto-deny (0 = wait forever)

[tools.approval.rules]
bash = "always"         # always ask before running shell commands
read_file = "never"     # auto-approve file reads
write_file = "always"   # always ask before writing files
web = "always"          # always ask before web requests
```

**Modes:**
- `"all"` — every tool call requires approval
- `"none"` — all tools auto-approved (headless/trusted mode)
- `"per-tool"` — each tool has its own rule (`"always"` or `"never"`)

---

## 5. Adapter Contract (Headless Mode)

What external channel developers need to know to build a Silo adapter.

### Authentication

Every request must include:
```
Authorization: Bearer <token>
```
Get the token from the vault (stored during `silo init`).

**Important:** This authenticates the *adapter* to Silo, not the end user to the adapter. The Bearer token proves the adapter is authorized to call Silo's API. Adapters are responsible for their own user authentication — e.g., DM pairing codes, allowlists, OAuth.

### Start a Chat

```
POST /silo/brain/chat
Content-Type: application/json
Authorization: Bearer <token>

{
  "message": "List all files in the current directory",
  "session_id": "tg-12345-67890"             // optional (channel-chat-sender)
}
```

Response: SSE stream (`text/event-stream`).

**Fields:**

| Field        | Required | Description |
|--------------|----------|-------------|
| `message`    | Yes      | The user's message text |
| `session_id` | No       | Opaque string that groups messages into a conversation. If omitted, a new session is created. **Multi-user channels must include the sender in the ID** to prevent session collisions: use `{chat_id}-{sender_id}` (e.g., `tg-12345-67890`, `slack-C099-U042-1700000000`). See [06_sessions.md](06_sessions.md) for session storage semantics. |

### Handle SSE Events

Adapters must parse these event types from the stream:

| Event            | When                          | Action                              |
|------------------|-------------------------------|-------------------------------------|
| `event: token`   | LLM producing output          | Display/buffer the text chunk       |
| `event: tool_call` | Agent invoking a tool       | Display tool name + args (informational) |
| `event: tool_pending` | Tool awaiting approval   | Show approval prompt to user        |
| `event: tool_result`  | Tool finished             | Display result                      |
| `event: tool_denied`  | Tool was denied           | Display denial notice               |
| `event: error`   | Something failed              | Display error to user               |
| `event: done`    | Stream complete               | Close connection, show final state  |

### Tool Approval (Optional)

If the adapter wants human-in-the-loop:
1. Watch for `event: tool_pending` events
2. Present the tool call to the user
3. POST approval/denial to `/silo/brain/tool-approval`

**If not implemented:** the agent auto-denies after the configured timeout (default 30s).

### Streaming vs. Batch Adapters

| Pattern | Examples | How to Handle `event: token` | When to Send |
|---------|----------|------------------------------|--------------|
| **Streaming** | CLI, Desktop, web UI | Render each token immediately | Continuously |
| **Batch** | Telegram, Slack, email, SMS | Append each token to a buffer | Send compiled message on `event: done` |

**Batch adapter rules of thumb:**

1. **Buffer tokens** — accumulate `event: token` data until `event: done`.
2. **Send once** — on `done`, compile the buffer and deliver as a single message.
3. **Optional "typing" effect** — channels that support message editing can periodically flush the buffer mid-stream. Rate-limit edits (every 1–2 seconds).
4. **Don't drop non-token events** — `tool_pending`, `tool_result`, and `error` events should still be surfaced.

### Simple Integration Alternative

For adapters that don't need rich events:

```
POST /v1/chat/completions
```

Use the OpenAI-compatible endpoint (V1). Standard request/response format, no Silo-specific events.

---

## 6. Auto-Approve Timeout

When a tool requires approval but no response arrives in time:

| Context          | Recommended Timeout | Behavior on Timeout |
|------------------|---------------------|---------------------|
| CLI (`silo chat`) | 30 seconds          | Auto-**deny** — user is staring at the terminal |
| Desktop app      | 60 seconds          | Auto-**deny** — user may have switched windows |
| Async messaging (Telegram, Slack) | 120–300 seconds | Auto-**deny** — user may be away from device |
| Headless / API   | 0 (immediate)       | No approval needed — headless callers ARE the decision makers |

- Configurable in `silo.toml`: `[tools.approval] timeout = 30`
- Setting `timeout = 0` with `mode = "none"` is the headless configuration — all tools auto-approved.
- The agent never hangs indefinitely waiting for approval.

---

## 7. Reference Adapter Pattern

Minimal pseudocode showing how an external adapter talks to Silo. This could be Python, Node, Go — anything that speaks HTTP.

```python
import requests
import sseclient

SILO_URL = "http://127.0.0.1:5110"
TOKEN = "your-bearer-token"  # from vault

headers = {
    "Authorization": f"Bearer {TOKEN}",
    "Content-Type": "application/json",
}

# 1. Start a chat — opens SSE stream
response = requests.post(
    f"{SILO_URL}/silo/brain/chat",
    json={"message": "List files in /tmp"},
    headers=headers,
    stream=True,
)

# 2. Parse SSE events
client = sseclient.SSEClient(response)
for event in client.events():

    if event.event == "token":
        # LLM is producing text — display it
        print(event.data, end="", flush=True)

    elif event.event == "tool_pending":
        # Agent wants to run a tool — ask the user
        import json
        data = json.loads(event.data)
        print(f"\nTool: {data['tool']} Args: {data['args']}")
        choice = input("Approve? [y/n]: ")

        # 3. Send approval back
        requests.post(
            f"{SILO_URL}/silo/brain/tool-approval",
            json={
                "call_id": data["call_id"],
                "approved": choice.lower() == "y",
            },
            headers=headers,
        )

    elif event.event == "tool_result":
        print(f"\n[Tool result]: {event.data}")

    elif event.event == "tool_denied":
        print(f"\n[Tool denied]: {event.data}")

    elif event.event == "error":
        print(f"\n[Error]: {event.data}")

    elif event.event == "done":
        print("\n--- Done ---")
        break
```

### Async / Event-Driven Adapter

The sync example above blocks on `input()` — fine for a CLI, unusable for an event-driven channel. Below is pseudocode for an async adapter that handles any webhook-based platform (Telegram, Slack, Discord, etc.). The key difference: **reading SSE and handling user input are two decoupled tasks**.

The three `platform_*` stubs are the only code that changes between channels — all Silo integration logic is identical everywhere.

```python
import asyncio, aiohttp, json, os
from aiohttp import web

SILO_URL = "http://127.0.0.1:5110"
SILO_TOKEN = "your-bearer-token"

# In-flight state: keyed by Silo call_id
pending_approvals = {}   # call_id → { chat_id, platform_message_id }
response_buffers = {}    # session_id → [token_chunks]

# ── Platform stubs (implement per channel) ─────────────────────────
# These are the ONLY functions that change between Telegram, Slack,
# Discord, etc. Everything else is pure Silo integration logic.

async def platform_send(chat_id, text):
    """Send a message to the user on the platform."""
    raise NotImplementedError  # e.g., Telegram sendMessage, Slack chat.postMessage

async def platform_send_approval(chat_id, call_id, tool_name, tool_args):
    """Send an approval prompt (with Approve/Deny buttons) to the user.
    Returns the platform's message ID so it can be edited later."""
    raise NotImplementedError  # e.g., Telegram inline keyboard, Slack Block Kit

async def platform_edit_message(chat_id, message_id, text):
    """Edit an existing message (e.g., to replace approval buttons with a verdict)."""
    raise NotImplementedError  # e.g., Telegram editMessageText, Slack chat.update

# ── 1. Incoming user message → start a Silo chat ──────────────────

async def on_user_message(chat_id, sender_id, text):
    """Called when a message arrives from the platform webhook."""
    session_id = f"{chat_id}-{sender_id}"

    # Fire-and-forget: read SSE in background
    asyncio.create_task(
        stream_silo_response(session_id, chat_id, text)
    )

# ── 2. SSE reader — runs as a background task ─────────────────────

async def stream_silo_response(session_id, chat_id, message):
    response_buffers[session_id] = []

    async with aiohttp.ClientSession() as http:
        async with http.post(
            f"{SILO_URL}/silo/brain/chat",
            json={"message": message, "session_id": session_id},
            headers={"Authorization": f"Bearer {SILO_TOKEN}"},
        ) as resp:
            async for line in resp.content:
                line = line.decode().strip()
                if not line:
                    continue

                if line.startswith("event:"):
                    event_type = line.split(":", 1)[1].strip()
                elif line.startswith("data:"):
                    data_str = line.split(":", 1)[1].strip()

                    if event_type == "token":
                        response_buffers[session_id].append(data_str)

                    elif event_type == "tool_pending":
                        data = json.loads(data_str)
                        msg_id = await platform_send_approval(
                            chat_id, data["call_id"],
                            data["tool"], json.dumps(data["args"]))
                        pending_approvals[data["call_id"]] = {
                            "chat_id": chat_id,
                            "platform_message_id": msg_id,
                        }

                    elif event_type == "tool_result":
                        response_buffers[session_id].append(
                            f"\n[Tool result]: {data_str}\n")

                    elif event_type == "error":
                        await platform_send(chat_id, f"Error: {data_str}")

                    elif event_type == "done":
                        # Compile buffer → single message to user
                        full_text = "".join(response_buffers.pop(session_id, []))
                        if full_text.strip():
                            await platform_send(chat_id, full_text)

# ── 3. User tapped Approve / Deny ─────────────────────────────────

async def on_user_approval(call_id, approved):
    """Called when the user responds to an approval prompt."""
    async with aiohttp.ClientSession() as http:
        await http.post(
            f"{SILO_URL}/silo/brain/tool-approval",
            json={"call_id": call_id, "approved": approved},
            headers={"Authorization": f"Bearer {SILO_TOKEN}"},
        )

    # Edit the original approval message to show the decision
    info = pending_approvals.pop(call_id, {})
    if info:
        label = "Approved" if approved else "Denied"
        await platform_edit_message(
            info["chat_id"], info["platform_message_id"], label)
```

**Key patterns to note:**

1. **Decoupled read/write** — SSE reading is a background task; platform webhook handling is a separate code path.
2. **`call_id` mapping** — `pending_approvals` dict bridges the Silo SSE world and the platform callback world.
3. **Buffer → send on done** — tokens accumulate; a single compiled message is sent when the stream ends.
4. **`session_id` = `{chat_id}-{sender_id}`** — prevents group chat collisions.
5. **Three platform stubs** — `platform_send`, `platform_send_approval`, and `platform_edit_message` are the only code that changes between channels.

This is the complete contract. Any language, any platform — just HTTP + SSE.

---

## 8. Walkthrough Example: Telegram Adapter (End-to-End)

- Others directly allow you to run telegram out of the box tightly integrated. But we are not doing that. We are going to build a generic adapter that can be used for any channel.

This section walks through building a real Telegram adapter from scratch. It demonstrates everything in §5 (Adapter Contract) and §7 (Reference Adapter Pattern) with a concrete channel.

### What You're Building

```
┌──────────┐         ┌──────────────┐         ┌──────────┐
│ Telegram │  HTTP   │  Your Python │  HTTP    │   Silo   │
│  Servers │ ──────→ │   Adapter    │ ──────→  │ Gateway  │
│          │ ←────── │  (port 8080) │ ←─ SSE ─ │ (:5110)  │
└──────────┘         └──────────────┘         └──────────┘
```

Your adapter is a small web server that:
1. Receives webhooks from Telegram (user messages, button taps)
2. Forwards them to Silo's `/silo/brain/chat` endpoint
3. Reads the SSE stream back and sends results to Telegram

### Prerequisites

| What | How |
|------|-----|
| Silo running | `silo start --headless` |
| Silo Bearer token | From vault (generated during `silo init`) |
| Python 3.9+ | With `aiohttp` installed |
| A Telegram bot | Created via [@BotFather](https://t.me/BotFather) |
| Public URL for webhooks | `ngrok http 8080` for local dev |

### Step 1: Create a Telegram Bot

1. Open Telegram, search for **@BotFather**, start a chat.
2. Send `/newbot`. Follow the prompts — pick a name and username.
3. BotFather replies with your bot token: `110201543:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw`
4. Save this token.

### Step 2: Configure Silo for Async Approval

Telegram users aren't staring at a terminal. Increase the tool approval timeout:

```toml
[tools.approval]
mode = "per-tool"
timeout = 120          # 2 minutes — gives mobile users time to respond
```

### Step 3: Write the Adapter

This is the complete, working adapter. It implements the three `platform_*` stubs from §7 for Telegram, plus the webhook wiring.

```python
"""
Silo ↔ Telegram Adapter
Bridges Telegram Bot API to Silo's /silo/brain/chat SSE endpoint.

Usage:
    pip install aiohttp
    export SILO_TOKEN="<from vault>"
    export TG_BOT_TOKEN="<from BotFather>"
    python telegram_adapter.py
"""

import asyncio, aiohttp, json, os
from aiohttp import web

# ── Configuration ──────────────────────────────────────────────────

SILO_URL   = os.environ.get("SILO_URL", "http://127.0.0.1:5110")
SILO_TOKEN = os.environ["SILO_TOKEN"]
TG_TOKEN   = os.environ["TG_BOT_TOKEN"]
TG_API     = f"https://api.telegram.org/bot{TG_TOKEN}"

# ── In-flight state ───────────────────────────────────────────────

pending_approvals = {}   # call_id → { chat_id, message_id }
response_buffers  = {}   # session_id → [token_chunks]

# ── Platform stubs (Telegram implementation) ──────────────────────

async def platform_send(chat_id, text):
    async with aiohttp.ClientSession() as http:
        await http.post(f"{TG_API}/sendMessage", json={
            "chat_id": chat_id,
            "text": text[:4096],
            "parse_mode": "Markdown",
        })

async def platform_send_approval(chat_id, call_id, tool_name, tool_args):
    async with aiohttp.ClientSession() as http:
        resp = await http.post(f"{TG_API}/sendMessage", json={
            "chat_id": chat_id,
            "text": f"*Tool approval requested*\n`{tool_name}`\n```\n{tool_args}\n```",
            "parse_mode": "Markdown",
            "reply_markup": {"inline_keyboard": [[
                {"text": "Approve", "callback_data": f"approve:{call_id}"},
                {"text": "Deny",    "callback_data": f"deny:{call_id}"},
            ]]},
        })
        data = await resp.json()
        return data["result"]["message_id"]

async def platform_edit_message(chat_id, message_id, text):
    async with aiohttp.ClientSession() as http:
        await http.post(f"{TG_API}/editMessageText", json={
            "chat_id": chat_id,
            "message_id": message_id,
            "text": text,
        })

# ── 1. Incoming Telegram message → start Silo chat ───────────────

async def handle_webhook(request):
    update = await request.json()

    if "callback_query" in update:
        return await handle_callback(update["callback_query"])

    msg = update.get("message", {})
    text = msg.get("text")
    if not text:
        return web.Response(status=200)

    chat_id   = msg["chat"]["id"]
    sender_id = msg["from"]["id"]
    session_id = f"tg-{chat_id}-{sender_id}"

    asyncio.create_task(
        stream_silo_response(session_id, chat_id, text)
    )
    return web.Response(status=200)

# ── 2. SSE reader — runs as a background task ────────────────────

async def stream_silo_response(session_id, chat_id, message):
    response_buffers[session_id] = []

    async with aiohttp.ClientSession() as http:
        async with http.post(
            f"{SILO_URL}/silo/brain/chat",
            json={"message": message, "session_id": session_id},
            headers={"Authorization": f"Bearer {SILO_TOKEN}"},
        ) as resp:
            event_type = None
            async for line in resp.content:
                line = line.decode().strip()
                if not line:
                    continue

                if line.startswith("event:"):
                    event_type = line.split(":", 1)[1].strip()
                elif line.startswith("data:"):
                    data_str = line.split(":", 1)[1].strip()

                    if event_type == "token":
                        response_buffers[session_id].append(data_str)

                    elif event_type == "tool_pending":
                        data = json.loads(data_str)
                        msg_id = await platform_send_approval(
                            chat_id, data["call_id"],
                            data["tool"], json.dumps(data["args"], indent=2))
                        pending_approvals[data["call_id"]] = {
                            "chat_id": chat_id,
                            "message_id": msg_id,
                        }

                    elif event_type == "tool_result":
                        response_buffers[session_id].append(
                            f"\n[Tool result]: {data_str}\n")

                    elif event_type == "error":
                        await platform_send(chat_id, f"Error: {data_str}")

                    elif event_type == "done":
                        full_text = "".join(
                            response_buffers.pop(session_id, []))
                        if full_text.strip():
                            await platform_send(chat_id, full_text)

# ── 3. User tapped Approve / Deny ────────────────────────────────

async def handle_callback(cb):
    action, call_id = cb["data"].split(":", 1)
    approved = action == "approve"

    async with aiohttp.ClientSession() as http:
        await http.post(
            f"{SILO_URL}/silo/brain/tool-approval",
            json={"call_id": call_id, "approved": approved},
            headers={"Authorization": f"Bearer {SILO_TOKEN}"},
        )

    info = pending_approvals.pop(call_id, {})
    if info:
        label = "Approved" if approved else "Denied"
        await platform_edit_message(
            info["chat_id"], info["message_id"], f"Tool {label}")

    return web.Response(status=200)

# ── Entrypoint ────────────────────────────────────────────────────

app = web.Application()
app.router.add_post("/webhook", handle_webhook)

if __name__ == "__main__":
    web.run_app(app, port=8080)
```

### Step 4: Run and Test

```bash
# Start Silo in headless mode
silo start --headless

# In another terminal
export SILO_TOKEN="$(cat ~/.silo/token)"  # or from vault
export TG_BOT_TOKEN="110201543:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"
python telegram_adapter.py
```

### Adapting This for Other Platforms

The Silo integration logic is **identical** for every platform. To build a Slack, Discord, or WhatsApp adapter, you only change:

| What Changes | Telegram | Slack | Discord |
|-------------|----------|-------|---------|
| `platform_send()` | `sendMessage` | `chat.postMessage` | Channel message POST |
| `platform_send_approval()` | Inline keyboard | Block Kit with buttons | Components with buttons |
| `platform_edit_message()` | `editMessageText` | `chat.update` | Edit message PATCH |
| Webhook parsing | `update["message"]` | `event["event"]["text"]` | Interaction payload |
| Session ID | `tg-{chat}-{user}` | `slack-{channel}-{user}-{thread_ts}` | `dc-{channel}-{user}` |

Everything else — SSE reading, token buffering, `tool_pending` handling, `tool-approval` POST, `done` compilation — stays the same.

---

## 9. Cross-References

| Spec | Interaction |
|------|-------------|
| [03_gateway.md](03_gateway.md) | Gateway endpoints, SSE streaming, middleware |
| [07_security.md](07_security.md) | Tool approval integrates with security layers |
| [08_cli.md](08_cli.md) | CLI `silo chat` is a built-in channel |
| [11_agent.md](11_agent.md) | ADK callbacks implement tool approval |
| [13_desktop_app.md](13_desktop_app.md) | Desktop app is a first-class channel |
| [14_adk_integration.md](14_adk_integration.md) | BeforeToolCallback implements approval logic |
