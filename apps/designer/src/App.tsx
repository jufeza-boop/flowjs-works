import { useState, useCallback } from 'react'
import type { Node, Edge } from '@xyflow/react'
import { NodePalette } from './components/NodePalette'
import { DesignerCanvas } from './components/DesignerCanvas'
import { ConfigPanel } from './components/ConfigPanel'
import { ExportButton } from './components/ExportButton'
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

/** Root application — three-column layout: Palette | Canvas | Config */
export default function App() {
  const [selectedNode, setSelectedNode] = useState<DesignerNode | null>(null)
  const [nodes, setNodes] = useState<Node<NodeData>[]>([])
  const [edges, setEdges] = useState<Edge[]>([])

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

  return (
    <div className="h-screen w-screen flex flex-col bg-white overflow-hidden">
      {/* Top bar */}
      <header className="flex items-center justify-between px-5 py-3 border-b border-gray-200 bg-white shadow-sm z-10">
        <div className="flex items-center gap-3">
          <span className="text-xl font-bold text-blue-600">flowjs</span>
          <span className="text-gray-300">|</span>
          <span className="text-sm text-gray-500">Process Designer</span>
        </div>
        <ExportButton nodes={nodes} edges={edges} definition={DEFAULT_DEFINITION} />
      </header>

      {/* Three-column body */}
      <main className="flex flex-1 overflow-hidden">
        {/* Left: Node Palette */}
        <NodePalette />

        {/* Center: React Flow Canvas */}
        <DesignerCanvas
          onSelectionChange={setSelectedNode}
          onNodesChange={setNodes}
          onEdgesChange={setEdges}
        />

        {/* Right: Config Panel */}
        <ConfigPanel selectedNode={selectedNode} onNodeUpdate={handleNodeUpdate} />
      </main>
    </div>
  )
}
