import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import type { NodeData } from '../../types/designer'

/** Custom node for SQL Insert activity */
const SqlInsertNode = memo(({ data, selected }: NodeProps) => {
  const nodeData = data as unknown as NodeData
  const processData = nodeData.nodeKind === 'process' ? nodeData : null

  return (
    <div
      className={`rounded-lg border-2 bg-white shadow-md min-w-[160px] ${
        selected ? 'border-blue-500' : 'border-orange-500'
      }`}
    >
      <div className="flex items-center gap-2 bg-orange-500 px-3 py-2 rounded-t-md">
        <span className="text-white text-sm">🗄️</span>
        <span className="text-white text-xs font-semibold uppercase tracking-wide">SQL Insert</span>
      </div>
      <div className="px-3 py-2">
        <p className="text-xs font-medium text-gray-700 truncate">
          {processData?.id ?? 'sql_node'}
        </p>
        {processData?.config?.table && (
          <p className="text-xs text-gray-400 truncate">
            {processData.config.datasource}.{processData.config.table}
          </p>
        )}
      </div>
      <Handle type="target" position={Position.Left} className="!bg-orange-500" />
      <Handle type="source" position={Position.Right} className="!bg-orange-500" />
    </div>
  )
})

SqlInsertNode.displayName = 'SqlInsertNode'
export default SqlInsertNode
