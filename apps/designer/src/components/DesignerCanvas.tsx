import { useCallback, useRef, useState, type DragEvent } from 'react'
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  addEdge,
  useNodesState,
  useEdgesState,
  type Connection,
  type Node,
  type Edge,
  type ReactFlowInstance,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'

import TriggerNode from './nodes/TriggerNode'
import ScriptNode from './nodes/ScriptNode'
import HttpRequestNode from './nodes/HttpRequestNode'
import SqlInsertNode from './nodes/SqlInsertNode'
import type { NodeData, NodeTypeKey, DesignerNode } from '../types/designer'

/** Registered custom node types for React Flow */
const nodeTypes = {
  triggerNode: TriggerNode,
  scriptNode: ScriptNode,
  httpRequestNode: HttpRequestNode,
  sqlInsertNode: SqlInsertNode,
}

/** Maps palette type key to the React Flow node type string */
const TYPE_MAP: Record<NodeTypeKey, string> = {
  trigger: 'triggerNode',
  script_ts: 'scriptNode',
  http_request: 'httpRequestNode',
  sql_insert: 'sqlInsertNode',
}

/** Builds default node data for a given node type key */
function buildDefaultData(type: NodeTypeKey, id: string): NodeData {
  if (type === 'trigger') {
    return {
      nodeKind: 'trigger',
      id,
      type: 'http_webhook',
      config: { path: '/v1/flow', method: 'POST' },
    }
  }
  const baseProcess = { nodeKind: 'process' as const, id }
  switch (type) {
    case 'script_ts':
      return { ...baseProcess, type: 'script_ts', description: 'Script node', script: 'export default (input) => input' }
    case 'http_request':
      return { ...baseProcess, type: 'http_request', description: 'HTTP call', config: { url: 'https://api.example.com', method: 'GET' } }
    case 'sql_insert':
      return { ...baseProcess, type: 'sql_insert', description: 'DB insert', config: { datasource: 'postgres_main', table: 'table_name' } }
  }
}

const initialNodes: Node<NodeData>[] = [
  {
    id: 'trg_01',
    type: 'triggerNode',
    position: { x: 80, y: 160 },
    data: {
      nodeKind: 'trigger',
      id: 'trg_01',
      type: 'http_webhook',
      config: { path: '/v1/flow', method: 'POST' },
    },
  },
]

interface DesignerCanvasProps {
  onSelectionChange: (node: DesignerNode | null) => void
  onNodesChange: (nodes: Node<NodeData>[]) => void
  onEdgesChange: (edges: Edge[]) => void
}

let nodeCounter = 1

/** Main React Flow canvas with drag-and-drop node creation */
export function DesignerCanvas({ onSelectionChange, onNodesChange, onEdgesChange }: DesignerCanvasProps) {
  const reactFlowWrapper = useRef<HTMLDivElement>(null)
  const [nodes, setNodes, handleNodesChange] = useNodesState<Node<NodeData>>(initialNodes)
  const [edges, setEdges, handleEdgesChange] = useEdgesState<Edge>([])
  const [rfInstance, setRfInstance] = useState<ReactFlowInstance<Node<NodeData>, Edge> | null>(null)

  const propagateNodes = useCallback((updatedNodes: Node<NodeData>[]) => {
    onNodesChange(updatedNodes)
  }, [onNodesChange])

  const propagateEdges = useCallback((updatedEdges: Edge[]) => {
    onEdgesChange(updatedEdges)
  }, [onEdgesChange])

  const onConnect = useCallback(
    (connection: Connection) => {
      setEdges((eds) => {
        const newEdges = addEdge(connection, eds)
        propagateEdges(newEdges)
        return newEdges
      })
    },
    [setEdges, propagateEdges],
  )

  const onEdgesChangeWrapped = useCallback(
    (changes: Parameters<typeof handleEdgesChange>[0]) => {
      handleEdgesChange(changes)
      setEdges((eds) => {
        propagateEdges(eds)
        return eds
      })
    },
    [handleEdgesChange, setEdges, propagateEdges],
  )

  const onNodesChangeWrapped = useCallback(
    (changes: Parameters<typeof handleNodesChange>[0]) => {
      handleNodesChange(changes)
      setNodes((nds) => {
        propagateNodes(nds)
        return nds
      })
    },
    [handleNodesChange, setNodes, propagateNodes],
  )

  const onDragOver = useCallback((event: DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    event.dataTransfer.dropEffect = 'move'
  }, [])

  const onDrop = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      event.preventDefault()
      const type = event.dataTransfer.getData('application/reactflow') as NodeTypeKey
      if (!type || !reactFlowWrapper.current) return

      const bounds = reactFlowWrapper.current.getBoundingClientRect()
      const position = rfInstance
        ? rfInstance.screenToFlowPosition({
          x: event.clientX - bounds.left,
          y: event.clientY - bounds.top,
        })
        : { x: event.clientX - bounds.left, y: event.clientY - bounds.top }

      const id = `${type}_${Date.now()}_${nodeCounter++}`
      const newNode: Node<NodeData> = {
        id,
        type: TYPE_MAP[type],
        position,
        data: buildDefaultData(type, id),
      }

      setNodes((nds) => {
        const updated = nds.concat(newNode)
        propagateNodes(updated)
        return updated
      })
    },
    [rfInstance, setNodes, propagateNodes],
  )

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      onSelectionChange(node as unknown as DesignerNode)
    },
    [onSelectionChange],
  )

  const onPaneClick = useCallback(() => {
    onSelectionChange(null)
  }, [onSelectionChange])

  return (
    <div ref={reactFlowWrapper} className="flex-1 h-full">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChangeWrapped}
        onEdgesChange={onEdgesChangeWrapped}
        onConnect={onConnect}
        onInit={(instance) => setRfInstance(instance as unknown as ReactFlowInstance<Node<NodeData>, Edge>)}
        onDrop={onDrop}
        onDragOver={onDragOver}
        onNodeClick={onNodeClick}
        onPaneClick={onPaneClick}
        nodeTypes={nodeTypes}
        deleteKeyCode={['Backspace', 'Delete']}
        fitView
        className="bg-gray-100"
      >
        <Background />
        <Controls />
        <MiniMap />
      </ReactFlow>
    </div>
  )
}
