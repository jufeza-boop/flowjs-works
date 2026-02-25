import { describe, it, expect } from 'vitest'
import type { Node, Edge } from '@xyflow/react'
import { serializeGraph } from './serializer'
import type { NodeData } from '../types/designer'
import type { FlowDefinition } from '../types/dsl'

const testDefinition: FlowDefinition = {
  id: 'test-flow',
  version: '1.0.0',
  name: 'Test Flow',
  description: 'A test flow',
  settings: { persistence: 'full', timeout: 30000, error_strategy: 'stop_and_rollback' },
}

describe('serializeGraph', () => {
  it('produces valid DSL with trigger, nodes and transitions', () => {
    const nodes: Node<NodeData>[] = [
      {
        id: 'trg_01',
        type: 'triggerNode',
        position: { x: 0, y: 0 },
        data: {
          nodeKind: 'trigger',
          id: 'trg_01',
          type: 'http_webhook',
          config: { path: '/v1/register', method: 'POST' },
        },
      },
      {
        id: 'script_01',
        type: 'scriptNode',
        position: { x: 200, y: 0 },
        data: {
          nodeKind: 'process',
          id: 'script_01',
          type: 'script_ts',
          description: 'Normalize data',
          input_mapping: { raw_data: '$.trigger.body' },
          script: 'export default (input) => input',
        },
      },
      {
        id: 'db_01',
        type: 'sqlNode',
        position: { x: 400, y: 0 },
        data: {
          nodeKind: 'process',
          id: 'db_01',
          type: 'sql_insert',
          description: 'Insert user',
          config: { datasource: 'postgres_main', table: 'users' },
          input_mapping: { fields: '$.nodes.script_01.output' },
        },
      },
    ]

    const edges: Edge[] = [
      { id: 'e1', source: 'script_01', target: 'db_01' },
      {
        id: 'e2',
        source: 'db_01',
        target: 'script_01',
        data: { condition: "$.nodes.db_01.status == 'success'" },
      },
    ]

    const dsl = serializeGraph(nodes, edges, testDefinition)

    // definition is preserved
    expect(dsl.definition).toEqual(testDefinition)

    // trigger is correctly extracted
    expect(dsl.trigger.id).toBe('trg_01')
    expect(dsl.trigger.type).toBe('http_webhook')
    expect(dsl.trigger.config.path).toBe('/v1/register')

    // process nodes are serialized
    expect(dsl.nodes).toHaveLength(2)
    const scriptNode = dsl.nodes.find((n) => n.id === 'script_01')
    expect(scriptNode).toBeDefined()
    expect(scriptNode?.type).toBe('script_ts')
    expect(scriptNode?.next).toEqual(['db_01'])

    const dbNode = dsl.nodes.find((n) => n.id === 'db_01')
    expect(dbNode).toBeDefined()
    expect(dbNode?.type).toBe('sql_insert')

    // transitions with conditions
    expect(dsl.transitions).toHaveLength(1)
    expect(dsl.transitions[0].from).toBe('db_01')
    expect(dsl.transitions[0].condition).toBe("$.nodes.db_01.status == 'success'")
  })

  it('generates default trigger when no trigger node exists', () => {
    const nodes: Node<NodeData>[] = [
      {
        id: 'n1',
        type: 'scriptNode',
        position: { x: 0, y: 0 },
        data: {
          nodeKind: 'process',
          id: 'n1',
          type: 'script_ts',
          description: 'Script',
        },
      },
    ]
    const dsl = serializeGraph(nodes, [], testDefinition)
    expect(dsl.trigger.id).toBe('trg_01')
    expect(dsl.nodes).toHaveLength(1)
    expect(dsl.transitions).toHaveLength(0)
  })

  it('handles empty graph', () => {
    const dsl = serializeGraph([], [], testDefinition)
    expect(dsl.definition).toEqual(testDefinition)
    expect(dsl.trigger).toBeDefined()
    expect(dsl.nodes).toHaveLength(0)
    expect(dsl.transitions).toHaveLength(0)
  })

  it('serializes input_mapping with JSONPath references', () => {
    const nodes: Node<NodeData>[] = [
      {
        id: 'n1',
        type: 'scriptNode',
        position: { x: 0, y: 0 },
        data: {
          nodeKind: 'process',
          id: 'n1',
          type: 'script_ts',
          input_mapping: {
            email: '$.trigger.body.email',
            ts: '$.trigger.headers.date',
          },
        },
      },
    ]
    const dsl = serializeGraph(nodes, [], testDefinition)
    expect(dsl.nodes[0].input_mapping).toEqual({
      email: '$.trigger.body.email',
      ts: '$.trigger.headers.date',
    })
  })

  it('serializes http_request node config with url, method, timeout and headers', () => {
    const nodes: Node<NodeData>[] = [
      {
        id: 'http_01',
        type: 'httpRequestNode',
        position: { x: 0, y: 0 },
        data: {
          nodeKind: 'process',
          id: 'http_01',
          type: 'http_request',
          description: 'Fetch cat fact',
          config: {
            url: 'https://catfact.ninja/fact',
            method: 'GET',
            timeout: 30,
            headers: { Accept: 'application/json' },
          },
        },
      },
    ]
    const dsl = serializeGraph(nodes, [], testDefinition)
    expect(dsl.nodes).toHaveLength(1)
    const httpNode = dsl.nodes[0]
    expect(httpNode.type).toBe('http_request')
    expect(httpNode.config).toEqual({
      url: 'https://catfact.ninja/fact',
      method: 'GET',
      timeout: 30,
      headers: { Accept: 'application/json' },
    })
  })

  it('includes type field in all transitions', () => {
    const nodes: Node<NodeData>[] = [
      {
        id: 'trg_01',
        type: 'triggerNode',
        position: { x: 0, y: 0 },
        data: {
          nodeKind: 'trigger',
          id: 'trg_01',
          type: 'rest',
          config: { path: '/v1/flow', method: 'POST' },
        },
      },
      {
        id: 'n1',
        type: 'activityNode',
        position: { x: 200, y: 0 },
        data: { nodeKind: 'process', id: 'n1', type: 'http', description: 'HTTP call' },
      },
      {
        id: 'n2',
        type: 'activityNode',
        position: { x: 400, y: 0 },
        data: { nodeKind: 'process', id: 'n2', type: 'log', description: 'Log' },
      },
    ]

    const edges: Edge[] = [
      { id: 'e1', source: 'n1', target: 'n2', data: { transitionType: 'success' } },
      { id: 'e2', source: 'n2', target: 'n1', data: { transitionType: 'error' } },
    ]

    const dsl = serializeGraph(nodes, edges, testDefinition)
    expect(dsl.transitions).toHaveLength(2)
    expect(dsl.transitions[0]).toMatchObject({ from: 'n1', to: 'n2', type: 'success' })
    expect(dsl.transitions[1]).toMatchObject({ from: 'n2', to: 'n1', type: 'error' })
  })

  it('uses rest as default trigger type', () => {
    const dsl = serializeGraph([], [], testDefinition)
    expect(dsl.trigger.type).toBe('rest')
  })
})
