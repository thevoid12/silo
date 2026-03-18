import { useState, FormEvent } from 'react'
import type { SiloConnection } from '@shared/types'

interface Props {
  onUnlock: (conn: SiloConnection) => void
}

export function UnlockScreen({ onUnlock }: Props) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    const result = await window.silo.unlock(password)
    setLoading(false)
    if (!result.ok) {
      setError(result.message ?? 'Failed to unlock')
      return
    }
    const conn = await window.silo.getConnection()
    if (conn) onUnlock(conn)
  }

  return (
    <div style={styles.root}>
      <div style={styles.card}>
        <div style={styles.wordmark}>silo</div>
        <p style={styles.subtitle}>Enter your vault password to continue</p>
        <form onSubmit={handleSubmit} style={styles.form}>
          <input
            type="password"
            placeholder="Vault password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            autoFocus
            disabled={loading}
            style={styles.input}
          />
          {error && <p style={styles.error}>{error}</p>}
          <button type="submit" disabled={loading || !password} style={styles.btn}>
            {loading ? 'Unlocking…' : 'Unlock'}
          </button>
        </form>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  root: {
    height: '100vh',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'var(--bg)',
  },
  card: {
    background: 'var(--card-strong)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-lg)',
    boxShadow: 'var(--shadow)',
    padding: '2.5rem 2rem',
    width: 340,
    display: 'flex',
    flexDirection: 'column',
    gap: '0.75rem',
    backdropFilter: 'blur(12px)',
  },
  wordmark: {
    fontSize: '1.5rem',
    fontWeight: 600,
    color: 'var(--ink)',
    letterSpacing: '-0.02em',
    marginBottom: '0.25rem',
  },
  subtitle: {
    fontSize: '0.875rem',
    color: 'var(--muted)',
    margin: 0,
  },
  form: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.75rem',
    marginTop: '0.5rem',
  },
  input: {
    width: '100%',
    padding: '0.625rem 0.875rem',
    fontSize: '0.9375rem',
    fontFamily: 'var(--font-sans)',
    background: 'rgba(255,255,255,0.9)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md)',
    color: 'var(--ink)',
    outline: 'none',
    transition: 'border-color var(--transition)',
  },
  error: {
    fontSize: '0.8125rem',
    color: '#c0392b',
    margin: 0,
  },
  btn: {
    padding: '0.625rem 1rem',
    fontSize: '0.9375rem',
    fontWeight: 500,
    fontFamily: 'var(--font-sans)',
    background: 'var(--ink)',
    color: '#fff',
    border: 'none',
    borderRadius: 'var(--radius-full)',
    cursor: 'pointer',
    transition: 'background var(--transition), transform var(--transition)',
    marginTop: '0.25rem',
  },
}
