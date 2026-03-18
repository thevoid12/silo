import { useState, useCallback, useRef } from 'react'
import type {
  Message,
  ToolCallEntry,
  ApprovalRequiredPayload,
  TokenPayload,
  ToolCallPayload,
  ToolResultPayload,
  DonePayload,
} from '@shared/types'
import type { ApiClient } from '../lib/api'

export function useSSE(client: ApiClient, initialSessionId: string | null) {
  const [messages, setMessages] = useState<Message[]>([])
  const [streaming, setStreaming] = useState(false)
  const [pendingApproval, setPendingApproval] = useState<ApprovalRequiredPayload | null>(null)
  const [sessionId, setSessionId] = useState(initialSessionId)
  const streamingRef = useRef(false)

  const send = useCallback(async (text: string) => {
    if (streamingRef.current) return
    streamingRef.current = true
    setStreaming(true)

    const assistantId = crypto.randomUUID()
    setMessages(prev => [
      ...prev,
      { id: crypto.randomUUID(), role: 'user', text, toolCalls: [], streaming: false },
      { id: assistantId, role: 'assistant', text: '', toolCalls: [], streaming: true },
    ])

    const update = (fn: (m: Message) => Message) =>
      setMessages(prev => prev.map(m => (m.id === assistantId ? fn(m) : m)))

    const finish = () => {
      streamingRef.current = false
      setStreaming(false)
    }

    try {
      const res = await client.chatStream(text, sessionId)
      if (!res.ok || !res.body) throw new Error('chat request failed')

      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let buf = ''

      outer: while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buf += decoder.decode(value, { stream: true })
        const blocks = buf.split('\n\n')
        buf = blocks.pop() ?? ''

        for (const block of blocks) {
          let event = ''
          let data = ''
          for (const line of block.split('\n')) {
            if (line.startsWith('event: ')) event = line.slice(7).trim()
            else if (line.startsWith('data: ')) data = line.slice(6)
          }
          if (!event || !data) continue

          const payload = JSON.parse(data)

          if (event === 'token') {
            update(m => ({ ...m, text: m.text + (payload as TokenPayload).text }))
          } else if (event === 'tool_call') {
            const p = payload as ToolCallPayload
            const entry: ToolCallEntry = { id: crypto.randomUUID(), tool: p.tool, args: p.args, pending: true }
            update(m => ({ ...m, toolCalls: [...m.toolCalls, entry] }))
          } else if (event === 'tool_result') {
            const p = payload as ToolResultPayload
            update(m => ({
              ...m,
              toolCalls: m.toolCalls.map(tc =>
                tc.tool === p.tool && tc.pending ? { ...tc, result: p.output, pending: false } : tc
              ),
            }))
          } else if (event === 'approval_required') {
            setPendingApproval(payload as ApprovalRequiredPayload)
          } else if (event === 'done') {
            setSessionId((payload as DonePayload).session_id)
            update(m => ({ ...m, streaming: false }))
            finish()
            break outer
          } else if (event === 'error') {
            update(m => ({ ...m, streaming: false, error: payload.message }))
            finish()
            break outer
          }
        }
      }
    } catch (err) {
      update(m => ({ ...m, streaming: false, error: String(err) }))
      finish()
    }
  }, [client, sessionId])

  const resolveApproval = useCallback(async (requestId: string, approved: boolean) => {
    setPendingApproval(null)
    await client.resolveApproval(requestId, approved)
  }, [client])

  return { messages, streaming, pendingApproval, sessionId, send, resolveApproval }
}
