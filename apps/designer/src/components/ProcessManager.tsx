import { useState, useEffect, useCallback, useRef } from 'react'
import type { ProcessSummary, ProcessStatus } from '../types/deployment'
import type { FlowDSL } from '../types/dsl'
import {
  listProcesses,
  saveProcess,
  deleteProcess,
  deployProcess,
  stopProcess,
  getProcess,
} from '../lib/api'
import { serializeGraph } from '../lib/serializer'
import type { Node, Edge } from '@xyflow/react'
import type { NodeData } from '../types/designer'
import type { FlowDefinition } from '../types/dsl'

const STATUS_BADGE: Record<ProcessStatus, string> = {
  draft: 'bg-gray-100 text-gray-600',
  deployed: 'bg-green-100 text-green-700',
  stopped: 'bg-yellow-100 text-yellow-700',
}

const STATUS_ICON: Record<ProcessStatus, string> = {
  draft: '📄',
  deployed: '▶',
  stopped: '⏸',
}

interface Props {
  /** Currently open graph nodes & edges — passed in so the user can save the
   *  current flow directly from the Deployments tab. */
  nodes: Node<NodeData>[]
  edges: Edge[]
  definition: FlowDefinition
  /** Called when the user wants to edit a saved process in the designer. */
  onEditProcess: (processId: string) => void
}

/**
 * ProcessManager — lists saved processes and provides one-click deploy / stop.
 * Users can also push the current designer graph to the server from here.
 */
export function ProcessManager({ nodes, edges, definition, onEditProcess }: Props) {
  const [processes, setProcesses] = useState<ProcessSummary[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  // Cache of resolved trigger endpoint URLs keyed by process id
  const [triggerUrls, setTriggerUrls] = useState<Record<string, string>>({})
  // Tracks which process id just had its URL copied (for feedback)
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const reload = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await listProcesses()
      setProcesses(data ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  // ── Save current graph ──────────────────────────────────────────────────

  const handleSaveCurrent = useCallback(async () => {
    setSaving(true)
    setActionError(null)
    try {
      const dsl: FlowDSL = serializeGraph(nodes, edges, definition)
      await saveProcess(dsl)
      await reload()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }, [nodes, edges, definition, reload])

  // ── Deploy ───────────────────────────────────────────────────────────────

  const handleDeploy = useCallback(
    async (id: string) => {
      setBusyId(id)
      setActionError(null)
      try {
        await deployProcess(id)
        await reload()
      } catch (err) {
        setActionError(err instanceof Error ? err.message : String(err))
      } finally {
        setBusyId(null)
      }
    },
    [reload],
  )

  // ── Stop ─────────────────────────────────────────────────────────────────

  const handleStop = useCallback(
    async (id: string) => {
      setBusyId(id)
      setActionError(null)
      try {
        await stopProcess(id)
        await reload()
      } catch (err) {
        setActionError(err instanceof Error ? err.message : String(err))
      } finally {
        setBusyId(null)
      }
    },
    [reload],
  )

  // ── Trigger URL ──────────────────────────────────────────────────────────

  /** Fetch the full DSL for a process and compute its inbound HTTP endpoint URL. */
  const handleFetchUrl = useCallback(async (id: string) => {
    if (triggerUrls[id]) {
      // Already cached — just copy again
      void navigator.clipboard.writeText(triggerUrls[id])
      setCopiedId(id)
      if (copyTimer.current) clearTimeout(copyTimer.current)
      copyTimer.current = setTimeout(() => setCopiedId(null), 2000)
      return
    }
    try {
      const rec = await getProcess(id)
      const engineBase = (import.meta.env.VITE_ENGINE_API_URL as string | undefined) ?? 'http://localhost:9090'
      const type = rec.dsl?.trigger?.type
      const cfg = rec.dsl?.trigger?.config as Record<string, string> | undefined
      const path = cfg?.path ?? ''
      let url = ''
      if (type === 'rest' && path) {
        url = `${engineBase}/triggers${path}`
      } else if (type === 'soap' && path) {
        url = `${engineBase}/soap${path}`
      }
      if (url) {
        setTriggerUrls((prev) => ({ ...prev, [id]: url }))
        void navigator.clipboard.writeText(url)
        setCopiedId(id)
        if (copyTimer.current) clearTimeout(copyTimer.current)
        copyTimer.current = setTimeout(() => setCopiedId(null), 2000)
      } else {
        setTriggerUrls((prev) => ({ ...prev, [id]: '__none__' }))
      }
    } catch {
      // Non-critical — silently ignore
    }
  }, [triggerUrls])

  // ── Delete ───────────────────────────────────────────────────────────────

  const handleDelete = useCallback(
    async (id: string) => {
      // Truncate the id in the confirmation message to prevent excessively long prompts.
      const displayId = id.length > 80 ? id.slice(0, 80) + '…' : id
      if (!window.confirm(`Delete process "${displayId}"? This cannot be undone.`)) return
      setBusyId(id)
      setActionError(null)
      try {
        await deleteProcess(id)
        await reload()
      } catch (err) {
        setActionError(err instanceof Error ? err.message : String(err))
      } finally {
        setBusyId(null)
      }
    },
    [reload],
  )

  return (
    <div className="flex-1 overflow-auto p-6">
      <div className="max-w-3xl mx-auto space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-lg font-semibold text-gray-800">Deployments</h1>
            <p className="text-xs text-gray-500 mt-0.5">
              Deploy and manage flow triggers — cron, REST, RabbitMQ, MCP
            </p>
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => void reload()}
              disabled={loading}
              className="px-3 py-1.5 text-xs border border-gray-300 rounded hover:bg-gray-100 disabled:opacity-50 transition-colors"
            >
              {loading ? '⏳' : '↺'} Refresh
            </button>
            <button
              onClick={() => void handleSaveCurrent()}
              disabled={saving}
              className="px-3 py-1.5 bg-blue-600 text-white text-xs font-medium rounded hover:bg-blue-700 disabled:opacity-50 transition-colors"
            >
              {saving ? '⏳ Saving…' : '💾 Save Current Flow'}
            </button>
          </div>
        </div>

        {/* Error banners */}
        {error && (
          <div role="alert" className="text-xs text-red-600 bg-red-50 border border-red-200 rounded p-3">
            {error}
          </div>
        )}
        {actionError && (
          <div role="alert" className="text-xs text-red-600 bg-red-50 border border-red-200 rounded p-3">
            {actionError}
          </div>
        )}

        {/* Process list */}
        {loading && processes.length === 0 ? (
          <p className="text-xs text-gray-500">Loading processes…</p>
        ) : processes.length === 0 ? (
          <div className="text-center py-16 text-gray-400">
            <div className="text-4xl mb-3">🚀</div>
            <p className="text-sm font-medium text-gray-500">No saved processes yet</p>
            <p className="text-xs mt-1">
              Design a flow in the Process Designer then click{' '}
              <strong>Save Current Flow</strong> to save it here.
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            {processes.map((p) => (
              <div
                key={p.id}
                className="flex items-center justify-between bg-white border border-gray-200 rounded-lg px-4 py-3"
              >
                {/* Left: process info */}
                <div className="flex items-center gap-3 min-w-0">
                  <span className="text-lg">{STATUS_ICON[p.status]}</span>
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <p className="text-sm font-medium text-gray-800 truncate">{p.name}</p>
                      <span
                        className={`text-[10px] font-semibold px-1.5 py-0.5 rounded-full ${STATUS_BADGE[p.status]}`}
                      >
                        {p.status}
                      </span>
                    </div>
                    <p className="text-xs text-gray-400 truncate">
                      <code className="bg-gray-100 px-1 rounded">{p.id}</code>
                      <span className="mx-1">·</span>
                      v{p.version}
                      <span className="mx-1">·</span>
                      {new Date(p.updated_at).toLocaleString()}
                    </p>
                    {/* Trigger endpoint URL — visible for deployed REST / SOAP processes */}
                    {p.status === 'deployed' && triggerUrls[p.id] && triggerUrls[p.id] !== '__none__' && (
                      <div className="flex items-center gap-1 mt-1">
                        <code className="text-[10px] bg-blue-50 text-blue-700 border border-blue-200 px-1.5 py-0.5 rounded truncate max-w-[260px]">
                          {triggerUrls[p.id]}
                        </code>
                        <button
                          onClick={() => void handleFetchUrl(p.id)}
                          className="text-[10px] px-1.5 py-0.5 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors flex-shrink-0"
                          title="Copy URL"
                        >
                          {copiedId === p.id ? '✓ Copied' : '⎘ Copy'}
                        </button>
                      </div>
                    )}
                  </div>
                </div>

                {/* Right: action buttons */}
                <div className="flex items-center gap-2 ml-4 flex-shrink-0">
                  {/* Show endpoint URL button — only for deployed REST / SOAP processes */}
                  {p.status === 'deployed' && (p.trigger_type === 'rest' || p.trigger_type === 'soap') && (
                    <button
                      onClick={() => void handleFetchUrl(p.id)}
                      disabled={busyId === p.id}
                      className="text-xs px-2.5 py-1.5 bg-indigo-50 text-indigo-600 border border-indigo-200 rounded hover:bg-indigo-100 disabled:opacity-50 transition-colors"
                      title="Show and copy the inbound endpoint URL for this trigger"
                      aria-label={`Copy endpoint URL for ${p.id}`}
                    >
                      {copiedId === p.id ? '✓ Copied!' : '🔗 URL'}
                    </button>
                  )}
                  <button
                    onClick={() => onEditProcess(p.id)}
                    disabled={busyId === p.id}
                    className="text-xs px-2.5 py-1.5 bg-blue-50 text-blue-600 border border-blue-200 rounded hover:bg-blue-100 disabled:opacity-50 transition-colors"
                    aria-label={`Edit ${p.id}`}
                  >
                    ✏️ Edit
                  </button>
                  {p.status !== 'deployed' ? (
                    <button
                      onClick={() => void handleDeploy(p.id)}
                      disabled={busyId === p.id}
                      className="text-xs px-2.5 py-1.5 bg-green-600 text-white rounded hover:bg-green-700 disabled:opacity-50 transition-colors"
                      aria-label={`Deploy ${p.id}`}
                    >
                      {busyId === p.id ? '⏳' : '▶ Deploy'}
                    </button>
                  ) : (
                    <button
                      onClick={() => void handleStop(p.id)}
                      disabled={busyId === p.id}
                      className="text-xs px-2.5 py-1.5 bg-yellow-500 text-white rounded hover:bg-yellow-600 disabled:opacity-50 transition-colors"
                      aria-label={`Stop ${p.id}`}
                    >
                      {busyId === p.id ? '⏳' : '⏹ Stop'}
                    </button>
                  )}
                  <button
                    onClick={() => void handleDelete(p.id)}
                    disabled={busyId === p.id}
                    className="text-xs px-2 py-1 bg-red-50 text-red-600 border border-red-200 rounded hover:bg-red-100 disabled:opacity-50 transition-colors"
                    aria-label={`Delete ${p.id}`}
                  >
                    {busyId === p.id ? '⏳' : '🗑'}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
