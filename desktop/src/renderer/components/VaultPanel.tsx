import { useState, FormEvent } from 'react'

interface VaultEntry {
  key: string
  value: string
  editing: boolean
  editValue: string
  saving: boolean
}

interface Props {
  onClose: () => void
}

type Phase = 'locked' | 'loading' | 'ready'

export function VaultPanel({ onClose }: Props) {
  const [phase, setPhase] = useState<Phase>('locked')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [entries, setEntries] = useState<VaultEntry[]>([])
  const [newKey, setNewKey] = useState('')
  const [newValue, setNewValue] = useState('')
  const [addingNew, setAddingNew] = useState(false)
  const [savingNew, setSavingNew] = useState(false)

  async function handleUnlock(e: FormEvent) {
    e.preventDefault()
    setError('')
    setPhase('loading')
    const listResult = await window.silo.vaultList(password)
    if (!listResult.ok) { setError(listResult.message ?? 'Wrong password'); setPhase('locked'); return }
    const keys = listResult.keys ?? []
    const allResult = await window.silo.vaultGetAll(password, keys)
    if (!allResult.ok) { setError(allResult.message ?? 'Failed to read vault'); setPhase('locked'); return }
    const loaded: VaultEntry[] = keys.map(k => ({
      key: k,
      value: allResult.entries?.[k] ?? '',
      editing: false,
      editValue: '',
      saving: false,
    }))
    setEntries(loaded)
    setPhase('ready')
  }

  function startEdit(key: string) {
    setEntries(prev => prev.map(e => e.key === key ? { ...e, editing: true, editValue: e.value } : e))
  }

  function cancelEdit(key: string) {
    setEntries(prev => prev.map(e => e.key === key ? { ...e, editing: false } : e))
  }

  async function saveEdit(key: string) {
    const entry = entries.find(e => e.key === key)
    if (!entry) return
    setEntries(prev => prev.map(e => e.key === key ? { ...e, saving: true } : e))
    const result = await window.silo.vaultSet(password, key, entry.editValue)
    if (!result.ok) {
      setEntries(prev => prev.map(e => e.key === key ? { ...e, saving: false } : e))
      setError(result.message ?? 'Failed to save')
      return
    }
    setEntries(prev => prev.map(e => e.key === key ? { ...e, value: entry.editValue, editing: false, saving: false } : e))
  }

  async function handleDelete(key: string) {
    const result = await window.silo.vaultDelete(password, key)
    if (!result.ok) { setError(result.message ?? 'Failed to delete'); return }
    setEntries(prev => prev.filter(e => e.key !== key))
  }

  async function handleAddNew(e: FormEvent) {
    e.preventDefault()
    if (!newKey.trim() || !newValue.trim()) return
    setSavingNew(true)
    const result = await window.silo.vaultSet(password, newKey.trim(), newValue.trim())
    setSavingNew(false)
    if (!result.ok) { setError(result.message ?? 'Failed to add'); return }
    setEntries(prev => [...prev, { key: newKey.trim(), value: newValue.trim(), editing: false, editValue: '', saving: false }])
    setNewKey('')
    setNewValue('')
    setAddingNew(false)
  }

  return (
    <div style={s.overlay} onClick={e => { if (e.target === e.currentTarget) onClose() }}>
      <div style={s.panel}>
        <div style={s.header}>
          <span style={s.title}>Vault</span>
          <button style={s.closeBtn} onClick={onClose} className="no-drag">✕</button>
        </div>

        {phase === 'locked' && (
          <form onSubmit={handleUnlock} style={s.lockForm}>
            <p style={s.lockHint}>Enter vault password to view and edit secrets</p>
            <input
              type="password"
              placeholder="Vault password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              autoFocus
              style={s.input}
            />
            {error && <p style={s.error}>{error}</p>}
            <button type="submit" disabled={!password} style={s.primaryBtn}>Unlock vault</button>
          </form>
        )}

        {phase === 'loading' && <p style={s.loading}>Loading…</p>}

        {phase === 'ready' && (
          <div style={s.content}>
            {error && <p style={s.error}>{error}</p>}
            {entries.length === 0 && !addingNew && <p style={s.empty}>No secrets stored</p>}
            <div style={s.entryList}>
              {entries.map(entry => (
                <div key={entry.key} style={s.entryRow}>
                  <span style={s.entryKey}>{entry.key}</span>
                  {entry.editing ? (
                    <div style={s.editRow}>
                      <input
                        type="text"
                        value={entry.editValue}
                        onChange={ev => setEntries(prev => prev.map(e => e.key === entry.key ? { ...e, editValue: ev.target.value } : e))}
                        style={{ ...s.input, flex: 1 }}
                        autoFocus
                      />
                      <button style={s.accentBtn} onClick={() => saveEdit(entry.key)} disabled={entry.saving}>
                        {entry.saving ? '…' : 'Save'}
                      </button>
                      <button style={s.ghostBtn} onClick={() => cancelEdit(entry.key)}>Cancel</button>
                    </div>
                  ) : (
                    <div style={s.editRow}>
                      <span style={s.entryValue}>{'•'.repeat(Math.min(entry.value.length, 24))}</span>
                      <button style={s.ghostBtn} onClick={() => startEdit(entry.key)}>Edit</button>
                      <button style={s.deleteBtn} onClick={() => handleDelete(entry.key)}>Delete</button>
                    </div>
                  )}
                </div>
              ))}
            </div>

            {addingNew ? (
              <form onSubmit={handleAddNew} style={s.addForm}>
                <input
                  type="text"
                  placeholder="Key"
                  value={newKey}
                  onChange={e => setNewKey(e.target.value)}
                  style={s.input}
                  autoFocus
                />
                <input
                  type="text"
                  placeholder="Value"
                  value={newValue}
                  onChange={e => setNewValue(e.target.value)}
                  style={s.input}
                />
                <div style={s.editRow}>
                  <button type="submit" style={s.accentBtn} disabled={savingNew || !newKey.trim() || !newValue.trim()}>
                    {savingNew ? 'Saving…' : 'Add'}
                  </button>
                  <button type="button" style={s.ghostBtn} onClick={() => setAddingNew(false)}>Cancel</button>
                </div>
              </form>
            ) : (
              <button style={s.addBtn} onClick={() => setAddingNew(true)}>+ Add secret</button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

const s: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(15, 35, 70, 0.18)',
    backdropFilter: 'blur(4px)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 200,
  },
  panel: {
    background: 'var(--card)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-lg)',
    boxShadow: 'var(--shadow)',
    width: 480,
    maxHeight: '80vh',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '1.125rem 1.5rem',
    borderBottom: '1px solid var(--border)',
    flexShrink: 0,
  },
  title: {
    fontSize: '0.9375rem',
    fontWeight: 600,
    color: 'var(--ink)',
  },
  closeBtn: {
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    color: 'var(--muted)',
    fontSize: '1rem',
    padding: '0.25rem',
    borderRadius: '0.375rem',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  lockForm: {
    padding: '1.5rem',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.75rem',
  },
  lockHint: {
    margin: 0,
    fontSize: '0.875rem',
    color: 'var(--muted)',
  },
  loading: {
    padding: '2rem',
    textAlign: 'center',
    color: 'var(--muted)',
    fontSize: '0.875rem',
    margin: 0,
  },
  empty: {
    color: 'var(--muted)',
    fontSize: '0.875rem',
    margin: '0 0 0.75rem',
  },
  content: {
    padding: '1.25rem 1.5rem',
    overflowY: 'auto',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
  },
  entryList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
  },
  entryRow: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.375rem',
    padding: '0.625rem 0.875rem',
    background: 'rgba(74, 124, 247, 0.04)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md)',
  },
  entryKey: {
    fontFamily: 'var(--font-mono)',
    fontSize: '0.75rem',
    fontWeight: 600,
    color: 'var(--accent)',
    letterSpacing: '0.03em',
  },
  editRow: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.5rem',
  },
  entryValue: {
    flex: 1,
    fontFamily: 'var(--font-mono)',
    fontSize: '0.875rem',
    color: 'var(--muted)',
    letterSpacing: '0.08em',
  },
  addForm: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    marginTop: '0.5rem',
    padding: '0.875rem',
    background: 'rgba(74, 124, 247, 0.04)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md)',
  },
  input: {
    width: '100%',
    padding: '0.5rem 0.75rem',
    fontSize: '0.875rem',
    fontFamily: 'var(--font-sans)',
    background: '#fff',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md)',
    color: 'var(--ink)',
    outline: 'none',
    boxSizing: 'border-box' as const,
  },
  error: {
    margin: 0,
    fontSize: '0.8125rem',
    color: '#c0392b',
  },
  primaryBtn: {
    padding: '0.5rem 1.25rem',
    fontSize: '0.9375rem',
    fontWeight: 500,
    fontFamily: 'var(--font-sans)',
    background: 'var(--accent)',
    color: '#fff',
    border: 'none',
    borderRadius: 'var(--radius-full)',
    cursor: 'pointer',
  },
  accentBtn: {
    padding: '0.375rem 0.875rem',
    fontSize: '0.8125rem',
    fontWeight: 500,
    fontFamily: 'var(--font-sans)',
    background: 'var(--accent)',
    color: '#fff',
    border: 'none',
    borderRadius: 'var(--radius-full)',
    cursor: 'pointer',
    flexShrink: 0,
  },
  ghostBtn: {
    padding: '0.375rem 0.875rem',
    fontSize: '0.8125rem',
    fontWeight: 500,
    fontFamily: 'var(--font-sans)',
    background: 'transparent',
    color: 'var(--muted)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-full)',
    cursor: 'pointer',
    flexShrink: 0,
  },
  deleteBtn: {
    padding: '0.375rem 0.875rem',
    fontSize: '0.8125rem',
    fontWeight: 500,
    fontFamily: 'var(--font-sans)',
    background: 'transparent',
    color: '#e53e3e',
    border: '1px solid rgba(229, 62, 62, 0.25)',
    borderRadius: 'var(--radius-full)',
    cursor: 'pointer',
    flexShrink: 0,
  },
  addBtn: {
    marginTop: '0.25rem',
    padding: '0.5rem 1rem',
    fontSize: '0.875rem',
    fontWeight: 500,
    fontFamily: 'var(--font-sans)',
    background: 'transparent',
    color: 'var(--accent)',
    border: '1px dashed rgba(74, 124, 247, 0.4)',
    borderRadius: 'var(--radius-md)',
    cursor: 'pointer',
    width: '100%',
    textAlign: 'left' as const,
  },
}
