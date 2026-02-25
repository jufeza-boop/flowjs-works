import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fetchExecutions, fetchActivityLogs, runFlow, listSecrets, createSecret, deleteSecret } from './api'
import type { Execution, ActivityLog } from '../types/audit'
import type { SecretMeta } from '../types/secrets'

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
