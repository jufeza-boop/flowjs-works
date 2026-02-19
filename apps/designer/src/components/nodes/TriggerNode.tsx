import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import type { NodeData } from '../../types/designer'

/** Custom node for HTTP Webhook Trigger */
const TriggerNode = memo(({ data, selected }: NodeProps) => {
  const nodeData = data as unknown as NodeData
  const triggerData = nodeData.nodeKind === 'trigger' ? nodeData : null

  return (
    <div
      className={`rounded-lg border-2 bg-white shadow-md min-w-[160px] ${
        selected ? 'border-blue-500' : 'border-green-500'
      }`}
    >
      <div className="flex items-center gap-2 bg-green-500 px-3 py-2 rounded-t-md">
        <span className="text-white text-sm">⚡</span>
        <span className="text-white text-xs font-semibold uppercase tracking-wide">Trigger</span>
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
