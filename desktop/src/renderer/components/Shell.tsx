import { useMemo, useState } from 'react'
import type { SiloConnection } from '@shared/types'
import { createApiClient } from '../lib/api'
import { useSessions } from '../hooks/useSessions'
import { SessionSidebar } from './SessionSidebar'
import { ChatView } from './ChatView'
import { VaultPanel } from './VaultPanel'

interface Props {
  connection: SiloConnection
}

export function Shell({ connection }: Props) {
  const client = useMemo(() => createApiClient(connection), [connection])
  const { sessions, loading, refresh } = useSessions(client)
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null)
  const [vaultOpen, setVaultOpen] = useState(false)

  function handleSelectSession(id: string) {
    setSelectedSessionId(id)
  }

  function handleNewSession() {
    setSelectedSessionId(null)
  }

  function handleSessionChange(_id: string) {
    refresh()
  }

  return (
    <div style={s.root}>
      <aside style={s.rail}>
        <div className="drag-region" style={s.railHeader}>
          <span style={s.wordmark}>silo</span>
        </div>
        <SessionSidebar
          sessions={sessions}
          loading={loading}
          activeSessionId={selectedSessionId}
          onSelect={handleSelectSession}
          onNew={handleNewSession}
        />
        <div style={s.railFooter}>
          <button style={s.vaultBtn} onClick={() => setVaultOpen(true)} title="Open vault">
            🔐 <span style={s.vaultLabel}>Vault</span>
          </button>
        </div>
      </aside>

      <main style={s.canvas}>
        <div className="drag-region" style={s.canvasHeader} />
        <ChatView
          key={selectedSessionId ?? 'new'}
          client={client}
          sessionId={selectedSessionId}
          onSessionChange={handleSessionChange}
        />
      </main>

      {vaultOpen && <VaultPanel onClose={() => setVaultOpen(false)} />}
    </div>
  )
}

const s: Record<string, React.CSSProperties> = {
  root: {
    display: 'flex',
    height: '100vh',
    overflow: 'hidden',
    background: 'var(--bg)',
  },
  rail: {
    width: 'var(--rail-left)',
    flexShrink: 0,
    background: 'var(--bg-panel)',
    borderRight: '1px solid var(--border)',
    display: 'flex',
    flexDirection: 'column',
  },
  railHeader: {
    height: 52,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingLeft: '1rem',
    paddingRight: '0.75rem',
    borderBottom: '1px solid var(--border)',
  },
  wordmark: {
    fontSize: '1rem',
    fontWeight: 700,
    color: 'var(--accent)',
    letterSpacing: '-0.02em',
  },
  railFooter: {
    padding: '0.75rem',
    borderTop: '1px solid var(--border)',
    flexShrink: 0,
  },
  vaultBtn: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.5rem',
    width: '100%',
    background: 'none',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md)',
    cursor: 'pointer',
    fontSize: '0.875rem',
    padding: '0.5rem 0.75rem',
    color: 'var(--muted)',
  },
  vaultLabel: {
    fontSize: '0.8125rem',
    fontWeight: 500,
    fontFamily: 'var(--font-sans)',
  },
  canvas: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    background: 'var(--card)',
    overflow: 'hidden',
  },
  canvasHeader: {
    height: 52,
    borderBottom: '1px solid var(--border)',
    flexShrink: 0,
    background: 'var(--bg)',
  },
}
