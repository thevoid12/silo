import type { SiloConnection, Session, Settings } from '@shared/types'

// createApiClient returns typed HTTP methods bound to the given connection
export function createApiClient(conn: SiloConnection) {
  const base = `http://127.0.0.1:${conn.port}`
  const auth = `Bearer ${conn.token}`

  async function get<T>(path: string): Promise<T> {
    const res = await fetch(`${base}${path}`, { headers: { Authorization: auth } })
    if (!res.ok) throw new Error(`GET ${path} failed: ${res.status}`)
    return res.json()
  }

  async function post(path: string, body: unknown): Promise<Response> {
    return fetch(`${base}${path}`, {
      method: 'POST',
      headers: { Authorization: auth, 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  }

  async function patch<T>(path: string, body: unknown): Promise<T> {
    const res = await fetch(`${base}${path}`, {
      method: 'PATCH',
      headers: { Authorization: auth, 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    if (!res.ok) throw new Error(`PATCH ${path} failed: ${res.status}`)
    return res.json()
  }

  return {
    listSessions: () => get<Session[]>('/silo/vault/sessions'),
    getSettings: () => get<Settings>('/silo/settings'),
    updateSettings: (changes: Partial<Settings>) => patch<Settings>('/silo/settings', changes),

    resolveApproval: async (requestId: string, approved: boolean): Promise<void> => {
      await post('/silo/brain/tool-approval', { request_id: requestId, approved })
    },

    chatStream: (message: string, sessionId: string | null): Promise<Response> =>
      post('/silo/brain/chat', { message, session_id: sessionId ?? '' }),
  }
}

export type ApiClient = ReturnType<typeof createApiClient>
