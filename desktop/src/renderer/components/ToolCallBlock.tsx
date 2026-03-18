import { useState } from 'react'
import type { ToolCallEntry } from '@shared/types'

interface Props {
  toolCall: ToolCallEntry
}

export function ToolCallBlock({ toolCall }: Props) {
  const [expanded, setExpanded] = useState(false)
  const args = toolCall.args ?? {}
  const cmd = typeof args.command === 'string' ? args.command : JSON.stringify(args)

  return (
    <div style={s.block}>
      <button style={s.header} onClick={() => setExpanded(e => !e)}>
        <span style={{ ...s.indicator, color: toolCall.pending ? '#f59e0b' : '#10b981' }}>
          {toolCall.pending ? '◌' : '✓'}
        </span>
        <span style={s.badge}>{toolCall.tool}</span>
        <code style={s.cmd}>{cmd}</code>
        <span style={s.chevron}>{expanded ? '▲' : '▼'}</span>
      </button>
      {expanded && toolCall.result && (
        <pre style={s.output}>{formatOutput(toolCall.result)}</pre>
      )}
    </div>
  )
}

function formatOutput(output: Record<string, unknown>): string {
  if (typeof output.stdout === 'string' && output.stdout) return output.stdout
  if (typeof output.stderr === 'string' && output.stderr) return output.stderr
  return JSON.stringify(output, null, 2)
}

const s: Record<string, React.CSSProperties> = {
  block: {
    background: 'rgba(1,22,39,0.03)',
    border: '1px solid var(--border)',
    borderRadius: '0.625rem',
    marginBottom: '0.5rem',
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.5rem',
    width: '100%',
    padding: '0.5rem 0.75rem',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    textAlign: 'left',
    fontFamily: 'inherit',
    transition: 'background var(--transition)',
  },
  indicator: {
    fontSize: '0.75rem',
    flexShrink: 0,
  },
  badge: {
    fontSize: '0.6875rem',
    fontWeight: 500,
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
    color: 'var(--muted)',
    background: 'rgba(1,22,39,0.06)',
    borderRadius: '9999px',
    padding: '1px 8px',
    flexShrink: 0,
  },
  cmd: {
    fontFamily: 'var(--font-mono)',
    fontSize: '0.8125rem',
    color: 'var(--ink)',
    flex: 1,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  chevron: {
    fontSize: '0.625rem',
    color: 'var(--muted)',
    flexShrink: 0,
  },
  output: {
    margin: 0,
    padding: '0.625rem 0.75rem',
    fontFamily: 'var(--font-mono)',
    fontSize: '0.8125rem',
    lineHeight: 1.5,
    color: 'var(--ink)',
    background: 'rgba(1,22,39,0.05)',
    borderTop: '1px solid var(--border)',
    overflowX: 'auto',
    maxHeight: '200px',
    overflowY: 'auto',
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-all',
  },
}
