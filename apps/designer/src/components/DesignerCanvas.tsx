import { useCallback, useRef, useState, useEffect, type DragEvent } from 'react'
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
import ActivityNode from './nodes/ActivityNode'
import { TransitionEdge } from './TransitionEdge'
import type { NodeData, NodeTypeKey, PaletteTriggerKey, DesignerNode, TransitionTypeEdge } from '../types/designer'
import type { TriggerType, NodeType } from '../types/dsl'

const nodeTypes = {
  triggerNode: TriggerNode,
  activityNode: ActivityNode,
}

const edgeTypes = {
  transitionEdge: TransitionEdge,
}

const TYPE_MAP: Record<NodeTypeKey, string> = {
  trg_cron: 'triggerNode', trg_rest: 'triggerNode', trg_soap: 'triggerNode',
  trg_rabbitmq: 'triggerNode', trg_mcp: 'triggerNode', trg_manual: 'triggerNode',
  http: 'activityNode', sftp: 'activityNode', s3: 'activityNode', smb: 'activityNode',
  mail: 'activityNode', rabbitmq: 'activityNode', sql: 'activityNode', code: 'activityNode',
  log: 'activityNode', transform: 'activityNode', file: 'activityNode',
}

const TRANSITION_TYPES: TransitionTypeEdge[] = ['success', 'error', 'condition', 'nocondition']

function buildDefaultData(type: NodeTypeKey, id: string): NodeData {
  const triggerMap: Record<PaletteTriggerKey, { type: TriggerType; config: object }> = {
    trg_cron:     { type: 'cron',     config: { expression: '0 * * * *' } },
    trg_rest:     { type: 'rest',     config: { path: '/v1/flow', method: 'POST' } },
    trg_soap:     { type: 'soap',     config: { path: '/ws/flow' } },
    trg_rabbitmq: { type: 'rabbitmq', config: { url_amqp: 'amqp://localhost', queue: 'flow-queue' } },
    trg_mcp:      { type: 'mcp',      config: { version: '1.0' } },
    trg_manual:   { type: 'manual',   config: {} as never },
  }
  if (type in triggerMap) {
    const t = triggerMap[type as PaletteTriggerKey]
    return { nodeKind: 'trigger', id, type: t.type, config: t.config } as unknown as NodeData
  }
  const baseProcess = { nodeKind: 'process' as const, id }
  const activityDefaults: Record<NodeType, object> = {
    http:      { url: 'https://api.example.com', method: 'GET' },
    sftp:      { server: 'sftp.example.com', port: 22, folder: '/files', method: 'get' },
    s3:        { bucket: 'my-bucket', region: 'us-east-1', folder: '/', method: 'get' },
    smb:       { server: '\\\\server\\share', folder: '/files', method: 'get' },
    mail:      { host: 'smtp.example.com', port: 587, action: 'send' },
    rabbitmq:  { url_amqp: 'amqp://localhost', exchange: '', routing_key: 'flow.event' },
    sql:       { engine: 'postgres', host: 'localhost', port: 5432, database: 'mydb', query: 'SELECT 1' },
    code:      { script: 'export default (input) => input' },
    log:       { level: 'INFO', message: '' },
    transform: { transform_type: 'json2csv' },
    file:      { operation: 'read', path: '/tmp/file.txt' },
  }
  return {
    ...baseProcess,
    type: type as NodeType,
    description: '',
    config: activityDefaults[type as NodeType] ?? {},
  } as unknown as NodeData
}

const initialNodes: Node<NodeData>[] = [
  {
    id: 'trg_01',
    type: 'triggerNode',
    position: { x: 80, y: 160 },
    data: {
      nodeKind: 'trigger',
      id: 'trg_01',
      type: 'rest',
      config: { path: '/v1/flow', method: 'POST' },
    },
  },
]

interface DesignerCanvasProps {
  onSelectionChange: (node: DesignerNode | null) => void
  onNodesChange: (nodes: Node<NodeData>[]) => void
  onEdgesChange: (edges: Edge[]) => void
  /** When set, the canvas replaces its current graph with these nodes/edges. */
  graphToLoad?: { nodes: Node<NodeData>[]; edges: Edge[] } | null
  /** Called once the graph has been loaded into the canvas state. */
  onGraphLoaded?: () => void
}

let nodeCounter = 1

/** React Flow canvas with drag-drop, connection and edge cycling */
export function DesignerCanvas({ onSelectionChange, onNodesChange, onEdgesChange, graphToLoad, onGraphLoaded }: DesignerCanvasProps) {
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

  // Load an external graph when graphToLoad is provided (e.g., after "Edit" in Deployments)
  useEffect(() => {
    if (!graphToLoad) return
    setNodes(graphToLoad.nodes)
    setEdges(graphToLoad.edges)
    propagateNodes(graphToLoad.nodes)
    propagateEdges(graphToLoad.edges)
    onGraphLoaded?.()
  }, [graphToLoad, setNodes, setEdges, propagateNodes, propagateEdges, onGraphLoaded])

  const onConnect = useCallback(
    (connection: Connection) => {
      setEdges((eds) => {
        const newEdges = addEdge(
          { ...connection, type: 'transitionEdge', data: { transitionType: 'success' } },
          eds,
        )
        propagateEdges(newEdges)
        return newEdges
      })
    },
    [setEdges, propagateEdges],
  )

  const onEdgeClick = useCallback(
    (_: React.MouseEvent, edge: Edge) => {
      setEdges((eds) =>
        eds.map((e) => {
          if (e.id !== edge.id) return e
          const current = (e.data as { transitionType?: string } | undefined)?.transitionType ?? 'success'
          const idx = TRANSITION_TYPES.indexOf(current as TransitionTypeEdge)
          const next = TRANSITION_TYPES[(idx + 1) % TRANSITION_TYPES.length]
          return { ...e, data: { ...(e.data ?? {}), transitionType: next } }
        }),
      )
    },
    [setEdges],
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
        onEdgeClick={onEdgeClick}
        onInit={(instance) => setRfInstance(instance as unknown as ReactFlowInstance<Node<NodeData>, Edge>)}
        onDrop={onDrop}
        onDragOver={onDragOver}
        onNodeClick={onNodeClick}
        onPaneClick={onPaneClick}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        defaultEdgeOptions={{ type: 'transitionEdge', data: { transitionType: 'success' } }}
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
