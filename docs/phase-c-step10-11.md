# Phase C — Steps 10 & 11: Chat SSE + Tool Approval Endpoints

## What's New

### POST /silo/brain/chat  (Step 10)

Streams the agent's response as Server-Sent Events.

**Request**
```json
{ "message": "List files in the workspace", "session_id": "optional-uuid" }
```

**SSE event stream**

| Event | Data |
|-------|------|
| `token` | `{"text": "..."}` — streaming text chunk |
| `tool_call` | `{"tool": "shell", "args": {"command": "ls"}}` |
| `tool_result` | `{"tool": "shell", "output": {...}}` |
| `approval_required` | `{"request_id": "...", "tool": "shell", "command": "ls"}` |
| `done` | `{"session_id": "..."}` — stream finished; reuse session_id for follow-ups |
| `error` | `{"message": "..."}` |

- If `session_id` is omitted a new session is created and returned in the `done` event.
- When `approval_required` fires the stream pauses until a response is posted to `/silo/brain/tool-approval`.

### POST /silo/brain/tool-approval  (Step 11)

Resolves a pending tool call.

**Request**
```json
{ "request_id": "...", "approved": true }
```

- `200 OK` on success — the paused SSE stream resumes.
- `404` if the request_id is unknown or already expired.

---

## Key design decisions

- **Per-request runner**: `RunnerFactory` creates a fresh ADK runner for each chat request so tool state is isolated.
- **Shared session service**: Session history persists across requests; pass `session_id` to continue a conversation.
- **Shared approval service**: A single `ApprovalService` per server. The tool-approval endpoint calls `Respond(id, approved)` regardless of which session owns the request.
- **Goroutine safety**: the runner goroutine closes `runnerDone` via `defer` so the approval goroutine always exits cleanly. All sends to `eventCh` include `ctx.Done()` and `runnerDone` escape hatches to prevent blocking on a slow client.
- **WriteTimeout = 0**: The `http.Server` write timeout is disabled to allow long-lived SSE streams.

---

## Quick verification

```bash
./silo start
# -> prompts vault password, starts server

# stream a chat
curl -N \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"message": "What files are here?"}' \
  http://localhost:5110/silo/brain/chat
# -> event: token ...  event: done {"session_id":"..."}

# approve a tool call
curl -X POST \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"request_id": "<id>", "approved": true}' \
  http://localhost:5110/silo/brain/tool-approval
# -> 200 OK, stream resumes
```
