import { getBezierPath, EdgeLabelRenderer, BaseEdge, type EdgeProps } from '@xyflow/react'

const EDGE_COLORS = {
  success:     { bg: 'bg-green-100',  text: 'text-green-700',  stroke: '#22c55e' },
  error:       { bg: 'bg-red-100',    text: 'text-red-700',    stroke: '#ef4444' },
  condition:   { bg: 'bg-blue-100',   text: 'text-blue-700',   stroke: '#3b82f6' },
  nocondition: { bg: 'bg-gray-100',   text: 'text-gray-600',   stroke: '#9ca3af' },
} as const

type TransitionTypeKey = keyof typeof EDGE_COLORS

export function TransitionEdge({
  id, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, data, selected,
}: EdgeProps) {
  const edgeData = data as { transitionType?: string; condition?: string } | undefined
  const transitionType = (edgeData?.transitionType ?? 'success') as TransitionTypeKey
  const colors = EDGE_COLORS[transitionType] ?? EDGE_COLORS.success

  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX, sourceY, sourcePosition, targetX, targetY, targetPosition,
  })

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        style={{ stroke: colors.stroke, strokeWidth: selected ? 2.5 : 1.5 }}
      />
      <EdgeLabelRenderer>
        <div
          style={{
            transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)`,
            pointerEvents: 'all',
            position: 'absolute',
          }}
          className={`text-[9px] font-semibold px-1.5 py-0.5 rounded-full ${colors.bg} ${colors.text} border border-current cursor-pointer nodrag nopan`}
          title={edgeData?.condition ? `Condition: ${edgeData.condition}` : 'Click to cycle transition type'}
        >
          {transitionType}
        </div>
      </EdgeLabelRenderer>
    </>
  )
}
