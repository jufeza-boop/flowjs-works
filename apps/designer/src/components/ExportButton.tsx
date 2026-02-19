import { useState } from 'react'
import type { Node, Edge } from '@xyflow/react'
import { serializeGraph } from '../lib/serializer'
import type { NodeData } from '../types/designer'
import type { FlowDefinition } from '../types/dsl'

interface ExportButtonProps {
  nodes: Node<NodeData>[]
  edges: Edge[]
  definition: FlowDefinition
}

/** Button that serializes the graph and shows the DSL JSON */
export function ExportButton({ nodes, edges, definition }: ExportButtonProps) {
  const [showModal, setShowModal] = useState(false)
  const [json, setJson] = useState('')

  const handleExport = () => {
    const dsl = serializeGraph(nodes, edges, definition)
    setJson(JSON.stringify(dsl, null, 2))
    setShowModal(true)
  }

  return (
    <>
      <button
        onClick={handleExport}
        className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors"
      >
        <span>⬇</span>
        Export DSL
      </button>

      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-xl shadow-2xl w-[600px] max-h-[80vh] flex flex-col">
            <div className="flex items-center justify-between px-5 py-4 border-b">
              <h3 className="font-semibold text-gray-800">Flow DSL — JSON Export</h3>
              <button
                onClick={() => setShowModal(false)}
                className="text-gray-400 hover:text-gray-600 text-xl leading-none"
              >
                ×
              </button>
            </div>
            <pre className="flex-1 overflow-auto p-5 text-xs font-mono bg-gray-50 text-gray-800 rounded-b-xl">
              {json}
            </pre>
          </div>
        </div>
      )}
    </>
  )
}
