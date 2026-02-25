import type { Node, Edge } from '@xyflow/react'
import type { FlowDSL, FlowNode, FlowTrigger, FlowTransition, FlowDefinition, TransitionType } from '../types/dsl'
import type { NodeData } from '../types/designer'

/** Default flow definition metadata */
const DEFAULT_DEFINITION: FlowDefinition = {
  id: 'new-flow',
  version: '1.0.0',
  name: 'New Flow',
  description: '',
  settings: {
    persistence: 'full',
    timeout: 30000,
    error_strategy: 'stop_and_rollback',
  },
}

/**
 * Serializes a React Flow graph (nodes + edges) into the flowjs-works JSON DSL format.
 * Edges with transitionType or condition data become transitions; plain edges without
 * either are excluded to maintain backward compatibility.
 */
export function serializeGraph(
  rfNodes: Node<NodeData>[],
  rfEdges: Edge[],
  definition: FlowDefinition = DEFAULT_DEFINITION,
): FlowDSL {
  const triggerRfNode = rfNodes.find((n) => n.data.nodeKind === 'trigger')
  const processRfNodes = rfNodes.filter((n) => n.data.nodeKind === 'process')

  let trigger: FlowTrigger
  if (triggerRfNode && triggerRfNode.data.nodeKind === 'trigger') {
    const { nodeKind: _k, ...rest } = triggerRfNode.data
    trigger = rest as FlowTrigger
  } else {
    trigger = {
      id: 'trg_01',
      type: 'rest',
      config: { path: '/v1/flow', method: 'POST' },
    }
  }

  const nextMap = new Map<string, string[]>()
  for (const edge of rfEdges) {
    const list = nextMap.get(edge.source) ?? []
    list.push(edge.target)
    nextMap.set(edge.source, list)
  }

  const nodes: FlowNode[] = processRfNodes.map((rfNode) => {
    const { nodeKind: _k, ...rest } = rfNode.data as NodeData & { nodeKind: 'process' }
    const node = rest as FlowNode
    const next = nextMap.get(rfNode.id)
    return {
      ...node,
      ...(next && next.length > 0 ? { next } : {}),
    }
  })

  // Include edges that have explicit transitionType OR condition metadata
  const transitions: FlowTransition[] = rfEdges
    .filter((e) => {
      const edgeData = e.data as { transitionType?: string; condition?: string } | undefined
      return edgeData?.transitionType !== undefined || edgeData?.condition !== undefined
    })
    .map((e) => {
      const edgeData = e.data as { transitionType?: string; condition?: string } | undefined
      const type = (edgeData?.transitionType ?? 'success') as TransitionType
      const result: FlowTransition = { from: e.source, to: e.target, type }
      if (edgeData?.condition) {
        result.condition = edgeData.condition
      }
      return result
    })

  return { definition, trigger, nodes, transitions }
}
