import type { Node, Edge } from '@xyflow/react'
import type { FlowNode, FlowTrigger, NodeType, TriggerType } from './dsl'

/** Data stored in each React Flow node — must satisfy Record<string, unknown> for @xyflow/react */
export type NodeData = Record<string, unknown> & (
  | (FlowTrigger & { nodeKind: 'trigger' })
  | (FlowNode & { nodeKind: 'process' })
)

/** React Flow node with typed data */
export type DesignerNode = Node<NodeData>

/** React Flow edge (standard) */
export type DesignerEdge = Edge

/** Node type keys used in the palette (trigger types + node types) */
export type NodeTypeKey = TriggerType | NodeType

/** Palette item definition */
export interface PaletteItem {
  type: NodeTypeKey
  label: string
  description: string
  icon: string
  color: string
}
