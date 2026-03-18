import { useMemo, useState } from 'react'
import type { SiloConnection } from '@shared/types'
import { createApiClient } from '../lib/api'
import { useSessions } from '../hooks/useSessions'
import { SessionSidebar } from './SessionSidebar'
import { ChatView } from './ChatView'
import { VaultPanel } from './VaultPanel'
import { SettingsView } from './SettingsView'

interface Props {
  connection: SiloConnection
}

export function Shell({ connection }: Props) {
  const client = useMemo(() => createApiClient(connection), [connection])
  const { sessions, loading, refresh } = useSessions(client)
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null)
  const [vaultOpen, setVaultOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)

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
          <button style={s.footerBtn} onClick={() => setVaultOpen(true)} title="Open vault">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
              <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
              <circle cx="12" cy="16" r="1" fill="currentColor" stroke="none"/>
            </svg>
            <span style={s.footerLabel}>Vault</span>
          </button>
          <button style={s.footerBtn} onClick={() => setSettingsOpen(true)} title="Settings">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="3"/>
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
            </svg>
            <span style={s.footerLabel}>Settings</span>
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
      {settingsOpen && <SettingsView client={client} onClose={() => setSettingsOpen(false)} />}
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
    display: 'flex',
    flexDirection: 'column',
    gap: '0.375rem',
  },
  footerBtn: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.5rem',
    width: '100%',
    background: 'none',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md)',
    cursor: 'pointer',
    padding: '0.5rem 0.75rem',
    color: 'var(--muted)',
  },
  footerLabel: {
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
