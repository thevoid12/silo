import { useState, useEffect } from 'react'
import type { Settings } from '@shared/types'
import type { ApiClient } from '../lib/api'

interface Props {
  client: ApiClient
  onClose: () => void
}

const APPROVAL_MODES = ['always', 'per-tool', 'never']

export function SettingsView({ client, onClose }: Props) {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [draft, setDraft] = useState<Settings | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    client.getSettings()
      .then(s => { setSettings(s); setDraft(s) })
      .catch(e => setError(String(e)))
  }, [client])

  function set<K extends keyof Settings>(key: K, value: Settings[K]) {
    setDraft(prev => prev ? { ...prev, [key]: value } : prev)
  }

  async function handleSave() {
    if (!draft || !settings) return
    setSaving(true)
    setError('')
    try {
      const updated = await client.updateSettings(draft)
      setSettings(updated)
      setDraft(updated)
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (e) {
      setError(String(e))
    } finally {
      setSaving(false)
    }
  }

  const dirty = draft && settings && JSON.stringify(draft) !== JSON.stringify(settings)

  return (
    <div style={s.overlay} onClick={e => { if (e.target === e.currentTarget) onClose() }}>
      <div style={s.panel}>
        <div style={s.header}>
          <span style={s.title}>Settings</span>
          <button style={s.closeBtn} onClick={onClose}>✕</button>
        </div>

        {!draft && !error && <p style={s.loading}>Loading…</p>}
        {error && <p style={s.error}>{error}</p>}

        {draft && (
          <div style={s.body}>
            <Section label="AI Provider">
              <Field label="Provider">
                <input
                  style={s.input}
                  placeholder="gemini, openai, anthropic, openrouter, …"
                  value={draft.provider}
                  onChange={e => set('provider', e.target.value)}
                />
              </Field>
              <Field label="Model">
                <input
                  style={s.input}
                  placeholder="e.g. gemini-2.0-flash, gpt-4o, claude-sonnet-4-6"
                  value={draft.model}
                  onChange={e => set('model', e.target.value)}
                />
              </Field>
              <Field label="Base URL">
                <input
                  style={s.input}
                  placeholder="Leave blank for built-in default (gemini,openai,claude,openrouter)"
                  value={draft.base_url}
                  onChange={e => set('base_url', e.target.value)}
                />
              </Field>
            </Section>

            <Section label="Agent">
              <Field label="Max iterations">
                <input
                  style={{ ...s.input, width: 80 }}
                  type="number"
                  min={1}
                  max={100}
                  value={draft.max_iterations}
                  onChange={e => set('max_iterations', parseInt(e.target.value, 10) || 1)}
                />
              </Field>
            </Section>

            <Section label="Tool Approval">
              <Field label="Mode">
                <select
                  value={draft.approval_mode}
                  onChange={e => set('approval_mode', e.target.value)}
                  style={s.select}
                >
                  {APPROVAL_MODES.map(m => <option key={m} value={m}>{m}</option>)}
                </select>
              </Field>
            </Section>

            <Section label="Shell">
              <Field label="Timeout (seconds)">
                <input
                  style={{ ...s.input, width: 80 }}
                  type="number"
                  min={1}
                  value={draft.shell_timeout_secs}
                  onChange={e => set('shell_timeout_secs', parseInt(e.target.value, 10) || 1)}
                />
              </Field>
            </Section>

            <div style={s.footer}>
              {error && <span style={s.error}>{error}</span>}
              <button
                style={saved ? s.savedBtn : dirty ? s.saveBtn : s.saveBtnDisabled}
                onClick={handleSave}
                disabled={saving || !dirty}
              >
                {saving ? 'Saving…' : saved ? 'Saved ✓' : 'Save changes'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={ss.section}>
      <p style={ss.sectionLabel}>{label}</p>
      <div style={ss.sectionBody}>{children}</div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={ss.field}>
      <span style={ss.fieldLabel}>{label}</span>
      {children}
    </div>
  )
}

const ss: Record<string, React.CSSProperties> = {
  section: { marginBottom: '1.25rem' },
  sectionLabel: {
    margin: '0 0 0.5rem',
    fontSize: '0.6875rem',
    fontWeight: 600,
    textTransform: 'uppercase',
    letterSpacing: '0.07em',
    color: 'var(--muted)',
  },
  sectionBody: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    padding: '0.75rem',
    background: 'rgba(74,124,247,0.04)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md)',
  },
  field: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: '1rem',
  },
  fieldLabel: {
    fontSize: '0.875rem',
    color: 'var(--ink)',
    flexShrink: 0,
  },
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
  },
  body: {
    padding: '1.25rem 1.5rem',
    overflowY: 'auto',
  },
  loading: {
    padding: '2rem',
    textAlign: 'center',
    color: 'var(--muted)',
    fontSize: '0.875rem',
    margin: 0,
  },
  footer: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
    gap: '0.75rem',
    paddingTop: '0.75rem',
    borderTop: '1px solid var(--border)',
    marginTop: '0.5rem',
  },
  error: {
    fontSize: '0.8125rem',
    color: '#c0392b',
    margin: 0,
    flex: 1,
  },
  input: {
    padding: '0.375rem 0.625rem',
    fontSize: '0.875rem',
    fontFamily: 'var(--font-sans)',
    background: '#fff',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md)',
    color: 'var(--ink)',
    outline: 'none',
    boxSizing: 'border-box' as const,
    flex: 1,
  },
  select: {
    padding: '0.375rem 0.625rem',
    fontSize: '0.875rem',
    fontFamily: 'var(--font-sans)',
    background: '#fff',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md)',
    color: 'var(--ink)',
    outline: 'none',
    cursor: 'pointer',
  },
  saveBtn: {
    padding: '0.5rem 1.25rem',
    fontSize: '0.875rem',
    fontWeight: 500,
    fontFamily: 'var(--font-sans)',
    background: 'var(--accent)',
    color: '#fff',
    border: 'none',
    borderRadius: 'var(--radius-full)',
    cursor: 'pointer',
  },
  saveBtnDisabled: {
    padding: '0.5rem 1.25rem',
    fontSize: '0.875rem',
    fontWeight: 500,
    fontFamily: 'var(--font-sans)',
    background: 'var(--border)',
    color: 'var(--muted)',
    border: 'none',
    borderRadius: 'var(--radius-full)',
    cursor: 'default',
  },
  savedBtn: {
    padding: '0.5rem 1.25rem',
    fontSize: '0.875rem',
    fontWeight: 500,
    fontFamily: 'var(--font-sans)',
    background: '#27ae60',
    color: '#fff',
    border: 'none',
    borderRadius: 'var(--radius-full)',
    cursor: 'default',
  },
}
