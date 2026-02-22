import { memo, useCallback } from 'react'
import { Handle, Position, useReactFlow, type NodeProps } from '@xyflow/react'
import type { NodeData } from '../../types/designer'

/** Custom node for HTTP Webhook Trigger */
const TriggerNode = memo(({ id, data, selected }: NodeProps) => {
  const { setNodes, setEdges } = useReactFlow()
  const handleDelete = useCallback((e: React.MouseEvent) => {
    e.stopPropagation()
    setNodes((nds) => nds.filter((n) => n.id !== id))
    setEdges((eds) => eds.filter((edge) => edge.source !== id && edge.target !== id))
  }, [id, setNodes, setEdges])
  const nodeData = data as unknown as NodeData
  const triggerData = nodeData.nodeKind === 'trigger' ? nodeData : null

  return (
    <div
      className={`rounded-lg border-2 bg-white shadow-md min-w-[160px] ${selected ? 'border-blue-500' : 'border-green-500'
        }`}
    >
      <div className="flex items-center justify-between bg-green-500 px-3 py-2 rounded-t-md">
        <div className="flex items-center gap-2">
          <span className="text-white text-sm">⚡</span>
          <span className="text-white text-xs font-semibold uppercase tracking-wide">Trigger</span>
        </div>
        <button
          onClick={handleDelete}
          className="text-white/80 hover:text-red-200 transition-colors"
          title="Delete Node"
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" /></svg>
        </button>
      </div>
      <div className="px-3 py-2">
        <p className="text-xs font-medium text-gray-700 truncate">
          {triggerData?.type ?? 'http_webhook'}
        </p>
        {triggerData?.config?.path && (
          <p className="text-xs text-gray-400 truncate">{triggerData.config.path}</p>
        )}
      </div>
      <Handle type="source" position={Position.Right} className="!bg-green-500" />
    </div>
  )
})

TriggerNode.displayName = 'TriggerNode'
export default TriggerNode
