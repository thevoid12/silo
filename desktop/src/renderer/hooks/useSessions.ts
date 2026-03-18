import { useState, useEffect, useCallback } from 'react'
import type { Session } from '@shared/types'
import type { ApiClient } from '../lib/api'

export function useSessions(client: ApiClient) {
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(false)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const data = await client.listSessions()
      setSessions(data.sort((a, b) => b.updated_at.localeCompare(a.updated_at)))
    } catch { /* silently fail — server may not have sessions yet */ }
    setLoading(false)
  }, [client])

  useEffect(() => { refresh() }, [refresh])

  return { sessions, loading, refresh }
}
