import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fetchExecutions, fetchActivityLogs, runFlow, listSecrets, createSecret, deleteSecret, listProcesses, saveProcess, deployProcess, stopProcess, deleteProcess, getProcess, fetchTriggerData, replayExecution, replayFromNode } from './api'
import type { Execution, ActivityLog } from '../types/audit'
import type { SecretMeta } from '../types/secrets'
import type { ProcessSummary } from '../types/deployment'

describe('fetchExecutions', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('returns an array of executions on success', async () => {
    const mockExecutions: Execution[] = [
      {
        execution_id: 'abc-123',
        flow_id: 'my-flow',
        version: '1.0.0',
        status: 'COMPLETED',
        correlation_id: '',
        start_time: '2024-01-01T00:00:00Z',
        trigger_type: 'http_webhook',
        main_error_message: '',
      },
    ]

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockExecutions),
      }),
    )

    const result = await fetchExecutions()
    expect(result).toEqual(mockExecutions)
  })

  it('throws an error when the response is not ok', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        text: () => Promise.resolve('Internal Server Error'),
      }),
    )

    await expect(fetchExecutions()).rejects.toThrow('Failed to fetch executions (500)')
  })
})

describe('fetchActivityLogs', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('returns activity logs for a given execution id', async () => {
    const mockLogs: ActivityLog[] = [
      {
        log_id: 1,
        node_id: 'node_1',
        node_type: 'logger',
        status: 'SUCCESS',
        input_data: { message: 'hello' },
        output_data: { logged: true },
        error_details: null,
        duration_ms: 5,
        created_at: '2024-01-01T00:00:00Z',
      },
    ]

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockLogs),
      }),
    )

    const result = await fetchActivityLogs('abc-123')
    expect(result).toEqual(mockLogs)
  })

  it('throws an error when the response is not ok', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
        text: () => Promise.resolve('Not Found'),
      }),
    )

    await expect(fetchActivityLogs('missing-id')).rejects.toThrow(
      'Failed to fetch activity logs (404)',
    )
  })

  it('URL-encodes the execution id', async () => {
    let capturedUrl = ''
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((url: string) => {
        capturedUrl = url
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve([]),
        })
      }),
    )

    await fetchActivityLogs('exec id with spaces')
    expect(capturedUrl).toContain('exec%20id%20with%20spaces')
  })
})

describe('runFlow', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('returns execution result on success', async () => {
    const mockResponse = {
      execution_id: 'exec-abc-123',
      nodes: {
        log_1: { status: 'success', output: { logged: true } },
      },
    }

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      }),
    )

    const result = await runFlow({
      dsl: {
        definition: { id: 'test', version: '1.0.0', name: 'Test', description: '', settings: { persistence: 'full', timeout: 30000, error_strategy: 'stop_and_rollback' } },
        trigger: { id: 'trg_01', type: 'http_webhook' as 'rest', config: {} as never },
        nodes: [],
        transitions: [],
      },
    })

    expect(result.execution_id).toBe('exec-abc-123')
    expect(result.nodes['log_1'].status).toBe('success')
  })

  it('throws an error when the response is not ok', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 422,
        json: () => Promise.resolve({ error: 'node failed', execution_id: '', nodes: {} }),
      }),
    )

    await expect(
      runFlow({
        dsl: {
          definition: { id: 'test', version: '1.0.0', name: 'Test', description: '', settings: { persistence: 'full', timeout: 30000, error_strategy: 'stop_and_rollback' } },
          trigger: { id: 'trg_01', type: 'http_webhook' as 'rest', config: {} as never },
          nodes: [],
          transitions: [],
        },
      }),
    ).rejects.toThrow('Run flow failed (422)')
  })
})

// ── Secrets API ──────────────────────────────────────────────────────────────

describe('listSecrets', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('returns an array of secret metadata on success', async () => {
    const mockSecrets: SecretMeta[] = [
      { id: 'sec_pg', name: 'Postgres', type: 'connection_string', created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
    ]
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(mockSecrets) }))

    const result = await listSecrets()
    expect(result).toEqual(mockSecrets)
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 503, text: () => Promise.resolve('unavailable') }))
    await expect(listSecrets()).rejects.toThrow('Failed to list secrets (503)')
  })
})

describe('createSecret', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('resolves without error on 201', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ id: 'sec_1', status: 'saved' }) }))
    await expect(createSecret({ id: 'sec_1', name: 'Test', type: 'token', value: { token: 'abc' } })).resolves.toBeUndefined()
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 400, text: () => Promise.resolve('id is required') }))
    await expect(createSecret({ id: '', name: '', type: 'token', value: {} })).rejects.toThrow('Failed to save secret (400)')
  })

  it('sends POST with correct body', async () => {
    let capturedBody = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedBody = opts.body as string
      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) })
    }))
    await createSecret({ id: 'sec_1', name: 'My Token', type: 'token', value: { token: 'xyz' } })
    const parsed = JSON.parse(capturedBody) as { id: string; name: string }
    expect(parsed.id).toBe('sec_1')
    expect(parsed.name).toBe('My Token')
  })
})

describe('deleteSecret', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('resolves without error on 204', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, text: () => Promise.resolve('') }))
    await expect(deleteSecret('sec_pg')).resolves.toBeUndefined()
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 503, text: () => Promise.resolve('store not configured') }))
    await expect(deleteSecret('sec_pg')).rejects.toThrow('Failed to delete secret (503)')
  })

  it('URL-encodes the secret id', async () => {
    let capturedUrl = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve({ ok: true, text: () => Promise.resolve('') })
    }))
    await deleteSecret('sec id with spaces')
    expect(capturedUrl).toContain('sec%20id%20with%20spaces')
  })
})

// ── Process & Deployment API ─────────────────────────────────────────────────

describe('listProcesses', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('returns an array of process summaries on success', async () => {
    const mockProcesses: ProcessSummary[] = [
      { id: 'my-flow', version: '1.0.0', name: 'My Flow', status: 'draft', updated_at: '2025-01-01T00:00:00Z' },
    ]
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(mockProcesses) }))

    const result = await listProcesses()
    expect(result).toEqual(mockProcesses)
  })

  it('includes status query param when provided', async () => {
    let capturedUrl = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve({ ok: true, json: () => Promise.resolve([]) })
    }))
    await listProcesses('deployed')
    expect(capturedUrl).toContain('status=deployed')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 503, text: () => Promise.resolve('unavailable') }))
    await expect(listProcesses()).rejects.toThrow('Failed to list processes (503)')
  })
})

describe('saveProcess', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  const sampleDSL = {
    definition: { id: 'p1', version: '1.0.0', name: 'P1', description: '', settings: { persistence: 'full' as const, timeout: 30000, error_strategy: 'stop_and_rollback' as const } },
    trigger: { id: 'trg_01', type: 'manual' as const, config: {} as never },
    nodes: [],
    transitions: [],
  }

  it('returns process summary on success', async () => {
    const summary: ProcessSummary = { id: 'p1', version: '1.0.0', name: 'P1', status: 'draft', updated_at: '2025-01-01T00:00:00Z' }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(summary) }))

    const result = await saveProcess(sampleDSL)
    expect(result.id).toBe('p1')
    expect(result.status).toBe('draft')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 400, json: () => Promise.resolve({ error: 'bad request' }) }))
    await expect(saveProcess(sampleDSL)).rejects.toThrow('Failed to save process (400)')
  })
})

describe('deployProcess', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('returns deployment status on success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ process_id: 'p1', status: 'deployed', message: 'cron trigger started' }) }))
    const result = await deployProcess('p1')
    expect(result.status).toBe('deployed')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 400, json: () => Promise.resolve({ error: 'bad trigger' }) }))
    await expect(deployProcess('p1')).rejects.toThrow('Failed to deploy process (400)')
  })
})

describe('stopProcess', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('returns stopped status on success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ process_id: 'p1', status: 'stopped' }) }))
    const result = await stopProcess('p1')
    expect(result.status).toBe('stopped')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 400, json: () => Promise.resolve({ error: 'not deployed' }) }))
    await expect(stopProcess('p1')).rejects.toThrow('Failed to stop process (400)')
  })
})

describe('deleteProcess', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('resolves without error on 204', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, text: () => Promise.resolve('') }))
    await expect(deleteProcess('p1')).resolves.toBeUndefined()
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, text: () => Promise.resolve('error') }))
    await expect(deleteProcess('p1')).rejects.toThrow('Failed to delete process (500)')
  })
})

describe('getProcess', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('returns a process record with dsl on success', async () => {
    const mockRecord = {
      id: 'my-flow',
      version: '1.0.0',
      name: 'My Flow',
      description: '',
      dsl: {
        definition: { id: 'my-flow', version: '1.0.0', name: 'My Flow', description: '', settings: { persistence: 'full', timeout: 30000, error_strategy: 'stop_and_rollback' } },
        trigger: { id: 'trg_01', type: 'rest', config: { path: '/v1/flow', method: 'POST' } },
        nodes: [],
        transitions: [],
      },
      status: 'draft',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(mockRecord) }))

    const result = await getProcess('my-flow')
    expect(result.id).toBe('my-flow')
    expect(result.dsl.definition.id).toBe('my-flow')
    expect(result.status).toBe('draft')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 404, text: () => Promise.resolve('not found') }))
    await expect(getProcess('missing')).rejects.toThrow('Failed to get process (404)')
  })

  it('URL-encodes the process id', async () => {
    let capturedUrl = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) })
    }))
    await getProcess('flow id with spaces').catch(() => undefined)
    expect(capturedUrl).toContain('flow%20id%20with%20spaces')
  })
})

// ── Observability & Replay API ────────────────────────────────────────────────

describe('fetchExecutions with options', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('appends status param when provided', async () => {
    let capturedUrl = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve({ ok: true, json: () => Promise.resolve([]) })
    }))
    await fetchExecutions({ status: 'FAILED' })
    expect(capturedUrl).toContain('status=FAILED')
  })

  it('appends search param when provided', async () => {
    let capturedUrl = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve({ ok: true, json: () => Promise.resolve([]) })
    }))
    await fetchExecutions({ search: 'hello world' })
    expect(capturedUrl).toContain('search=hello+world')
  })

  it('appends limit and offset params when provided', async () => {
    let capturedUrl = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve({ ok: true, json: () => Promise.resolve([]) })
    }))
    await fetchExecutions({ limit: 20, offset: 40 })
    expect(capturedUrl).toContain('limit=20')
    expect(capturedUrl).toContain('offset=40')
  })

  it('does not append params when options are empty', async () => {
    let capturedUrl = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve({ ok: true, json: () => Promise.resolve([]) })
    }))
    await fetchExecutions({})
    expect(capturedUrl).not.toContain('?')
  })
})

describe('fetchTriggerData', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('returns trigger data on success', async () => {
    const mockData = { event: 'order_placed', amount: 100 }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(mockData) }))
    const result = await fetchTriggerData('exec-1')
    expect(result).toEqual(mockData)
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 404, text: () => Promise.resolve('not found') }))
    await expect(fetchTriggerData('exec-missing')).rejects.toThrow('Failed to fetch trigger data (404)')
  })

  it('URL-encodes the execution id', async () => {
    let capturedUrl = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) })
    }))
    await fetchTriggerData('exec id with spaces')
    expect(capturedUrl).toContain('exec%20id%20with%20spaces')
  })
})

describe('replayExecution', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('returns RunFlowResponse on success', async () => {
    const mockResponse = { execution_id: 'replay-1', nodes: {} }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(mockResponse) }))
    const result = await replayExecution('my-flow', { event: 'test' })
    expect(result.execution_id).toBe('replay-1')
  })

  it('sends trigger_data in request body', async () => {
    let capturedBody = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((_url: string, opts: RequestInit) => {
      capturedBody = opts.body as string
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ execution_id: 'r1', nodes: {} }) })
    }))
    await replayExecution('my-flow', { key: 'value' })
    const parsed = JSON.parse(capturedBody) as { trigger_data: Record<string, unknown> }
    expect(parsed.trigger_data).toEqual({ key: 'value' })
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, json: () => Promise.resolve({ error: 'engine error', execution_id: '', nodes: {} }) }))
    await expect(replayExecution('my-flow', {})).rejects.toThrow('Replay failed (500)')
  })
})

describe('replayFromNode', () => {
  beforeEach(() => { vi.restoreAllMocks() })

  it('returns RunFlowResponse on success', async () => {
    const mockResponse = { execution_id: 'partial-1', nodes: {} }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(mockResponse) }))
    const result = await replayFromNode('my-flow', 'node_1', { x: 1 })
    expect(result.execution_id).toBe('partial-1')
  })

  it('sends node_input in request body and uses correct URL', async () => {
    let capturedUrl = ''
    let capturedBody = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string, opts: RequestInit) => {
      capturedUrl = url
      capturedBody = opts.body as string
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ execution_id: 'r2', nodes: {} }) })
    }))
    await replayFromNode('my-flow', 'node_1', { foo: 'bar' })
    expect(capturedUrl).toContain('/replay-from/node_1')
    const parsed = JSON.parse(capturedBody) as { node_input: Record<string, unknown> }
    expect(parsed.node_input).toEqual({ foo: 'bar' })
  })

  it('URL-encodes the node id', async () => {
    let capturedUrl = ''
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      capturedUrl = url
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ execution_id: 'r3', nodes: {} }) })
    }))
    await replayFromNode('my-flow', 'node id with spaces', {})
    expect(capturedUrl).toContain('node%20id%20with%20spaces')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 422, json: () => Promise.resolve({ error: 'node not found', execution_id: '', nodes: {} }) }))
    await expect(replayFromNode('my-flow', 'node_1', {})).rejects.toThrow('Replay from node failed (422)')
  })
})
