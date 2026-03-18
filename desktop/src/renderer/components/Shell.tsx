import { useMemo, useState } from 'react'
import type { SiloConnection } from '@shared/types'
import { createApiClient } from '../lib/api'
import { useSessions } from '../hooks/useSessions'
import { SessionSidebar } from './SessionSidebar'
import { ChatView } from './ChatView'

interface Props {
  connection: SiloConnection
}

export function Shell({ connection }: Props) {
  const client = useMemo(() => createApiClient(connection), [connection])
  const { sessions, loading, refresh } = useSessions(client)
  // selectedSessionId drives which session the user explicitly opened (controls ChatView key)
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null)

  function handleSelectSession(id: string) {
    setSelectedSessionId(id)
  }

  function handleNewSession() {
    setSelectedSessionId(null)
  }

  // called when backend auto-assigns a session id after the first message — only refresh sidebar
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
    flexShrink: 0,
  },
}
