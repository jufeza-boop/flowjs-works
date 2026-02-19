import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import type { NodeData } from '../../types/designer'

/** Custom node for HTTP Request activity */
const HttpRequestNode = memo(({ data, selected }: NodeProps) => {
  const nodeData = data as unknown as NodeData
  const processData = nodeData.nodeKind === 'process' ? nodeData : null

  return (
    <div
      className={`rounded-lg border-2 bg-white shadow-md min-w-[160px] ${
        selected ? 'border-blue-500' : 'border-blue-400'
      }`}
    >
      <div className="flex items-center gap-2 bg-blue-400 px-3 py-2 rounded-t-md">
        <span className="text-white text-sm">🌐</span>
        <span className="text-white text-xs font-semibold uppercase tracking-wide">HTTP Request</span>
      </div>
      <div className="px-3 py-2">
        <p className="text-xs font-medium text-gray-700 truncate">
          {processData?.id ?? 'http_node'}
        </p>
        {processData?.config?.url && (
          <p className="text-xs text-gray-400 truncate">{processData.config.url}</p>
        )}
      </div>
      <Handle type="target" position={Position.Left} className="!bg-blue-400" />
      <Handle type="source" position={Position.Right} className="!bg-blue-400" />
    </div>
  )
})

HttpRequestNode.displayName = 'HttpRequestNode'
export default HttpRequestNode
