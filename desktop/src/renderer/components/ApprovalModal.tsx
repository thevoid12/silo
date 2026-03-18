import { useEffect, useState } from 'react'
import type { ApprovalRequiredPayload } from '@shared/types'

const AUTO_DENY_SECS = 30

interface Props {
  approval: ApprovalRequiredPayload
  onDecide: (approved: boolean) => void
}

export function ApprovalModal({ approval, onDecide }: Props) {
  const [remaining, setRemaining] = useState(AUTO_DENY_SECS)

  useEffect(() => {
    const t = setInterval(() => {
      setRemaining(r => {
        if (r <= 1) { clearInterval(t); onDecide(false); return 0 }
        return r - 1
      })
    }, 1000)
    return () => clearInterval(t)
  }, [onDecide])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Enter') onDecide(true)
      if (e.key === 'Escape') onDecide(false)
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onDecide])

  const cmd = approval.command || [approval.tool, ...(approval.args ?? [])].join(' ')

  return (
    <div style={s.overlay}>
      <div style={s.modal}>
        <h3 style={s.title}>Tool Approval Required</h3>
        <div style={s.meta}>
          <span style={s.badge}>{approval.tool}</span>
        </div>
        <pre style={s.command}>{cmd}</pre>
        <p style={s.timer}>Auto-deny in {remaining}s</p>
        <div style={s.actions}>
          <button style={s.denyBtn} onClick={() => onDecide(false)}>Deny</button>
          <button style={s.approveBtn} onClick={() => onDecide(true)}>Approve</button>
        </div>
        <p style={s.hint}>Enter to approve · Escape to deny</p>
      </div>
    </div>
  )
}

const s: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(1,22,39,0.25)',
    backdropFilter: 'blur(4px)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 100,
  },
  modal: {
    background: 'var(--card-strong)',
    border: '1px solid var(--border)',
    borderRadius: '1.25rem',
    boxShadow: 'var(--shadow)',
    padding: '1.75rem',
    width: 400,
    display: 'flex',
    flexDirection: 'column',
    gap: '1rem',
    backdropFilter: 'blur(12px)',
  },
  title: {
    margin: 0,
    fontSize: '1rem',
    fontWeight: 600,
    color: 'var(--ink)',
  },
  meta: {
    display: 'flex',
    gap: '0.5rem',
  },
  badge: {
    fontSize: '0.75rem',
    fontWeight: 500,
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
    color: 'var(--muted)',
    background: 'rgba(1,22,39,0.06)',
    borderRadius: '9999px',
    padding: '2px 10px',
  },
  command: {
    margin: 0,
    padding: '0.75rem 1rem',
    fontFamily: 'var(--font-mono)',
    fontSize: '0.875rem',
    lineHeight: 1.5,
    color: 'var(--ink)',
    background: 'rgba(1,22,39,0.04)',
    borderRadius: '0.625rem',
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-all',
    border: '1px solid var(--border)',
  },
  timer: {
    margin: 0,
    fontSize: '0.875rem',
    color: 'var(--muted)',
  },
  actions: {
    display: 'flex',
    gap: '0.75rem',
    justifyContent: 'flex-end',
  },
  denyBtn: {
    padding: '0.5rem 1.25rem',
    fontSize: '0.9375rem',
    fontWeight: 500,
    fontFamily: 'inherit',
    background: '#fff',
    color: 'var(--ink)',
    border: '1px solid var(--border)',
    borderRadius: '9999px',
    cursor: 'pointer',
    transition: 'background var(--transition)',
  },
  approveBtn: {
    padding: '0.5rem 1.25rem',
    fontSize: '0.9375rem',
    fontWeight: 500,
    fontFamily: 'inherit',
    background: 'var(--ink)',
    color: '#fff',
    border: 'none',
    borderRadius: '9999px',
    cursor: 'pointer',
    transition: 'background var(--transition), transform var(--transition)',
  },
  hint: {
    margin: 0,
    fontSize: '0.75rem',
    color: 'var(--muted)',
    textAlign: 'center',
  },
}
