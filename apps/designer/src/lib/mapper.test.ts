import { describe, it, expect } from 'vitest'
import { buildInputMapping, buildSourceFields, objectToSchemaFields } from './mapper'
import type { MappingConnection, SchemaField } from '../types/mapper'
import type { DesignerNode } from '../types/designer'

// ---------------------------------------------------------------------------
// buildInputMapping
// ---------------------------------------------------------------------------
describe('buildInputMapping', () => {
  it('returns an empty object when there are no connections', () => {
    expect(buildInputMapping([])).toEqual({})
  })

  it('maps target key to source JSONPath', () => {
    const connections: MappingConnection[] = [
      {
        sourceField: { key: 'email', path: '$.trigger.body.email', type: 'string' },
        targetKey: 'email',
      },
    ]
    expect(buildInputMapping(connections)).toEqual({ email: '$.trigger.body.email' })
  })

  it('handles multiple connections', () => {
    const connections: MappingConnection[] = [
      {
        sourceField: { key: 'email', path: '$.trigger.body.email', type: 'string' },
        targetKey: 'email',
      },
      {
        sourceField: { key: 'output', path: '$.nodes.node_01.output', type: 'object' },
        targetKey: 'fields',
      },
    ]
    const result = buildInputMapping(connections)
    expect(result).toEqual({
      email: '$.trigger.body.email',
      fields: '$.nodes.node_01.output',
    })
  })

  it('last connection wins when multiple connections share the same targetKey', () => {
    const connections: MappingConnection[] = [
      {
        sourceField: { key: 'a', path: '$.trigger.body.a', type: 'string' },
        targetKey: 'data',
      },
      {
        sourceField: { key: 'b', path: '$.trigger.body.b', type: 'string' },
        targetKey: 'data',
      },
    ]
    const result = buildInputMapping(connections)
    expect(result['data']).toBe('$.trigger.body.b')
  })
})

// ---------------------------------------------------------------------------
// buildSourceFields
// ---------------------------------------------------------------------------
describe('buildSourceFields', () => {
  const makeNode = (id: string): DesignerNode => ({
    id,
    type: 'scriptNode',
    position: { x: 0, y: 0 },
    data: { nodeKind: 'process', id, type: 'script_ts' },
  })

  it('always includes trigger fields', () => {
    const fields = buildSourceFields([], 'current')
    const trigger = fields.find((f) => f.key === 'trigger')
    expect(trigger).toBeDefined()
    expect(trigger?.path).toBe('$.trigger')
    expect(trigger?.children).toHaveLength(2)
  })

  it('excludes the current node from source fields', () => {
    const nodes: DesignerNode[] = [makeNode('current'), makeNode('other')]
    const fields = buildSourceFields(nodes, 'current')
    const ids = fields.map((f) => f.key)
    expect(ids).not.toContain('current')
    expect(ids).toContain('other')
  })

  it('creates output path using $.nodes.<id>.output', () => {
    const nodes: DesignerNode[] = [makeNode('node_01')]
    const fields = buildSourceFields(nodes, 'current')
    const nodeField = fields.find((f) => f.key === 'node_01')
    expect(nodeField?.path).toBe('$.nodes.node_01.output')
  })

  it('expands children from knownOutputs', () => {
    const nodes: DesignerNode[] = [makeNode('node_01')]
    const fields = buildSourceFields(nodes, 'current', {
      node_01: { email: 'a@b.com', active: true },
    })
    const nodeField = fields.find((f) => f.key === 'node_01')
    expect(nodeField?.children).toHaveLength(2)
    expect(nodeField?.children?.[0].key).toBe('email')
    expect(nodeField?.children?.[0].path).toBe('$.nodes.node_01.output.email')
  })

  it('ignores trigger nodes from the nodes list', () => {
    const triggerNode: DesignerNode = {
      id: 'trg_01',
      type: 'triggerNode',
      position: { x: 0, y: 0 },
      data: { nodeKind: 'trigger', id: 'trg_01', type: 'http_webhook', config: {} },
    }
    const fields = buildSourceFields([triggerNode], 'current')
    // Only the hardcoded trigger field — the trigger DesignerNode should not add a duplicate
    const triggerFields = fields.filter((f) => f.key === 'trg_01')
    expect(triggerFields).toHaveLength(0)
  })
})

// ---------------------------------------------------------------------------
// objectToSchemaFields
// ---------------------------------------------------------------------------
describe('objectToSchemaFields', () => {
  it('handles primitive values', () => {
    const fields = objectToSchemaFields({ name: 'Alice', age: 30, active: true }, '$.root')
    expect(fields).toHaveLength(3)
    const nameField = fields.find((f) => f.key === 'name')
    expect(nameField?.type).toBe('string')
    expect(nameField?.path).toBe('$.root.name')

    const ageField = fields.find((f) => f.key === 'age')
    expect(ageField?.type).toBe('number')

    const activeField = fields.find((f) => f.key === 'active')
    expect(activeField?.type).toBe('boolean')
  })

  it('recursively expands nested objects', () => {
    const fields = objectToSchemaFields({ address: { city: 'Madrid', zip: '28001' } }, '$.root')
    const addressField = fields.find((f) => f.key === 'address')
    expect(addressField?.type).toBe('object')
    expect(addressField?.children).toHaveLength(2)
    expect(addressField?.children?.[0].path).toBe('$.root.address.city')
  })

  it('marks arrays with type array', () => {
    const fields = objectToSchemaFields({ tags: ['a', 'b'] }, '$.root')
    const tagsField = fields.find((f) => f.key === 'tags')
    expect(tagsField?.type).toBe('array')
    expect(tagsField?.children).toBeUndefined()
  })

  it('marks null values as unknown', () => {
    const fields = objectToSchemaFields({ data: null }, '$.root')
    expect(fields[0].type).toBe('unknown')
  })

  it('returns empty array for empty object', () => {
    expect(objectToSchemaFields({}, '$.root')).toEqual([])
  })
})

// ---------------------------------------------------------------------------
// Round-trip: build connections from fields then call buildInputMapping
// ---------------------------------------------------------------------------
describe('round-trip: buildSourceFields -> buildInputMapping', () => {
  it('produces valid JSONPath mapping from visual connections', () => {
    const nodes: DesignerNode[] = [
      {
        id: 'map_01',
        type: 'scriptNode',
        position: { x: 0, y: 0 },
        data: { nodeKind: 'process', id: 'map_01', type: 'script_ts' },
      },
    ]
    const sourceFields = buildSourceFields(nodes, 'current_node')
    // Simulate user connecting trigger.body -> raw_data and map_01.output -> fields
    const triggerField = sourceFields.find((f) => f.key === 'trigger')
    const emailField: SchemaField = triggerField!.children!.find((f) => f.key === 'body')!
    const outputField: SchemaField = sourceFields.find((f) => f.key === 'map_01')!
    expect(emailField).toBeDefined()
    expect(outputField).toBeDefined()

    const connections: MappingConnection[] = [
      { sourceField: emailField, targetKey: 'raw_data' },
      { sourceField: outputField, targetKey: 'fields' },
    ]
    const mapping = buildInputMapping(connections)
    expect(mapping).toEqual({
      raw_data: '$.trigger.body',
      fields: '$.nodes.map_01.output',
    })
  })
})
