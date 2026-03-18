# Steps 14 & 15 — Chat View + Tool Approval Modal

## What changed

### Chat view (Step 14)
The desktop app now has a fully functional chat interface. Users can:
- Send messages to the silo agent and see streamed responses in real time
- See tool calls the agent makes (with a collapsible output view)
- Switch between sessions using the sidebar
- Start a new session from the sidebar's `+` button

### Tool approval modal (Step 15)
When the agent requests to run a shell command, a modal appears asking the user to approve or deny it. The modal:
- Shows the tool name and the full command
- Auto-denies after 30 seconds if no action is taken
- Supports keyboard shortcuts: Enter to approve, Escape to deny

### Session sidebar
Sessions are listed with relative timestamps (e.g. "3m ago"). The active session is highlighted. Selecting a session reloads chat history.

## Backend additions
- New endpoint `GET /silo/vault/sessions` returns the list of sessions for the desktop client.

## Files added/changed
- `desktop/src/renderer/components/Shell.tsx` — wired sidebar + chat together
- `desktop/src/renderer/components/ChatView.tsx` — composer, message list, SSE integration
- `desktop/src/renderer/components/MessageBubble.tsx` — renders user and assistant messages
- `desktop/src/renderer/components/ToolCallBlock.tsx` — collapsible tool call display
- `desktop/src/renderer/components/ApprovalModal.tsx` — tool approval overlay
- `desktop/src/renderer/components/SessionSidebar.tsx` — session list
- `desktop/src/renderer/hooks/useSSE.ts` — SSE state machine (streaming, tool calls, approval)
- `desktop/src/renderer/hooks/useSessions.ts` — session list fetching
- `desktop/src/renderer/lib/api.ts` — typed API client
- `desktop/src/shared/types.ts` — SSE event types, Message, Session
- `pkg/gateway/sessions.go` — sessions list handler
- `desktop/tests/chat.spec.ts` — Playwright tests for chat and approval modal
