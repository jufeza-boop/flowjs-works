import { memo, useCallback } from 'react'
import { Handle, Position, useReactFlow, type NodeProps } from '@xyflow/react'
import type { NodeData } from '../../types/designer'

const NODE_DISPLAY: Record<string, { icon: string; color: string; label: string; border: string }> = {
  http:      { icon: '🌐', color: 'bg-blue-400',    label: 'HTTP',      border: 'border-blue-400' },
  sftp:      { icon: '📂', color: 'bg-teal-500',    label: 'SFTP',      border: 'border-teal-500' },
  s3:        { icon: '☁️', color: 'bg-yellow-500',  label: 'S3',        border: 'border-yellow-500' },
  smb:       { icon: '🗂️', color: 'bg-gray-500',    label: 'SMB',       border: 'border-gray-500' },
  mail:      { icon: '📧', color: 'bg-red-400',     label: 'Mail',      border: 'border-red-400' },
  rabbitmq:  { icon: '🐇', color: 'bg-orange-400',  label: 'RabbitMQ',  border: 'border-orange-400' },
  sql:       { icon: '🗄️', color: 'bg-orange-500',  label: 'SQL',       border: 'border-orange-500' },
  code:      { icon: '📜', color: 'bg-purple-500',  label: 'Code',      border: 'border-purple-500' },
  log:       { icon: '📋', color: 'bg-gray-400',    label: 'Log',       border: 'border-gray-400' },
  transform: { icon: '🔄', color: 'bg-indigo-500',  label: 'Transform', border: 'border-indigo-500' },
  file:      { icon: '📄', color: 'bg-lime-500',    label: 'File',      border: 'border-lime-500' },
}

const ActivityNode = memo(({ id, data, selected }: NodeProps) => {
  const { setNodes, setEdges } = useReactFlow()
  const handleDelete = useCallback((e: React.MouseEvent) => {
    e.stopPropagation()
    setNodes((nds) => nds.filter((n) => n.id !== id))
    setEdges((eds) => eds.filter((edge) => edge.source !== id && edge.target !== id))
  }, [id, setNodes, setEdges])

  const nodeData = data as unknown as NodeData
  const processData = nodeData.nodeKind === 'process' ? nodeData : null
  const nodeType = (processData?.type as string) ?? 'http'
  const display = NODE_DISPLAY[nodeType] ?? { icon: '⚙️', color: 'bg-gray-500', label: nodeType, border: 'border-gray-500' }

  return (
    <div className={`rounded-lg border-2 bg-white shadow-md min-w-[160px] ${selected ? 'border-blue-500' : display.border}`}>
      <div className={`flex items-center justify-between ${display.color} px-3 py-2 rounded-t-md`}>
        <div className="flex items-center gap-2">
          <span className="text-white text-sm">{display.icon}</span>
          <span className="text-white text-xs font-semibold uppercase tracking-wide">{display.label}</span>
        </div>
        <button
          onClick={handleDelete}
          className="text-white/80 hover:text-red-200 transition-colors"
          title="Delete Node"
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div className="px-3 py-2">
        <p className="text-xs font-medium text-gray-700 truncate">{id}</p>
        {processData?.description && (
          <p className="text-xs text-gray-400 truncate">{processData.description as string}</p>
        )}
      </div>
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
    </div>
  )
})

ActivityNode.displayName = 'ActivityNode'
export default ActivityNode
