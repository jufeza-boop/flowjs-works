import { useState, useEffect, useCallback } from 'react'
import type { Execution, ActivityLog } from '../types/audit'
import { fetchExecutions, fetchActivityLogs, fetchTriggerData, replayExecution, replayFromNode } from '../lib/api'

const LIMIT = 20

// ---------------------------------------------------------------------------
// JSON Viewer — lightweight recursive renderer
// ---------------------------------------------------------------------------

interface JsonViewerProps {
  value: unknown
  depth?: number
}

function JsonViewer({ value, depth = 0 }: JsonViewerProps) {
  if (value === null || value === undefined) {
    return <span className="text-gray-400">null</span>
  }
  if (typeof value === 'boolean') {
    return <span className="text-blue-500">{value ? 'true' : 'false'}</span>
  }
  if (typeof value === 'number') {
    return <span className="text-purple-500">{value}</span>
  }
  if (typeof value === 'string') {
    return <span className="text-green-600">"{value}"</span>
  }
  if (Array.isArray(value)) {
    if (value.length === 0) return <span className="text-gray-500">[]</span>
    return (
      <span>
        {'['}
        <div style={{ paddingLeft: (depth + 1) * 12 }}>
          {value.map((item, i) => (
            <div key={i}>
              <JsonViewer value={item} depth={depth + 1} />
              {i < value.length - 1 && ','}
            </div>
          ))}
        </div>
        {']'}
      </span>
    )
  }
  if (typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
    if (entries.length === 0) return <span className="text-gray-500">{'{}'}</span>
    return (
      <span>
        {'{'}
        <div style={{ paddingLeft: (depth + 1) * 12 }}>
          {entries.map(([k, v], i) => (
            <div key={k}>
              <span className="text-red-500">"{k}"</span>:{' '}
              <JsonViewer value={v} depth={depth + 1} />
              {i < entries.length - 1 && ','}
            </div>
          ))}
        </div>
        {'}'}
      </span>
    )
  }
  return <span>{String(value)}</span>
}

// ---------------------------------------------------------------------------
// Status badge helper
// ---------------------------------------------------------------------------

function StatusBadge({ status }: { status: string }) {
  const color =
    status === 'COMPLETED' || status === 'SUCCESS'
      ? 'bg-green-100 text-green-700'
      : status === 'FAILED' || status === 'ERROR'
        ? 'bg-red-100 text-red-700'
        : status === 'STARTED'
          ? 'bg-blue-100 text-blue-700'
          : 'bg-gray-100 text-gray-600'
  return (
    <span className={`px-2 py-0.5 rounded text-xs font-medium ${color}`}>{status || '—'}</span>
  )
}

// ---------------------------------------------------------------------------
// Log detail panel — displays activity logs for the selected execution
// ---------------------------------------------------------------------------

interface LogDetailPanelProps {
  executionId: string
  flowId: string
  onClose: () => void
}

function LogDetailPanel({ executionId, flowId, onClose }: LogDetailPanelProps) {
  const [logs, setLogs] = useState<ActivityLog[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedLog, setSelectedLog] = useState<ActivityLog | null>(null)
  const [activeTab, setActiveTab] = useState<'input' | 'output' | 'error'>('input')
  const [resumeStatus, setResumeStatus] = useState<{ nodeId: string; success: boolean } | null>(null)

  useEffect(() => {
    setLoading(true)
    setError(null)
    fetchActivityLogs(executionId)
      .then((data) => {
        setLogs(data)
        setSelectedLog(data[0] ?? null)
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false))
  }, [executionId])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-lg shadow-xl w-[900px] max-h-[80vh] flex flex-col overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200 bg-gray-50">
          <div>
            <h2 className="text-sm font-semibold text-gray-800">Execution Detail</h2>
            <p className="text-xs text-gray-400 font-mono mt-0.5">{executionId}</p>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 text-xl leading-none"
            aria-label="Close"
          >
            ×
          </button>
        </div>

        {loading && (
          <div className="flex-1 flex items-center justify-center p-8 text-sm text-gray-400">
            Loading activity logs…
          </div>
        )}
        {error && (
          <div className="flex-1 flex items-center justify-center p-8 text-sm text-red-500">
            {error}
          </div>
        )}
        {!loading && !error && (
          <div className="flex flex-1 overflow-hidden">
            {/* Left: node list */}
            <div className="w-52 border-r border-gray-200 overflow-y-auto">
              {logs.length === 0 ? (
                <p className="p-4 text-xs text-gray-400">No activity logs found.</p>
              ) : (
                logs.map((log) => (
                  <div key={log.log_id} className="border-b border-gray-100">
                    <button
                      onClick={() => setSelectedLog(log)}
                      className={`w-full text-left px-3 py-2 hover:bg-gray-50 transition-colors ${
                        selectedLog?.log_id === log.log_id ? 'bg-blue-50' : ''
                      }`}
                    >
                      <p className="text-xs font-medium text-gray-800 truncate">{log.node_id}</p>
                      <p className="text-xs text-gray-400 truncate">{log.node_type}</p>
                      <StatusBadge status={log.status} />
                    </button>
                    <div className="px-3 pb-2">
                      <button
                        onClick={() => {
                          setResumeStatus(null)
                          const inputObj =
                            log.input_data !== null && typeof log.input_data === 'object' && !Array.isArray(log.input_data)
                              ? (log.input_data as Record<string, unknown>)
                              : {}
                          replayFromNode(flowId, log.node_id, inputObj)
                            .then(() => setResumeStatus({ nodeId: log.node_id, success: true }))
                            .catch(() => setResumeStatus({ nodeId: log.node_id, success: false }))
                        }}
                        className="text-xs text-indigo-500 hover:text-indigo-700 transition-colors"
                      >
                        ▶ Resume
                      </button>
                      {resumeStatus?.nodeId === log.node_id && (
                        <span className={`ml-2 text-xs ${resumeStatus.success ? 'text-green-600' : 'text-red-500'}`}>
                          {resumeStatus.success ? '✓ Replayed' : '✗ Error'}
                        </span>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>

            {/* Right: payload viewer */}
            <div className="flex-1 flex flex-col overflow-hidden">
              {selectedLog ? (
                <>
                  {/* Tabs */}
                  <div className="flex border-b border-gray-200 px-4 pt-2 gap-4">
                    {(['input', 'output', 'error'] as const).map((tab) => (
                      <button
                        key={tab}
                        onClick={() => setActiveTab(tab)}
                        className={`pb-2 text-xs font-medium capitalize border-b-2 transition-colors ${
                          activeTab === tab
                            ? 'border-blue-500 text-blue-600'
                            : 'border-transparent text-gray-400 hover:text-gray-600'
                        }`}
                      >
                        {tab === 'input' ? 'Input Data' : tab === 'output' ? 'Output Data' : 'Error Details'}
                      </button>
                    ))}
                  </div>
                  {/* JSON viewer */}
                  <div className="flex-1 overflow-auto p-4 font-mono text-xs bg-gray-50">
                    {activeTab === 'input' && (
                      <JsonViewer value={selectedLog.input_data} />
                    )}
                    {activeTab === 'output' && (
                      <JsonViewer value={selectedLog.output_data} />
                    )}
                    {activeTab === 'error' && (
                      <JsonViewer value={selectedLog.error_details} />
                    )}
                  </div>
                  {/* Footer meta */}
                  <div className="px-4 py-2 border-t border-gray-200 flex gap-4 text-xs text-gray-400">
                    <span>Duration: {selectedLog.duration_ms}ms</span>
                    <span>Created: {new Date(selectedLog.created_at).toLocaleString()}</span>
                  </div>
                </>
              ) : (
                <div className="flex-1 flex items-center justify-center text-sm text-gray-400">
                  Select a node from the list.
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Execution History — main view
// ---------------------------------------------------------------------------

export function ExecutionHistory() {
  const [executions, setExecutions] = useState<Execution[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedExecution, setSelectedExecution] = useState<Execution | null>(null)
  const [statusFilter, setStatusFilter] = useState('')
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)
  const [replayMessage, setReplayMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const load = useCallback((currentOffset: number, currentStatus: string, currentSearch: string) => {
    setLoading(true)
    setError(null)
    fetchExecutions({
      status: currentStatus || undefined,
      search: currentSearch || undefined,
      limit: LIMIT,
      offset: currentOffset,
    })
      .then(setExecutions)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    load(offset, statusFilter, search)
  }, [load, offset, statusFilter, search])

  function handleStatusChange(e: React.ChangeEvent<HTMLSelectElement>) {
    setStatusFilter(e.target.value)
    setOffset(0)
  }

  function triggerSearch() {
    setSearch(searchInput)
    setOffset(0)
  }

  function handleSearchKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') triggerSearch()
  }

  async function handleReplay(e: React.MouseEvent, exec: Execution) {
    e.stopPropagation()
    setReplayMessage(null)
    try {
      const triggerData = await fetchTriggerData(exec.execution_id)
      await replayExecution(exec.flow_id, triggerData)
      setReplayMessage({ type: 'success', text: `Replay started for ${exec.execution_id}` })
    } catch (err) {
      setReplayMessage({ type: 'error', text: err instanceof Error ? err.message : String(err) })
    }
  }

  return (
    <div className="flex-1 flex flex-col overflow-hidden bg-white">
      {/* Panel header */}
      <div className="flex flex-col gap-2 px-5 py-3 border-b border-gray-200 bg-white shadow-sm">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-gray-700">Execution History</h2>
          <button
            onClick={() => load(offset, statusFilter, search)}
            className="text-xs text-blue-500 hover:text-blue-700 transition-colors"
            disabled={loading}
          >
            {loading ? 'Loading…' : '↻ Refresh'}
          </button>
        </div>
        {/* Filters row */}
        <div className="flex items-center gap-2">
          <select
            value={statusFilter}
            onChange={handleStatusChange}
            className="text-xs border border-gray-200 rounded px-2 py-1 bg-white text-gray-700 focus:outline-none focus:ring-1 focus:ring-blue-400"
          >
            <option value="">All statuses</option>
            <option value="STARTED">STARTED</option>
            <option value="COMPLETED">COMPLETED</option>
            <option value="FAILED">FAILED</option>
            <option value="REPLAYED">REPLAYED</option>
          </select>
          <input
            type="text"
            placeholder="Search input/output data…"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={handleSearchKeyDown}
            className="text-xs border border-gray-200 rounded px-2 py-1 flex-1 focus:outline-none focus:ring-1 focus:ring-blue-400"
          />
          <button
            onClick={triggerSearch}
            className="text-xs bg-blue-50 text-blue-600 border border-blue-200 rounded px-3 py-1 hover:bg-blue-100 transition-colors"
          >
            Search
          </button>
        </div>
        {/* Replay message */}
        {replayMessage && (
          <div className={`text-xs px-2 py-1 rounded ${replayMessage.type === 'success' ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}`}>
            {replayMessage.text}
          </div>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto">
        {loading && (
          <div className="flex items-center justify-center h-full text-sm text-gray-400">
            Loading executions…
          </div>
        )}
        {error && (
          <div className="flex flex-col items-center justify-center h-full gap-3">
            <p className="text-sm text-red-500">{error}</p>
            <button
              onClick={() => load(offset, statusFilter, search)}
              className="text-xs bg-red-50 text-red-600 border border-red-200 rounded px-3 py-1.5 hover:bg-red-100 transition-colors"
            >
              Retry
            </button>
          </div>
        )}
        {!loading && !error && executions.length === 0 && (
          <div className="flex items-center justify-center h-full text-sm text-gray-400">
            No executions recorded yet.
          </div>
        )}
        {!loading && !error && executions.length > 0 && (
          <table className="min-w-full text-xs border-collapse">
            <thead className="bg-gray-50 sticky top-0 z-10">
              <tr>
                {['Execution ID', 'Flow ID', 'Status', 'Trigger', 'Start Time', ''].map((h) => (
                  <th
                    key={h}
                    className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider border-b border-gray-200"
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {executions.map((exec, idx) => (
                <tr
                  key={exec.execution_id}
                  className={`hover:bg-blue-50 transition-colors cursor-pointer ${idx % 2 === 0 ? 'bg-white' : 'bg-gray-50/50'}`}
                  onClick={() => setSelectedExecution(exec)}
                >
                  <td className="px-4 py-2 font-mono text-gray-600 max-w-[160px] truncate">
                    {exec.execution_id}
                  </td>
                  <td className="px-4 py-2 text-gray-700">{exec.flow_id}</td>
                  <td className="px-4 py-2">
                    <StatusBadge status={exec.status} />
                  </td>
                  <td className="px-4 py-2 text-gray-500">{exec.trigger_type || '—'}</td>
                  <td className="px-4 py-2 text-gray-500">
                    {exec.start_time ? new Date(exec.start_time).toLocaleString() : '—'}
                  </td>
                  <td className="px-4 py-2 flex items-center gap-2">
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        setSelectedExecution(exec)
                      }}
                      className="text-blue-500 hover:text-blue-700 transition-colors"
                    >
                      View logs →
                    </button>
                    <button
                      onClick={(e) => { void handleReplay(e, exec) }}
                      className="text-indigo-500 hover:text-indigo-700 transition-colors"
                    >
                      ↺ Replay
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Pagination */}
      {!loading && !error && (
        <div className="flex items-center justify-between px-5 py-2 border-t border-gray-200 bg-white text-xs text-gray-500">
          <button
            onClick={() => setOffset((o) => Math.max(0, o - LIMIT))}
            disabled={offset === 0}
            className="px-3 py-1 rounded border border-gray-200 disabled:opacity-40 hover:bg-gray-50 transition-colors"
          >
            ← Previous
          </button>
          <span>Page {Math.floor(offset / LIMIT) + 1}</span>
          <button
            onClick={() => setOffset((o) => o + LIMIT)}
            disabled={executions.length < LIMIT}
            className="px-3 py-1 rounded border border-gray-200 disabled:opacity-40 hover:bg-gray-50 transition-colors"
          >
            Next →
          </button>
        </div>
      )}

      {/* Log detail modal */}
      {selectedExecution && (
        <LogDetailPanel
          executionId={selectedExecution.execution_id}
          flowId={selectedExecution.flow_id}
          onClose={() => setSelectedExecution(null)}
        />
      )}
    </div>
  )
}
