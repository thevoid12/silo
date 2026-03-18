import { useState, useEffect } from 'react'
import { SetupScreen } from './components/SetupScreen'
import { UnlockScreen } from './components/UnlockScreen'
import { Shell } from './components/Shell'
import type { SiloConnection } from '@shared/types'

type Screen = 'loading' | 'setup' | 'unlock' | 'shell'

export default function App() {
  const [screen, setScreen] = useState<Screen>('loading')
  const [connection, setConnection] = useState<SiloConnection | null>(null)

  useEffect(() => {
    window.silo.onReady(conn => { setConnection(conn); setScreen('shell') })
    window.silo.getConnection().then(conn => {
      if (conn) { setConnection(conn); setScreen('shell'); return }
      window.silo.vaultExists().then(exists => setScreen(exists ? 'unlock' : 'setup'))
    })
  }, [])

  // after setup, auto-unlock with the same password — no need to re-enter it
  async function handleSetupComplete(password: string) {
    const result = await window.silo.unlock(password)
    if (!result.ok) setScreen('unlock')
  }

  if (screen === 'loading') return null
  if (screen === 'setup') return <SetupScreen onComplete={handleSetupComplete} />
  if (screen === 'unlock' || !connection) return <UnlockScreen onUnlock={conn => { setConnection(conn); setScreen('shell') }} />
  return <Shell connection={connection} />
}
