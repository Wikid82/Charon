import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor, act } from '@testing-library/react'
import React from 'react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import * as api from '../../api/orthrus'
import { useOrthrus, useAgentList, useProvisionAgent, AGENTS_QUERY_KEY } from '../useOrthrus'

vi.mock('../../api/orthrus', () => ({
  listAgents: vi.fn(),
  provisionAgent: vi.fn(),
  deleteAgent: vi.fn(),
  revokeAgent: vi.fn(),
  getInstallSnippets: vi.fn(),
}))

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

const mockAgent: api.OrthrusAgent = {
  uuid: 'agent-1',
  name: 'Test Agent',
  status: 'online',
  capabilities: '["proxy"]',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

describe('AGENTS_QUERY_KEY', () => {
  it('is stable', () => {
    expect(AGENTS_QUERY_KEY).toEqual(['orthrus', 'agents'])
  })
})

describe('useAgentList', () => {
  beforeEach(() => vi.clearAllMocks())

  it('fetches agents via listAgents', async () => {
    vi.mocked(api.listAgents).mockResolvedValue([mockAgent])

    const { result } = renderHook(() => useAgentList(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(result.current.data).toEqual([mockAgent])
    expect(api.listAgents).toHaveBeenCalledTimes(1)
  })
})

describe('useProvisionAgent', () => {
  beforeEach(() => vi.clearAllMocks())

  it('calls provisionAgent and invalidates agents query', async () => {
    const response: api.ProvisionAgentResponse = { agent: mockAgent, auth_key: 'plain-key' }
    vi.mocked(api.listAgents).mockResolvedValue([])
    vi.mocked(api.provisionAgent).mockImplementation(async () => {
      vi.mocked(api.listAgents).mockResolvedValue([mockAgent])
      return response
    })

    const { result } = renderHook(() => useProvisionAgent(), { wrapper: createWrapper() })

    let provisionResult: api.ProvisionAgentResponse | undefined
    await act(async () => {
      provisionResult = await result.current.mutateAsync({ name: 'Test Agent' })
    })

    expect(api.provisionAgent).toHaveBeenCalledWith({ name: 'Test Agent' })
    expect(provisionResult?.auth_key).toBe('plain-key')
  })
})

describe('useOrthrus', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.listAgents).mockResolvedValue([mockAgent])
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('loads agents on mount', async () => {
    const { result } = renderHook(() => useOrthrus(), { wrapper: createWrapper() })

    expect(result.current.loading).toBe(true)

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.agents).toEqual([mockAgent])
    expect(result.current.error).toBeNull()
  })

  it('handles loading error', async () => {
    vi.mocked(api.listAgents).mockRejectedValue(new Error('list failed'))

    const { result } = renderHook(() => useOrthrus(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.error).toBeTruthy()
    expect(result.current.agents).toEqual([])
  })

  it('provisionAgent calls API and invalidates agents query', async () => {
    const response: api.ProvisionAgentResponse = { agent: mockAgent, auth_key: 'key-123' }
    vi.mocked(api.provisionAgent).mockResolvedValue(response)

    const { result } = renderHook(() => useOrthrus(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.loading).toBe(false))

    let res: api.ProvisionAgentResponse | undefined
    await act(async () => {
      res = await result.current.provisionAgent({ name: 'New Agent' })
    })

    expect(api.provisionAgent).toHaveBeenCalledWith({ name: 'New Agent' })
    expect(res?.agent.uuid).toBe('agent-1')
    await waitFor(() => {
      expect(result.current.provisionResult).toEqual(response)
    })
  })

  it('deleteAgent calls API', async () => {
    vi.mocked(api.deleteAgent).mockImplementation(async () => {
      vi.mocked(api.listAgents).mockResolvedValue([])
    })

    const { result } = renderHook(() => useOrthrus(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.loading).toBe(false))

    act(() => {
      result.current.deleteAgent('agent-1')
    })

    await waitFor(() => expect(api.deleteAgent).toHaveBeenCalledWith('agent-1'))
  })

  it('revokeAgent calls API', async () => {
    vi.mocked(api.revokeAgent).mockResolvedValue({ message: 'revoked' })

    const { result } = renderHook(() => useOrthrus(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.loading).toBe(false))

    act(() => {
      result.current.revokeAgent('agent-1')
    })

    await waitFor(() => expect(api.revokeAgent).toHaveBeenCalledWith('agent-1'))
  })

  it('getInstallSnippets calls API', async () => {
    const snippets: api.InstallSnippets = {
      docker_compose: 'compose',
      systemd: 'systemd',
      tarball: 'tar',
      homebrew: 'brew',
      kubernetes_daemonset: 'k8s',
    }
    vi.mocked(api.getInstallSnippets).mockResolvedValue(snippets)

    const { result } = renderHook(() => useOrthrus(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.loading).toBe(false))

    let snippetResult: api.InstallSnippets | undefined
    await act(async () => {
      snippetResult = await result.current.getInstallSnippets('agent-1')
    })

    expect(api.getInstallSnippets).toHaveBeenCalledWith('agent-1')
    expect(snippetResult?.docker_compose).toBe('compose')
  })

  it('exposes pending state flags', async () => {
    const { result } = renderHook(() => useOrthrus(), { wrapper: createWrapper() })

    expect(result.current.isProvisioning).toBe(false)
    expect(result.current.isDeleting).toBe(false)
    expect(result.current.isRevoking).toBe(false)
    expect(result.current.isFetchingSnippets).toBe(false)
  })
})
