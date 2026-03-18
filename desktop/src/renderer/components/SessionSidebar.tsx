import type { Session } from '@shared/types'

interface Props {
  sessions: Session[]
  loading: boolean
  activeSessionId: string | null
  onSelect: (id: string) => void
  onNew: () => void
}

export function SessionSidebar({ sessions, loading, activeSessionId, onSelect, onNew }: Props) {
  return (
    <>
      <div style={s.header}>
        <span style={s.label}>Sessions</span>
        <button style={s.newBtn} onClick={onNew} title="New session" className="no-drag">+</button>
      </div>
      <nav style={s.list}>
        {loading && <p style={s.dim}>Loading…</p>}
        {!loading && sessions.length === 0 && <p style={s.dim}>No sessions yet</p>}
        {sessions.map(sess => (
          <button
            key={sess.id}
            style={{ ...s.row, ...(sess.id === activeSessionId ? s.rowActive : {}) }}
            onClick={() => onSelect(sess.id)}
            className="no-drag"
          >
            <span style={s.rowId}>{sess.id.slice(0, 8)}</span>
            <span style={s.rowTime}>{relativeTime(sess.updated_at)}</span>
          </button>
        ))}
      </nav>
    </>
  )
}

function relativeTime(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime()
  if (ms < 60_000) return 'just now'
  if (ms < 3_600_000) return `${Math.floor(ms / 60_000)}m ago`
  if (ms < 86_400_000) return `${Math.floor(ms / 3_600_000)}h ago`
  return `${Math.floor(ms / 86_400_000)}d ago`
}

const s: Record<string, React.CSSProperties> = {
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '0.75rem 1rem 0.5rem',
  },
  label: {
    fontSize: '0.6875rem',
    fontWeight: 500,
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
    color: 'var(--muted)',
  },
  newBtn: {
    width: 22,
    height: 22,
    borderRadius: '9999px',
    border: '1px solid var(--border)',
    background: 'transparent',
    color: 'var(--muted)',
    fontSize: '1rem',
    lineHeight: 1,
    cursor: 'pointer',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    transition: 'color var(--transition), background var(--transition)',
    padding: 0,
  },
  list: {
    flex: 1,
    overflowY: 'auto',
    padding: '0 0.5rem 1rem',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.125rem',
  },
  row: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '0.5rem 0.625rem',
    borderRadius: '0.625rem',
    border: 'none',
    background: 'transparent',
    cursor: 'pointer',
    fontFamily: 'inherit',
    width: '100%',
    textAlign: 'left',
    transition: 'background var(--transition)',
  },
  rowActive: {
    background: 'rgba(1,22,39,0.07)',
  },
  rowId: {
    fontFamily: 'var(--font-mono)',
    fontSize: '0.8125rem',
    color: 'var(--ink)',
  },
  rowTime: {
    fontSize: '0.75rem',
    color: 'var(--muted)',
  },
  dim: {
    fontSize: '0.8125rem',
    color: 'var(--muted)',
    padding: '0.5rem 0.625rem',
    margin: 0,
  },
}
