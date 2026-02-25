import type { Node, Edge } from '@xyflow/react'
import type { FlowNode, FlowTrigger, NodeType } from './dsl'

/** Data stored in each React Flow node */
export type NodeData = Record<string, unknown> & (
  | (FlowTrigger & { nodeKind: 'trigger' })
  | (FlowNode & { nodeKind: 'process' })
)

/** React Flow node with typed data */
export type DesignerNode = Node<NodeData>

/** React Flow edge with transition metadata */
export type DesignerEdge = Edge<{ transitionType?: TransitionTypeEdge; condition?: string }>

/** Palette trigger keys (prefixed to avoid conflict with node type 'rabbitmq') */
export type PaletteTriggerKey =
  | 'trg_cron' | 'trg_rest' | 'trg_soap' | 'trg_rabbitmq' | 'trg_mcp' | 'trg_manual'

/** Node type keys used in the palette */
export type NodeTypeKey = PaletteTriggerKey | NodeType

/** Transition type used on edges */
export type TransitionTypeEdge = 'success' | 'error' | 'condition' | 'nocondition'

/** Palette item definition */
export interface PaletteItem {
  type: NodeTypeKey
  label: string
  description: string
  icon: string
  color: string
}

/** Palette category */
export interface PaletteCategory {
  label: string
  items: PaletteItem[]
}
