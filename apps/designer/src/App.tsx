import { useState, useCallback } from 'react'
import type { Node, Edge } from '@xyflow/react'
import { NodePalette } from './components/NodePalette'
import { DesignerCanvas } from './components/DesignerCanvas'
import { ConfigPanel } from './components/ConfigPanel'
import { ExportButton } from './components/ExportButton'
import { ExecutionHistory } from './components/ExecutionHistory'
import { serializeGraph } from './lib/serializer'
import { runFlow } from './lib/api'
import type { NodeData, DesignerNode } from './types/designer'
import type { FlowDefinition } from './types/dsl'

const DEFAULT_DEFINITION: FlowDefinition = {
  id: 'new-flow',
  version: '1.0.0',
  name: 'New Flow',
  description: '',
  settings: {
    persistence: 'full',
    timeout: 30000,
    error_strategy: 'stop_and_rollback',
  },
}

type View = 'designer' | 'history'

/** Root application — three-column designer layout or execution history view */
export default function App() {
  const [view, setView] = useState<View>('designer')
  const [selectedNode, setSelectedNode] = useState<DesignerNode | null>(null)
  const [nodes, setNodes] = useState<Node<NodeData>[]>([])
  const [edges, setEdges] = useState<Edge[]>([])
  const [runLoading, setRunLoading] = useState(false)
  const [runResult, setRunResult] = useState<string | null>(null)
  const [runError, setRunError] = useState<string | null>(null)

  const handleNodeUpdate = useCallback(
    (nodeId: string, updates: Partial<NodeData>) => {
      setNodes((nds) =>
        nds.map((n) => (n.id === nodeId ? { ...n, data: { ...n.data, ...updates } as NodeData } : n)),
      )
      if (selectedNode?.id === nodeId) {
        setSelectedNode((prev) =>
          prev ? { ...prev, data: { ...prev.data, ...updates } as NodeData } : null,
        )
      }
    },
    [selectedNode],
  )

  const handleRunFlow = useCallback(async () => {
    setRunLoading(true)
    setRunResult(null)
    setRunError(null)
    try {
      const dsl = serializeGraph(nodes, edges, DEFAULT_DEFINITION)
      const result = await runFlow({ dsl })
      setRunResult(JSON.stringify(result, null, 2))
    } catch (err) {
      setRunError(err instanceof Error ? err.message : String(err))
    } finally {
      setRunLoading(false)
    }
  }, [nodes, edges])

  return (
    <div className="h-screen w-screen flex flex-col bg-white overflow-hidden">
      {/* Top bar */}
      <header className="flex items-center justify-between px-5 py-3 border-b border-gray-200 bg-white shadow-sm z-10">
        <div className="flex items-center gap-3">
          <span className="text-xl font-bold text-blue-600">flowjs</span>
          <span className="text-gray-300">|</span>
          {/* View switcher */}
          <nav className="flex gap-1">
            <button
              onClick={() => setView('designer')}
              className={`text-sm px-3 py-1 rounded transition-colors ${
                view === 'designer'
                  ? 'bg-blue-50 text-blue-600 font-medium'
                  : 'text-gray-500 hover:text-gray-700'
              }`}
            >
              Process Designer
            </button>
            <button
              onClick={() => setView('history')}
              className={`text-sm px-3 py-1 rounded transition-colors ${
                view === 'history'
                  ? 'bg-blue-50 text-blue-600 font-medium'
                  : 'text-gray-500 hover:text-gray-700'
              }`}
            >
              Execution History
            </button>
          </nav>
        </div>
        {view === 'designer' && (
          <div className="flex items-center gap-2">
            <button
              onClick={handleRunFlow}
              disabled={runLoading}
              className="flex items-center gap-2 px-4 py-2 bg-green-600 text-white text-sm font-medium rounded-lg hover:bg-green-700 disabled:opacity-50 transition-colors"
            >
              <span>{runLoading ? '⏳' : '▶'}</span>
              {runLoading ? 'Running…' : 'Run Flow'}
            </button>
            <ExportButton nodes={nodes} edges={edges} definition={DEFAULT_DEFINITION} />
          </div>
        )}
      </header>

      {/* Main content */}
      <main className="flex flex-1 overflow-hidden">
        {view === 'designer' ? (
          <>
            {/* Left: Node Palette */}
            <NodePalette />

            {/* Center: React Flow Canvas */}
            <DesignerCanvas
              onSelectionChange={setSelectedNode}
              onNodesChange={setNodes}
              onEdgesChange={setEdges}
            />

            {/* Right: Config Panel */}
            <ConfigPanel selectedNode={selectedNode} onNodeUpdate={handleNodeUpdate} allNodes={nodes as DesignerNode[]} />
          </>
        ) : (
          <ExecutionHistory />
        )}
      </main>

      {/* Run Flow result toast */}
      {(runResult || runError) && (
        <div
          role="alert"
          aria-live="polite"
          className="fixed bottom-4 right-4 z-50 w-96 max-h-72 flex flex-col rounded-lg shadow-xl overflow-hidden border"
        >
          <div className={`flex items-center justify-between px-3 py-2 text-xs font-semibold text-white ${runError ? 'bg-red-600' : 'bg-green-600'}`}>
            <span>{runError ? '✕ Flow execution failed' : '✓ Flow executed successfully'}</span>
            <button onClick={() => { setRunResult(null); setRunError(null) }} className="hover:opacity-75" aria-label="Dismiss">×</button>
          </div>
          <pre className="flex-1 overflow-auto p-3 text-[10px] font-mono bg-gray-900 text-green-400">
            {runError ?? runResult}
          </pre>
        </div>
      )}
    </div>
  )
}
