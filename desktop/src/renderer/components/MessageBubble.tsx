import ReactMarkdown from 'react-markdown'
import { ToolCallBlock } from './ToolCallBlock'
import type { Message } from '@shared/types'

interface Props {
  message: Message
}

export function MessageBubble({ message }: Props) {
  const isUser = message.role === 'user'

  return (
    <div style={{ ...s.wrap, justifyContent: isUser ? 'flex-end' : 'flex-start' }}>
      <div style={isUser ? s.userBubble : s.assistantBubble}>
        {isUser ? (
          <span style={s.userText}>{message.text}</span>
        ) : (
          <>
            {message.toolCalls.map(tc => <ToolCallBlock key={tc.id} toolCall={tc} />)}
            {message.text && (
              <div style={s.md}>
                <ReactMarkdown>{message.text}</ReactMarkdown>
              </div>
            )}
            {message.streaming && !message.text && message.toolCalls.length === 0 && (
              <span style={s.cursor}>▌</span>
            )}
            {message.error && <span style={s.error}>{message.error}</span>}
          </>
        )}
      </div>
    </div>
  )
}

const s: Record<string, React.CSSProperties> = {
  wrap: {
    display: 'flex',
    marginBottom: '1rem',
  },
  userBubble: {
    background: 'var(--ink)',
    color: '#fff',
    padding: '0.625rem 1rem',
    borderRadius: '1.25rem 1.25rem 0.25rem 1.25rem',
    maxWidth: '70%',
    fontSize: '0.9375rem',
    lineHeight: 1.5,
  },
  assistantBubble: {
    maxWidth: '100%',
    flex: 1,
    fontSize: '0.9375rem',
    lineHeight: 1.7,
    color: 'var(--ink)',
  },
  userText: {
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
  },
  md: {
    // prose-like reset for react-markdown output
  },
  cursor: {
    display: 'inline-block',
    animation: 'blink 1s step-end infinite',
    color: 'var(--muted)',
  },
  error: {
    color: '#c0392b',
    fontSize: '0.875rem',
  },
}
