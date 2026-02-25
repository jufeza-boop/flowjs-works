import type { RunFlowResponse } from '../lib/api'

interface DebugPanelProps {
  result: RunFlowResponse | null
  rawResult: string | null
  onClose: () => void
}

export function DebugPanel({ result, rawResult, onClose }: DebugPanelProps) {
  if (!result && !rawResult) return null

  const nodeResults = result?.node_results ?? []

  return (
    <div className="fixed bottom-0 left-0 right-0 z-40 h-52 bg-gray-900 text-white border-t border-gray-700 flex flex-col shadow-2xl">
      <div className="flex items-center justify-between px-4 py-2 border-b border-gray-700 flex-shrink-0">
        <span className="text-xs font-semibold text-green-400">🔍 Debug Panel — Flow Execution Results</span>
        <button onClick={onClose} className="text-gray-400 hover:text-white text-xs">✕ Close</button>
      </div>
      {nodeResults.length > 0 ? (
        <div className="flex-1 overflow-auto">
          <table className="w-full text-[10px]">
            <thead className="sticky top-0 bg-gray-800">
              <tr>
                <th className="px-3 py-1.5 text-left text-gray-400 font-semibold w-36">Node ID</th>
                <th className="px-3 py-1.5 text-left text-gray-400 font-semibold w-16">Status</th>
                <th className="px-3 py-1.5 text-left text-gray-400 font-semibold">Output</th>
              </tr>
            </thead>
            <tbody>
              {nodeResults.map((nr, i) => (
                <tr key={i} className="border-t border-gray-700/50 hover:bg-gray-800/50">
                  <td className="px-3 py-1.5 font-mono text-blue-300 truncate max-w-[140px]">{nr.node_id}</td>
                  <td className="px-3 py-1.5">
                    <span className={`font-semibold ${nr.status === 'success' ? 'text-green-400' : 'text-red-400'}`}>
                      {nr.status === 'success' ? '✓' : '✗'} {nr.status}
                    </span>
                  </td>
                  <td className="px-3 py-1.5 font-mono text-gray-300 truncate max-w-xs" title={JSON.stringify(nr.output ?? null)}>
                    {nr.output !== undefined ? JSON.stringify(nr.output).slice(0, 120) : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <pre className="flex-1 overflow-auto p-3 text-[10px] font-mono text-green-400">{rawResult}</pre>
      )}
    </div>
  )
}
