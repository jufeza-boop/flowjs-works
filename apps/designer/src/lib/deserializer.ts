import type { Node, Edge } from '@xyflow/react'
import type { FlowDSL } from '../types/dsl'
import type { NodeData } from '../types/designer'

/** Horizontal spacing between nodes when auto-generating positions. */
const NODE_SPACING_X = 250
/** Fixed y-coordinate for all nodes in the auto-layout. */
const NODE_Y = 200
/** x-coordinate of the trigger node. */
const TRIGGER_X = 80

/**
 * Converts a FlowDSL document back into React Flow nodes and edges.
 *
 * Since the DSL stores execution logic only (no canvas layout), positions are
 * auto-generated in a simple left-to-right arrangement:
 *   - Trigger node at x=80
 *   - Activity nodes spaced 250 px apart to the right
 *
 * Edges are reconstructed from:
 *   1. `dsl.transitions` — carry explicit type (success/error/condition/nocondition) and
 *      optional condition expression.
 *   2. `node.next[]` — implicit sequential "success" edges that do not already have an
 *      explicit transition entry.
 */
export function deserializeGraph(dsl: FlowDSL): { nodes: Node<NodeData>[]; edges: Edge[] } {
  const rfNodes: Node<NodeData>[] = []
  const rfEdges: Edge[] = []

  // ── Trigger node ──────────────────────────────────────────────────────────
  rfNodes.push({
    id: dsl.trigger.id,
    type: 'triggerNode',
    position: { x: TRIGGER_X, y: NODE_Y },
    data: {
      nodeKind: 'trigger',
      id: dsl.trigger.id,
      type: dsl.trigger.type,
      config: dsl.trigger.config,
    } as unknown as NodeData,
  })

  // ── Activity nodes ────────────────────────────────────────────────────────
  dsl.nodes.forEach((node, index) => {
    rfNodes.push({
      id: node.id,
      type: 'activityNode',
      position: { x: TRIGGER_X + (index + 1) * NODE_SPACING_X, y: NODE_Y },
      data: {
        nodeKind: 'process',
        ...node,
      } as unknown as NodeData,
    })
  })

  // ── Edges from explicit transitions ──────────────────────────────────────
  const coveredPairs = new Set<string>()
  dsl.transitions.forEach((t, i) => {
    rfEdges.push({
      id: `e_trans_${t.from}_${t.to}_${i}`,
      source: t.from,
      target: t.to,
      type: 'transitionEdge',
      data: {
        transitionType: t.type,
        ...(t.condition ? { condition: t.condition } : {}),
      },
    })
    coveredPairs.add(`${t.from}→${t.to}`)
  })

  // ── Edges from node.next[] (sequential / no-transition mode) ─────────────
  dsl.nodes.forEach((node) => {
    node.next?.forEach((targetId) => {
      const key = `${node.id}→${targetId}`
      if (!coveredPairs.has(key)) {
        rfEdges.push({
          id: `e_next_${node.id}_${targetId}`,
          source: node.id,
          target: targetId,
          type: 'transitionEdge',
          data: { transitionType: 'success' },
        })
      }
    })
  })

  return { nodes: rfNodes, edges: rfEdges }
}
