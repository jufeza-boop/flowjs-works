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
import { NameDialog } from './components/NameDialog'
import { serializeGraph } from './lib/serializer'
import { deserializeGraph } from './lib/deserializer'
import { slugify } from './lib/slugify'
import { runFlow, saveProcess, getProcess } from './lib/api'
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

/** Returns a blank React Flow graph (just the default trigger node). */
function makeInitialGraph(): { nodes: Node<NodeData>[]; edges: Edge[] } {
  return {
    nodes: [
      {
        id: 'trg_01',
        type: 'triggerNode',
        position: { x: 80, y: 160 },
        data: {
          nodeKind: 'trigger',
          id: 'trg_01',
          type: 'rest',
          config: { path: '/v1/flow', method: 'POST' },
        } as unknown as NodeData,
      },
    ],
    edges: [],
  }
}

type View = 'designer' | 'history' | 'secrets' | 'deployments'

/** Root application — three-column designer layout or execution history view */
export default function App() {
  const [view, setView] = useState<View>('designer')
  const [definition, setDefinition] = useState<FlowDefinition>(DEFAULT_DEFINITION)
  const [selectedNode, setSelectedNode] = useState<DesignerNode | null>(null)
  const [nodes, setNodes] = useState<Node<NodeData>[]>([])
  const [edges, setEdges] = useState<Edge[]>([])
  const [graphToLoad, setGraphToLoad] = useState<{ nodes: Node<NodeData>[]; edges: Edge[] } | null>(null)
  const [runLoading, setRunLoading] = useState(false)
  const [runFlowResult, setRunFlowResult] = useState<RunFlowResponse | null>(null)
  const [runRawResult, setRunRawResult] = useState<string | null>(null)
  const [runError, setRunError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [saveMsg, setSaveMsg] = useState<string | null>(null)
  /** Controls which name-prompt dialog is open: 'new', 'saveAs', or null (closed) */
  const [dialogMode, setDialogMode] = useState<'new' | 'saveAs' | null>(null)

  const isNameEmpty = !definition.name.trim()

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
      const dsl = serializeGraph(nodes, edges, definition)
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

  // ── Save current flow to the DB ──────────────────────────────────────────

  const handleSave = useCallback(async () => {
    setSaving(true)
    setSaveMsg(null)
    try {
      const dsl = serializeGraph(nodes, edges, definition)
      await saveProcess(dsl)
      setSaveMsg('Saved ✓')
      setTimeout(() => setSaveMsg(null), 3000)
    } catch (err) {
      setSaveMsg(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }, [nodes, edges, definition])

  // ── Load a saved process into the designer for editing ───────────────────

  const handleEditProcess = useCallback(async (processId: string) => {
    try {
      const record = await getProcess(processId)
      const loaded = deserializeGraph(record.dsl)
      setDefinition(record.dsl.definition)
      setGraphToLoad(loaded)
      setView('designer')
    } catch (err) {
      setRunError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  const handleGraphLoaded = useCallback(() => setGraphToLoad(null), [])

  // ── Flow name inline edit ─────────────────────────────────────────────────

  const handleDefinitionNameChange = useCallback((name: string) => {
    setDefinition((prev) => ({ ...prev, name }))
  }, [])

  // ── New flow ──────────────────────────────────────────────────────────────

  const handleNewConfirm = useCallback((name: string) => {
    setDialogMode(null)
    const id = `${slugify(name)}-${Date.now()}`
    setDefinition({ ...DEFAULT_DEFINITION, id, name })
    setGraphToLoad(makeInitialGraph())
    setSelectedNode(null)
    setSaveMsg(null)
  }, [])

  // ── Save As ───────────────────────────────────────────────────────────────

  const handleSaveAsConfirm = useCallback(async (name: string) => {
    setDialogMode(null)
    setSaving(true)
    setSaveMsg(null)
    try {
      const id = `${slugify(name)}-${Date.now()}`
      const newDefinition: FlowDefinition = { ...definition, id, name }
      const dsl = serializeGraph(nodes, edges, newDefinition)
      await saveProcess(dsl)
      // Switch the designer to the newly saved copy
      setDefinition(newDefinition)
      setSaveMsg('Saved ✓')
      setTimeout(() => setSaveMsg(null), 3000)
    } catch (err) {
      setSaveMsg(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }, [definition, nodes, edges])

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
            {/* Editable flow name */}
            <input
              type="text"
              value={definition.name}
              onChange={(e) => handleDefinitionNameChange(e.target.value)}
              className={`w-44 px-2 py-1 text-sm border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-300 text-gray-700 ${isNameEmpty ? 'border-red-300' : 'border-gray-200'}`}
              aria-label="Flow name"
              title="Flow name"
            />
            <button
              onClick={() => setDialogMode('new')}
              className="flex items-center gap-1 px-3 py-2 border border-gray-200 text-gray-600 text-sm rounded-lg hover:bg-gray-50 transition-colors"
              title="New flow"
            >
              ✚ New
            </button>
            <button
              onClick={() => void handleSave()}
              disabled={saving || isNameEmpty}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50 transition-colors"
            >
              <span>{saving ? '⏳' : '💾'}</span>
              {saving ? 'Saving…' : 'Save'}
            </button>
            <button
              onClick={() => setDialogMode('saveAs')}
              disabled={saving || isNameEmpty}
              className="flex items-center gap-1 px-3 py-2 border border-blue-200 text-blue-600 text-sm rounded-lg hover:bg-blue-50 disabled:opacity-50 transition-colors"
              title="Save a copy with a new name"
            >
              📋 Save As
            </button>
            {saveMsg && (
              <span className={`text-xs font-medium ${saveMsg.includes('✓') ? 'text-green-600' : 'text-red-500'}`}>
                {saveMsg}
              </span>
            )}
            <button
              onClick={handleRunFlow}
              disabled={runLoading}
              className="flex items-center gap-2 px-4 py-2 bg-green-600 text-white text-sm font-medium rounded-lg hover:bg-green-700 disabled:opacity-50 transition-colors"
            >
              <span>{runLoading ? '⏳' : '▶'}</span>
              {runLoading ? 'Running…' : 'Run Flow'}
            </button>
            <ExportButton nodes={nodes} edges={edges} definition={definition} />
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
                graphToLoad={graphToLoad}
                onGraphLoaded={handleGraphLoaded}
              />
              <ConfigPanel selectedNode={selectedNode} onNodeUpdate={handleNodeUpdate} allNodes={nodes as DesignerNode[]} />
            </ReactFlowProvider>
          </>
        ) : view === 'history' ? (
          <ExecutionHistory />
        ) : view === 'deployments' ? (
          <ProcessManager nodes={nodes} edges={edges} definition={definition} onEditProcess={handleEditProcess} />
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

      {/* Name dialogs for New / Save As */}
      {dialogMode === 'new' && (
        <NameDialog
          title="New Flow"
          defaultValue=""
          placeholder="e.g. Order Processing"
          confirmLabel="Create"
          onConfirm={handleNewConfirm}
          onCancel={() => setDialogMode(null)}
        />
      )}
      {dialogMode === 'saveAs' && (
        <NameDialog
          title="Save As — new copy"
          defaultValue={definition.name}
          placeholder="e.g. Order Processing v2"
          confirmLabel="Save Copy"
          onConfirm={(name) => void handleSaveAsConfirm(name)}
          onCancel={() => setDialogMode(null)}
        />
      )}
    </div>
  )
}
