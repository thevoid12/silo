import { useState, FormEvent } from 'react'
import type { SetupParams } from '@shared/types'

interface Props {
  onComplete: (password: string) => void
}

export function SetupScreen({ onComplete }: Props) {
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [provider, setProvider] = useState<'gemini' | 'openai'>('gemini')
  const [apiKey, setApiKey] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (password.length < 8) { setError('Password must be at least 8 characters'); return }
    if (password !== confirm) { setError('Passwords do not match'); return }
    if (!apiKey.trim()) { setError('API key is required'); return }
    setLoading(true)
    const params: SetupParams = { password, provider, apiKey: apiKey.trim() }
    const result = await window.silo.setup(params)
    setLoading(false)
    if (!result.ok) { setError(result.message ?? 'Setup failed'); return }
    onComplete(password)
  }

  return (
    <div style={s.root}>
      <div style={s.card}>
        <div style={s.wordmark}>silo</div>
        <p style={s.subtitle}>Set up your vault to get started</p>
        <form onSubmit={handleSubmit} style={s.form}>
          <label style={s.label}>Vault password</label>
          <input
            type="password"
            placeholder="Min. 8 characters"
            value={password}
            onChange={e => setPassword(e.target.value)}
            disabled={loading}
            autoFocus
            style={s.input}
          />
          <label style={s.label}>Confirm password</label>
          <input
            type="password"
            placeholder="Repeat password"
            value={confirm}
            onChange={e => setConfirm(e.target.value)}
            disabled={loading}
            style={s.input}
          />
          <label style={s.label}>LLM provider</label>
          <select
            value={provider}
            onChange={e => setProvider(e.target.value as 'gemini' | 'openai')}
            disabled={loading}
            style={s.select}
          >
            <option value="gemini">Gemini</option>
            <option value="openai">OpenAI</option>
          </select>
          <label style={s.label}>API key</label>
          <input
            type="password"
            placeholder={provider === 'gemini' ? 'AIza…' : 'sk-…'}
            value={apiKey}
            onChange={e => setApiKey(e.target.value)}
            disabled={loading}
            style={s.input}
          />
          {error && <p style={s.error}>{error}</p>}
          <button type="submit" disabled={loading || !password || !confirm || !apiKey} style={s.btn}>
            {loading ? 'Setting up…' : 'Create vault'}
          </button>
        </form>
      </div>
    </div>
  )
}

const s: Record<string, React.CSSProperties> = {
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
    width: 360,
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
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
    margin: '0 0 0.5rem',
  },
  form: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
  },
  label: {
    fontSize: '0.75rem',
    fontWeight: 500,
    color: 'var(--muted)',
    marginTop: '0.25rem',
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
    boxSizing: 'border-box',
  },
  select: {
    width: '100%',
    padding: '0.625rem 0.875rem',
    fontSize: '0.9375rem',
    fontFamily: 'var(--font-sans)',
    background: 'rgba(255,255,255,0.9)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md)',
    color: 'var(--ink)',
    outline: 'none',
    cursor: 'pointer',
    boxSizing: 'border-box',
  },
  error: {
    fontSize: '0.8125rem',
    color: '#c0392b',
    margin: '0.25rem 0 0',
  },
  btn: {
    marginTop: '0.5rem',
    padding: '0.625rem 1rem',
    fontSize: '0.9375rem',
    fontWeight: 500,
    fontFamily: 'var(--font-sans)',
    background: 'var(--accent)',
    color: '#fff',
    border: 'none',
    borderRadius: 'var(--radius-full)',
    cursor: 'pointer',
    transition: 'background var(--transition)',
  },
}
