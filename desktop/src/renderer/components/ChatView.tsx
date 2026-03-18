import { useEffect, useRef, useState, FormEvent } from 'react'
import { MessageBubble } from './MessageBubble'
import { ApprovalModal } from './ApprovalModal'
import { useSSE } from '../hooks/useSSE'
import type { ApiClient } from '../lib/api'

interface Props {
  client: ApiClient
  sessionId: string | null
  onSessionChange: (id: string) => void
}

export function ChatView({ client, sessionId, onSessionChange }: Props) {
  const { messages, streaming, pendingApproval, sessionId: activeSessionId, send, resolveApproval } = useSSE(client, sessionId)
  const [input, setInput] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (activeSessionId) onSessionChange(activeSessionId)
  }, [activeSessionId, onSessionChange])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const text = input.trim()
    if (!text || streaming) return
    setInput('')
    if (textareaRef.current) textareaRef.current.style.height = 'auto'
    send(text)
  }

  function handleTextareaInput(e: React.ChangeEvent<HTMLTextAreaElement>) {
    setInput(e.target.value)
    const el = e.target
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`
  }

  return (
    <div style={s.root}>
      <div style={s.messages}>
        {messages.length === 0 && (
          <div style={s.empty}>
            <p style={s.emptyText}>What can I help with?</p>
          </div>
        )}
        {messages.map(m => <MessageBubble key={m.id} message={m} />)}
        <div ref={bottomRef} />
      </div>

      <form onSubmit={handleSubmit} style={s.composer}>
        <div style={s.composerInner}>
          <textarea
            ref={textareaRef}
            style={s.textarea}
            placeholder="Message silo…"
            value={input}
            onChange={handleTextareaInput}
            onKeyDown={e => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                handleSubmit(e as unknown as FormEvent)
              }
            }}
            rows={1}
            disabled={streaming}
          />
          <button type="submit" disabled={streaming || !input.trim()} style={s.sendBtn}>
            ↑
          </button>
        </div>
      </form>

      {pendingApproval && (
        <ApprovalModal
          approval={pendingApproval}
          onDecide={approved => resolveApproval(pendingApproval.request_id, approved)}
        />
      )}
    </div>
  )
}

const s: Record<string, React.CSSProperties> = {
  root: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
  messages: {
    flex: 1,
    overflowY: 'auto',
    padding: '1.5rem 2rem',
    display: 'flex',
    flexDirection: 'column',
  },
  empty: {
    flex: 1,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  emptyText: {
    fontSize: '1.125rem',
    fontWeight: 500,
    color: 'var(--muted)',
    margin: 0,
  },
  composer: {
    padding: '0.875rem 1.5rem 1.25rem',
    borderTop: '1px solid var(--border)',
    background: 'rgba(255,255,255,0.92)',
    backdropFilter: 'blur(8px)',
  },
  composerInner: {
    display: 'flex',
    alignItems: 'flex-end',
    gap: '0.625rem',
    background: 'var(--card)',
    border: '1px solid var(--border)',
    borderRadius: '1rem',
    padding: '0.5rem 0.5rem 0.5rem 1rem',
    boxShadow: 'var(--shadow-sm)',
  },
  textarea: {
    flex: 1,
    resize: 'none',
    border: 'none',
    outline: 'none',
    fontFamily: 'var(--font-sans)',
    fontSize: '0.9375rem',
    lineHeight: 1.5,
    color: 'var(--ink)',
    background: 'transparent',
    maxHeight: '160px',
    overflowY: 'auto',
  },
  sendBtn: {
    width: 34,
    height: 34,
    borderRadius: '9999px',
    border: 'none',
    background: 'var(--accent)',
    color: '#fff',
    fontSize: '1rem',
    cursor: 'pointer',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
    transition: 'background var(--transition), transform var(--transition)',
  },
}
