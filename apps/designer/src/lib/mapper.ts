import type { InputMapping } from '../types/dsl'
import type { SchemaField, MappingConnection } from '../types/mapper'
import type { DesignerNode } from '../types/designer'

/**
 * Converts an array of visual mapping connections into the DSL `input_mapping` object.
 * Each connection maps a target key to the source JSONPath.
 * Note: `transformScript` on a connection is not persisted here — it is stored
 * separately in the node's `script` property via the MonacoModal editor.
 */
export function buildInputMapping(connections: MappingConnection[]): InputMapping {
  return connections.reduce<InputMapping>((acc, conn) => {
    acc[conn.targetKey] = conn.sourceField.path
    return acc
  }, {})
}

/**
 * Derives a flat list of source schema fields from all nodes that precede
 * `currentNodeId` in the flow. The fields are built from the known node ids
 * so the designer always has valid JSONPath suggestions.
 *
 * When actual output schemas become available (e.g. from execution history)
 * the `knownOutputs` map can seed richer field trees.
 */
export function buildSourceFields(
  nodes: DesignerNode[],
  currentNodeId: string,
  knownOutputs: Record<string, Record<string, unknown>> = {},
): SchemaField[] {
  const fields: SchemaField[] = []

  // Trigger output is always available
  fields.push({
    key: 'trigger',
    path: '$.trigger',
    type: 'object',
    children: [
      { key: 'body', path: '$.trigger.body', type: 'object' },
      { key: 'headers', path: '$.trigger.headers', type: 'object' },
    ],
  })

  // Add output fields for every process node except the current one
  for (const node of nodes) {
    if (node.id === currentNodeId) continue
    if (node.data.nodeKind !== 'process') continue

    const basePath = `$.nodes.${node.data.id}.output`
    const knownOutput = knownOutputs[node.data.id]

    const children = knownOutput
      ? objectToSchemaFields(knownOutput, basePath)
      : []

    fields.push({
      key: node.data.id,
      path: basePath,
      type: 'object',
      children,
    })
  }

  return fields
}

/**
 * Recursively converts a plain JS object into a SchemaField tree.
 * Used to build source trees from known execution outputs.
 */
export function objectToSchemaFields(
  obj: Record<string, unknown>,
  basePath: string,
): SchemaField[] {
  return Object.entries(obj).map(([key, value]) => {
    const path = `${basePath}.${key}`
    const type = inferType(value)
    const field: SchemaField = { key, path, type }
    if (type === 'object' && value !== null && typeof value === 'object' && !Array.isArray(value)) {
      field.children = objectToSchemaFields(value as Record<string, unknown>, path)
    }
    return field
  })
}

function inferType(value: unknown): SchemaField['type'] {
  if (value === null || value === undefined) return 'unknown'
  if (Array.isArray(value)) return 'array'
  switch (typeof value) {
    case 'string': return 'string'
    case 'number': return 'number'
    case 'boolean': return 'boolean'
    case 'object': return 'object'
    default: return 'unknown'
  }
}
