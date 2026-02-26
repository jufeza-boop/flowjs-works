import { useState, useCallback, useEffect } from 'react'
import type { Node, Edge } from '@xyflow/react'
import { ReactFlowProvider } from '@xyflow/react'
import { NodePalette } from './components/NodePalette'
import { DesignerCanvas } from './components/DesignerCanvas'
import { ConfigPanel } from './components/ConfigPanel'
import { ExportButton } from './components/ExportButton'
import { ExecutionHistory } from './components/ExecutionHistory'
import { DebugPanel } from './components/DebugPanel'
import { SecretsManager } from './components/SecretsManager'
import { ProcessManager } from './components/ProcessManager'
import { serializeGraph } from './lib/serializer'
import { runFlow } from './lib/api'
import type { RunFlowResponse } from './lib/api'
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

type View = 'designer' | 'history' | 'secrets' | 'deployments'

/** Root application — three-column designer layout or execution history view */
export default function App() {
  const [view, setView] = useState<View>('designer')
  const [selectedNode, setSelectedNode] = useState<DesignerNode | null>(null)
  const [nodes, setNodes] = useState<Node<NodeData>[]>([])
  const [edges, setEdges] = useState<Edge[]>([])
  const [runLoading, setRunLoading] = useState(false)
  const [runFlowResult, setRunFlowResult] = useState<RunFlowResponse | null>(null)
  const [runRawResult, setRunRawResult] = useState<string | null>(null)
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

  // Clear selection if the node is deleted from the canvas
  useEffect(() => {
    if (selectedNode && !nodes.some((n) => n.id === selectedNode.id)) {
      setSelectedNode(null)
    }
  }, [nodes, selectedNode])

  const handleRunFlow = useCallback(async () => {
    setRunLoading(true)
    setRunFlowResult(null)
    setRunRawResult(null)
    setRunError(null)
    try {
      const dsl = serializeGraph(nodes, edges, DEFAULT_DEFINITION)
      const result = await runFlow({ dsl })
      // Normalize the engine's `nodes` map into a flat `node_results` array for DebugPanel.
      // The engine returns nodes as Record<nodeId, {output, status}> — never as node_results.
      if (!result.node_results?.length && result.nodes) {
        result.node_results = Object.entries(result.nodes).map(([nodeId, nodeData]) => ({
          node_id: nodeId,
          status: (nodeData.status as string) ?? 'unknown',
          output: nodeData.output as Record<string, unknown> | undefined,
        }))
      }
      setRunFlowResult(result)
      // Fallback raw text only when the nodes map is also empty
      if (!result.node_results?.length) {
        setRunRawResult(JSON.stringify(result, null, 2))
      }
    } catch (err) {
      setRunError(err instanceof Error ? err.message : String(err))
    } finally {
      setRunLoading(false)
    }
  }, [nodes, edges])

  const handleCloseDebug = useCallback(() => {
    setRunFlowResult(null)
    setRunRawResult(null)
    setRunError(null)
  }, [])

  return (
    <div className="h-screen w-screen flex flex-col bg-white overflow-hidden">
      {/* Top bar */}
      <header className="flex items-center justify-between px-5 py-3 border-b border-gray-200 bg-white shadow-sm z-10">
        <div className="flex items-center gap-3">
          <span className="text-xl font-bold text-blue-600">flowjs</span>
          <span className="text-gray-300">|</span>
          <nav className="flex gap-1">
            <button
              onClick={() => setView('designer')}
              className={`text-sm px-3 py-1 rounded transition-colors ${view === 'designer'
                ? 'bg-blue-50 text-blue-600 font-medium'
                : 'text-gray-500 hover:text-gray-700'
                }`}
            >
              Process Designer
            </button>
            <button
              onClick={() => setView('history')}
              className={`text-sm px-3 py-1 rounded transition-colors ${view === 'history'
                ? 'bg-blue-50 text-blue-600 font-medium'
                : 'text-gray-500 hover:text-gray-700'
                }`}
            >
              Execution History
            </button>
            <button
              onClick={() => setView('secrets')}
              className={`text-sm px-3 py-1 rounded transition-colors ${view === 'secrets'
                ? 'bg-blue-50 text-blue-600 font-medium'
                : 'text-gray-500 hover:text-gray-700'
                }`}
            >
              🔐 Secrets
            </button>
            <button
              onClick={() => setView('deployments')}
              className={`text-sm px-3 py-1 rounded transition-colors ${view === 'deployments'
                ? 'bg-blue-50 text-blue-600 font-medium'
                : 'text-gray-500 hover:text-gray-700'
                }`}
            >
              🚀 Deployments
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
            <ReactFlowProvider>
              <NodePalette />
              <DesignerCanvas
                onSelectionChange={setSelectedNode}
                onNodesChange={setNodes}
                onEdgesChange={setEdges}
              />
              <ConfigPanel selectedNode={selectedNode} onNodeUpdate={handleNodeUpdate} allNodes={nodes as DesignerNode[]} />
            </ReactFlowProvider>
          </>
        ) : view === 'history' ? (
          <ExecutionHistory />
        ) : view === 'deployments' ? (
          <ProcessManager nodes={nodes} edges={edges} definition={DEFAULT_DEFINITION} />
        ) : (
          <SecretsManager />
        )}
      </main>

      {/* Run error toast */}
      {runError && (
        <div
          role="alert"
          aria-live="polite"
          className="fixed bottom-4 right-4 z-50 w-96 max-h-72 flex flex-col rounded-lg shadow-xl overflow-hidden border"
        >
          <div className="flex items-center justify-between px-3 py-2 text-xs font-semibold text-white bg-red-600">
            <span>✕ Flow execution failed</span>
            <button onClick={handleCloseDebug} className="hover:opacity-75" aria-label="Dismiss">×</button>
          </div>
          <pre className="flex-1 overflow-auto p-3 text-[10px] font-mono bg-gray-900 text-red-400">{runError}</pre>
        </div>
      )}

      {/* Debug panel for successful runs */}
      {!runError && (
        <DebugPanel result={runFlowResult} rawResult={runRawResult} onClose={handleCloseDebug} />
      )}
    </div>
  )
}
