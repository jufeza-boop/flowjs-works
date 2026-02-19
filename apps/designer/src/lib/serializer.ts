import type { Node, Edge } from '@xyflow/react'
import type { FlowDSL, FlowNode, FlowTrigger, FlowTransition, FlowDefinition } from '../types/dsl'
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
 * The trigger node is extracted separately; all other nodes become entries in 'nodes'.
 * Edges become 'transitions' with optional condition metadata.
 */
export function serializeGraph(
  rfNodes: Node<NodeData>[],
  rfEdges: Edge[],
  definition: FlowDefinition = DEFAULT_DEFINITION,
): FlowDSL {
  // Separate trigger from process nodes
  const triggerRfNode = rfNodes.find(
    (n) => n.data.nodeKind === 'trigger',
  )
  const processRfNodes = rfNodes.filter(
    (n) => n.data.nodeKind === 'process',
  )

  // Build trigger
  let trigger: FlowTrigger
  if (triggerRfNode && triggerRfNode.data.nodeKind === 'trigger') {
    const { nodeKind: _k, ...rest } = triggerRfNode.data
    trigger = rest as FlowTrigger
  } else {
    trigger = {
      id: 'trg_01',
      type: 'http_webhook',
      config: { path: '/v1/flow', method: 'POST' },
    }
  }

  // Build next map from edges: source -> [target, ...]
  const nextMap = new Map<string, string[]>()
  for (const edge of rfEdges) {
    const list = nextMap.get(edge.source) ?? []
    list.push(edge.target)
    nextMap.set(edge.source, list)
  }

  // Build process nodes
  const nodes: FlowNode[] = processRfNodes.map((rfNode) => {
    const { nodeKind: _k, ...rest } = rfNode.data as NodeData & { nodeKind: 'process' }
    const node = rest as FlowNode
    const next = nextMap.get(rfNode.id)
    return {
      ...node,
      ...(next && next.length > 0 ? { next } : {}),
    }
  })

  // Build transitions from edges that have condition metadata
  const transitions: FlowTransition[] = rfEdges
    .filter((e) => Boolean((e.data as { condition?: string } | undefined)?.condition))
    .map((e) => ({
      from: e.source,
      to: e.target,
      condition: (e.data as { condition: string }).condition,
    }))

  return { definition, trigger, nodes, transitions }
}
