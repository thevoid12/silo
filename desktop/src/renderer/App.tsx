import { useState, useEffect } from 'react'
import { UnlockScreen } from './components/UnlockScreen'
import { Shell } from './components/Shell'
import type { SiloConnection } from '@shared/types'

export default function App() {
  const [connection, setConnection] = useState<SiloConnection | null>(null)

  useEffect(() => {
    window.silo.onReady(setConnection)
    window.silo.getConnection().then(conn => { if (conn) setConnection(conn) })
  }, [])

  if (!connection) return <UnlockScreen onUnlock={setConnection} />
  return <Shell connection={connection} />
}
