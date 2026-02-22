import { memo, useCallback } from 'react'
import { Handle, Position, useReactFlow, type NodeProps } from '@xyflow/react'
import type { NodeData } from '../../types/designer'

/** Custom node for SQL Insert activity */
const SqlInsertNode = memo(({ id, data, selected }: NodeProps) => {
  const { setNodes, setEdges } = useReactFlow()
  const handleDelete = useCallback((e: React.MouseEvent) => {
    e.stopPropagation()
    setNodes((nds) => nds.filter((n) => n.id !== id))
    setEdges((eds) => eds.filter((edge) => edge.source !== id && edge.target !== id))
  }, [id, setNodes, setEdges])
  const nodeData = data as unknown as NodeData
  const processData = nodeData.nodeKind === 'process' ? nodeData : null

  return (
    <div
      className={`rounded-lg border-2 bg-white shadow-md min-w-[160px] ${selected ? 'border-blue-500' : 'border-orange-500'
        }`}
    >
      <div className="flex items-center justify-between bg-orange-500 px-3 py-2 rounded-t-md">
        <div className="flex items-center gap-2">
          <span className="text-white text-sm">🗄️</span>
          <span className="text-white text-xs font-semibold uppercase tracking-wide">SQL Insert</span>
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
          {processData?.id ?? 'sql_node'}
        </p>
        {processData?.config && 'table' in processData.config && (
          <p className="text-xs text-gray-400 truncate">
            {(processData.config as Record<string, string>).datasource}.{(processData.config as Record<string, string>).table}
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
