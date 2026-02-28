import { describe, it, expect } from 'vitest'
import { deserializeGraph } from './deserializer'
import type { FlowDSL } from '../types/dsl'

// Shared minimal DSL used across tests
const baseDSL: FlowDSL = {
  definition: {
    id: 'test-flow',
    version: '1.0.0',
    name: 'Test Flow',
    description: '',
    settings: { persistence: 'full', timeout: 30000, error_strategy: 'stop_and_rollback' },
  },
  trigger: { id: 'trg_01', type: 'rest', config: { path: '/v1/flow', method: 'POST' } },
  nodes: [],
  transitions: [],
}

describe('deserializeGraph', () => {
  it('creates a trigger node from dsl.trigger', () => {
    const { nodes } = deserializeGraph(baseDSL)
    expect(nodes).toHaveLength(1)
    const triggerNode = nodes[0]
    expect(triggerNode.id).toBe('trg_01')
    expect(triggerNode.type).toBe('triggerNode')
    expect((triggerNode.data as Record<string, unknown>).nodeKind).toBe('trigger')
    expect((triggerNode.data as Record<string, unknown>).type).toBe('rest')
  })

  it('creates activity nodes from dsl.nodes', () => {
    const dsl: FlowDSL = {
      ...baseDSL,
      nodes: [
        { id: 'n1', type: 'http', description: 'Call API', config: { url: 'https://example.com', method: 'GET' } },
        { id: 'n2', type: 'log', description: 'Log result', config: { level: 'INFO', message: 'done' } },
      ],
    }
    const { nodes } = deserializeGraph(dsl)
    expect(nodes).toHaveLength(3) // 1 trigger + 2 activity

    const n1 = nodes.find((n) => n.id === 'n1')
    const n2 = nodes.find((n) => n.id === 'n2')
    expect(n1?.type).toBe('activityNode')
    expect((n1?.data as Record<string, unknown>).nodeKind).toBe('process')
    expect((n1?.data as Record<string, unknown>).type).toBe('http')
    expect(n2?.type).toBe('activityNode')
  })

  it('positions trigger at x=80 and spaces activity nodes 250px apart', () => {
    const dsl: FlowDSL = {
      ...baseDSL,
      nodes: [
        { id: 'n1', type: 'log', config: { level: 'INFO', message: '' } },
        { id: 'n2', type: 'log', config: { level: 'INFO', message: '' } },
      ],
    }
    const { nodes } = deserializeGraph(dsl)
    const triggerNode = nodes.find((n) => n.id === 'trg_01')
    const n1 = nodes.find((n) => n.id === 'n1')
    const n2 = nodes.find((n) => n.id === 'n2')

    expect(triggerNode?.position.x).toBe(80)
    expect(n1?.position.x).toBe(80 + 250)
    expect(n2?.position.x).toBe(80 + 500)
    // all share the same y
    expect(triggerNode?.position.y).toBe(200)
    expect(n1?.position.y).toBe(200)
  })

  it('creates edges from dsl.transitions with type and condition', () => {
    const dsl: FlowDSL = {
      ...baseDSL,
      nodes: [
        { id: 'n1', type: 'http', config: {} as never },
        { id: 'n2', type: 'log', config: {} as never },
      ],
      transitions: [
        { from: 'n1', to: 'n2', type: 'success' },
        { from: 'n2', to: 'n1', type: 'condition', condition: "$.nodes.n2.status == 'error'" },
      ],
    }
    const { edges } = deserializeGraph(dsl)
    expect(edges).toHaveLength(2)

    const successEdge = edges.find((e) => e.source === 'n1')
    expect(successEdge?.target).toBe('n2')
    expect((successEdge?.data as Record<string, unknown>).transitionType).toBe('success')

    const condEdge = edges.find((e) => e.source === 'n2')
    expect(condEdge?.target).toBe('n1')
    expect((condEdge?.data as Record<string, unknown>).transitionType).toBe('condition')
    expect((condEdge?.data as Record<string, unknown>).condition).toBe("$.nodes.n2.status == 'error'")
  })

  it('creates success edges from node.next[] when no matching transition exists', () => {
    const dsl: FlowDSL = {
      ...baseDSL,
      nodes: [
        { id: 'n1', type: 'http', config: {} as never, next: ['n2'] },
        { id: 'n2', type: 'log', config: {} as never },
      ],
      transitions: [],
    }
    const { edges } = deserializeGraph(dsl)
    expect(edges).toHaveLength(1)
    expect(edges[0].source).toBe('n1')
    expect(edges[0].target).toBe('n2')
    expect((edges[0].data as Record<string, unknown>).transitionType).toBe('success')
  })

  it('does not duplicate edges when both transition and next[] cover the same pair', () => {
    const dsl: FlowDSL = {
      ...baseDSL,
      nodes: [
        { id: 'n1', type: 'http', config: {} as never, next: ['n2'] },
        { id: 'n2', type: 'log', config: {} as never },
      ],
      transitions: [{ from: 'n1', to: 'n2', type: 'success' }],
    }
    const { edges } = deserializeGraph(dsl)
    // Only 1 edge, not 2 (transition takes priority; next[] pair is skipped)
    expect(edges).toHaveLength(1)
  })

  it('returns only trigger node and no edges for an empty flow', () => {
    const { nodes, edges } = deserializeGraph(baseDSL)
    expect(nodes).toHaveLength(1)
    expect(edges).toHaveLength(0)
  })

  it('preserves all node data fields (config, input_mapping, script, etc.)', () => {
    const dsl: FlowDSL = {
      ...baseDSL,
      nodes: [
        {
          id: 'script_01',
          type: 'code',
          description: 'Transform payload',
          input_mapping: { raw: '$.trigger.body' },
          config: { script: 'export default (i) => i' },
          secret_ref: 'sec_api_key',
        },
      ],
    }
    const { nodes } = deserializeGraph(dsl)
    const scriptNode = nodes.find((n) => n.id === 'script_01')
    const data = scriptNode?.data as Record<string, unknown>
    expect(data.description).toBe('Transform payload')
    expect(data.input_mapping).toEqual({ raw: '$.trigger.body' })
    expect(data.secret_ref).toBe('sec_api_key')
  })
})
