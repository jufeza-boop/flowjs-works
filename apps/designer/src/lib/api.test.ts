import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fetchExecutions, fetchActivityLogs, runFlow } from './api'
import type { Execution, ActivityLog } from '../types/audit'

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
