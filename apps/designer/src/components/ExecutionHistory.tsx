import { useState, useEffect, useCallback } from 'react'
import type { Execution, ActivityLog } from '../types/audit'
import { fetchExecutions, fetchActivityLogs } from '../lib/api'

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
  onClose: () => void
}

function LogDetailPanel({ executionId, onClose }: LogDetailPanelProps) {
  const [logs, setLogs] = useState<ActivityLog[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedLog, setSelectedLog] = useState<ActivityLog | null>(null)
  const [activeTab, setActiveTab] = useState<'input' | 'output' | 'error'>('input')

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
                  <button
                    key={log.log_id}
                    onClick={() => setSelectedLog(log)}
                    className={`w-full text-left px-3 py-2 border-b border-gray-100 hover:bg-gray-50 transition-colors ${
                      selectedLog?.log_id === log.log_id ? 'bg-blue-50' : ''
                    }`}
                  >
                    <p className="text-xs font-medium text-gray-800 truncate">{log.node_id}</p>
                    <p className="text-xs text-gray-400 truncate">{log.node_type}</p>
                    <StatusBadge status={log.status} />
                  </button>
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
  const [selectedExecutionId, setSelectedExecutionId] = useState<string | null>(null)

  const load = useCallback(() => {
    setLoading(true)
    setError(null)
    fetchExecutions()
      .then(setExecutions)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    load()
  }, [load])

  return (
    <div className="flex-1 flex flex-col overflow-hidden bg-white">
      {/* Panel header */}
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200 bg-white shadow-sm">
        <h2 className="text-sm font-semibold text-gray-700">Execution History</h2>
        <button
          onClick={load}
          className="text-xs text-blue-500 hover:text-blue-700 transition-colors"
          disabled={loading}
        >
          {loading ? 'Loading…' : '↻ Refresh'}
        </button>
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
              onClick={load}
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
                  onClick={() => setSelectedExecutionId(exec.execution_id)}
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
                  <td className="px-4 py-2">
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        setSelectedExecutionId(exec.execution_id)
                      }}
                      className="text-blue-500 hover:text-blue-700 transition-colors"
                    >
                      View logs →
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Log detail modal */}
      {selectedExecutionId && (
        <LogDetailPanel
          executionId={selectedExecutionId}
          onClose={() => setSelectedExecutionId(null)}
        />
      )}
    </div>
  )
}
