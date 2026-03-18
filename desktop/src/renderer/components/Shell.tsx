import type { SiloConnection } from '@shared/types'

interface Props {
  connection: SiloConnection
}

export function Shell({ connection }: Props) {
  return (
    <div style={styles.root}>
      {/* Left rail */}
      <aside style={styles.rail}>
        <div className="drag-region" style={styles.railHeader}>
          <span style={styles.wordmark}>silo</span>
        </div>
        <nav style={styles.railNav}>
          <p style={styles.placeholder}>Sessions</p>
        </nav>
      </aside>

      {/* Center canvas */}
      <main style={styles.canvas}>
        <div className="drag-region" style={styles.canvasHeader} />
        <div style={styles.canvasBody}>
          <p style={styles.placeholder}>
            Connected to port {connection.port}
          </p>
        </div>
      </main>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  root: {
    display: 'flex',
    height: '100vh',
    overflow: 'hidden',
    background: 'var(--bg)',
  },
  rail: {
    width: 'var(--rail-left)',
    flexShrink: 0,
    background: 'rgba(246, 249, 252, 0.85)',
    borderRight: '1px solid var(--border)',
    display: 'flex',
    flexDirection: 'column',
    backdropFilter: 'blur(8px)',
  },
  railHeader: {
    height: 52,
    display: 'flex',
    alignItems: 'center',
    paddingLeft: '1rem',
    borderBottom: '1px solid var(--border)',
  },
  wordmark: {
    fontSize: '1rem',
    fontWeight: 600,
    color: 'var(--ink)',
    letterSpacing: '-0.02em',
  },
  railNav: {
    flex: 1,
    overflowY: 'auto',
    padding: '0.75rem 0.5rem',
  },
  canvas: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    background: '#fff',
    overflow: 'hidden',
  },
  canvasHeader: {
    height: 52,
    borderBottom: '1px solid var(--border)',
  },
  canvasBody: {
    flex: 1,
    overflowY: 'auto',
    padding: '2rem',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  placeholder: {
    color: 'var(--muted)',
    fontSize: '0.875rem',
  },
}
