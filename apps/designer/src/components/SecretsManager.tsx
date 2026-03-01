import { useState, useEffect, useCallback } from 'react'
import type { SecretMeta, SecretInput, SecretType } from '../types/secrets'
import { listSecrets, createSecret, deleteSecret } from '../lib/api'

const SECRET_TYPES: SecretType[] = ['basic_auth', 'token', 'certificate', 'connection_string', 'aws_credentials', 'ssh_key', 'amqp_url']

/** Fields shown per secret type when creating a new secret */
const SECRET_VALUE_FIELDS: Record<SecretType, string[]> = {
  // user (not username) matches what SFTP, SMB, Mail, and SQL activities expect
  basic_auth: ['user', 'password'],
  token: ['token'],
  certificate: ['cert', 'key'],
  connection_string: ['connection_string'],
  // aws_credentials: matches S3 activity (access_key_id, secret_access_key, session_token)
  aws_credentials: ['access_key_id', 'secret_access_key', 'session_token'],
  // ssh_key: matches SFTP private-key auth (user, private_key)
  ssh_key: ['user', 'private_key'],
  // amqp_url: matches RabbitMQ activity (url_amqp with embedded credentials)
  amqp_url: ['url_amqp'],
}

/** Fields that are optional within their secret type */
const OPTIONAL_FIELDS = new Set<string>(['session_token'])


/** Blank form state */
function emptyForm(): { id: string; name: string; type: SecretType; values: Record<string, string> } {
  return { id: '', name: '', type: DEFAULT_TYPE, values: {} }
}

/**
 * SecretsManager — full CRUD panel for managing encrypted secrets.
 * Secrets are referenced in flow nodes via `secret_ref`.
 */
export function SecretsManager() {
  const [secrets, setSecrets] = useState<SecretMeta[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState(emptyForm())
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)

  const loadSecrets = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await listSecrets()
      setSecrets(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadSecrets()
  }, [loadSecrets])

  const handleTypeChange = (type: SecretType) => {
    setForm((f) => ({ ...f, type, values: {} }))
  }

  const handleValueChange = (field: string, value: string) => {
    setForm((f) => ({ ...f, values: { ...f.values, [field]: value } }))
  }

  const handleSave = async () => {
    setSaving(true)
    setSaveError(null)
    try {
      const input: SecretInput = {
        id: form.id.trim(),
        name: form.name.trim(),
        type: form.type,
        value: form.values,
      }
      await createSecret(input)
      setShowForm(false)
      setForm(emptyForm())
      await loadSecrets()
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!window.confirm(`Delete secret "${id}"? This action cannot be undone.`)) return
    setDeletingId(id)
    try {
      await deleteSecret(id)
      await loadSecrets()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setDeletingId(null)
    }
  }

  const inputClass =
    'w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400'
  const labelClass = 'block text-xs font-medium text-gray-600 mb-1'
  const selectClass =
    'w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400 bg-white'

  const valueFields = SECRET_VALUE_FIELDS[form.type]

  return (
    <div className="flex-1 overflow-auto p-6">
      <div className="max-w-3xl mx-auto space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-lg font-semibold text-gray-800">Secrets</h1>
            <p className="text-xs text-gray-500 mt-0.5">
              Encrypted credentials referenced by nodes via <code className="bg-gray-100 px-1 rounded">secret_ref</code>
            </p>
          </div>
          <button
            onClick={() => { setShowForm(true); setForm(emptyForm()); setSaveError(null) }}
            className="px-3 py-1.5 bg-blue-600 text-white text-xs font-medium rounded hover:bg-blue-700 transition-colors"
          >
            + New Secret
          </button>
        </div>

        {/* Error banner */}
        {error && (
          <div role="alert" className="text-xs text-red-600 bg-red-50 border border-red-200 rounded p-3">
            {error}
          </div>
        )}

        {/* New secret form */}
        {showForm && (
          <div className="border border-blue-200 rounded-lg bg-blue-50 p-4 space-y-4">
            <h2 className="text-sm font-semibold text-blue-800">New Secret</h2>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className={labelClass}>
                  ID <span className="text-gray-400 font-normal">(e.g. sec_postgres_main)</span>
                </label>
                <input
                  type="text"
                  value={form.id}
                  onChange={(e) => setForm((f) => ({ ...f, id: e.target.value }))}
                  className={inputClass}
                  placeholder="sec_my_credential"
                />
              </div>
              <div>
                <label className={labelClass}>Name</label>
                <input
                  type="text"
                  value={form.name}
                  onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                  className={inputClass}
                  placeholder="Postgres Main"
                />
              </div>
            </div>
            <div>
              <label className={labelClass}>Type</label>
              <select value={form.type} onChange={(e) => handleTypeChange(e.target.value as SecretType)} className={selectClass}>
                {SECRET_TYPES.map((t) => (
                  <option key={t} value={t}>{t}</option>
                ))}
              </select>
            </div>

            {/* Dynamic value fields per type */}
            <div className="space-y-2">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">
                Credentials <span className="text-red-400">— stored encrypted</span>
              </label>
              {valueFields.map((field) => (
                <div key={field}>
                  <label className={labelClass}>
                    {field}
                    {OPTIONAL_FIELDS.has(field) && (
                      <span className="text-gray-400 font-normal ml-1">(optional)</span>
                    )}
                  </label>
                  <input
                    type={field === 'password' || field === 'token' || field === 'key' || field === 'private_key' || field === 'secret_access_key' ? 'password' : 'text'}
                    value={form.values[field] ?? ''}
                    onChange={(e) => handleValueChange(field, e.target.value)}
                    className={inputClass}
                    autoComplete="off"
                  />
                </div>
              ))}
            </div>

            {saveError && (
              <p role="alert" className="text-xs text-red-600">{saveError}</p>
            )}

            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setShowForm(false)}
                className="px-3 py-1.5 text-xs border border-gray-300 rounded hover:bg-gray-100 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                disabled={saving || !form.id || !form.name}
                className="px-3 py-1.5 bg-blue-600 text-white text-xs font-medium rounded hover:bg-blue-700 disabled:opacity-50 transition-colors"
              >
                {saving ? '⏳ Saving…' : '💾 Save'}
              </button>
            </div>
          </div>
        )}

        {/* Secrets list */}
        {loading ? (
          <p className="text-xs text-gray-500">Loading secrets…</p>
        ) : secrets.length === 0 ? (
          <div className="text-center py-16 text-gray-400">
            <div className="text-4xl mb-3">🔐</div>
            <p className="text-sm font-medium text-gray-500">No secrets yet</p>
            <p className="text-xs mt-1">Create a secret to reference it in flow nodes</p>
          </div>
        ) : (
          <div className="space-y-2">
            {secrets.map((s) => (
              <div
                key={s.id}
                className="flex items-center justify-between bg-white border border-gray-200 rounded-lg px-4 py-3"
              >
                <div className="flex items-center gap-3 min-w-0">
                  <span className="text-lg">🔑</span>
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-gray-800 truncate">{s.name}</p>
                    <p className="text-xs text-gray-400 truncate">
                      <code>{s.id}</code>
                      <span className="mx-1">·</span>
                      <span className="inline-block bg-gray-100 text-gray-600 rounded px-1">{s.type}</span>
                      <span className="mx-1">·</span>
                      {new Date(s.created_at).toLocaleDateString()}
                    </p>
                  </div>
                </div>
                <button
                  onClick={() => handleDelete(s.id)}
                  disabled={deletingId === s.id}
                  className="ml-4 text-xs px-2 py-1 bg-red-50 text-red-600 border border-red-200 rounded hover:bg-red-100 disabled:opacity-50 transition-colors flex-shrink-0"
                  aria-label={`Delete secret ${s.id}`}
                >
                  {deletingId === s.id ? '⏳' : '🗑 Delete'}
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
