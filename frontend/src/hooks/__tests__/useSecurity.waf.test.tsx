import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { AxiosError, type AxiosResponse } from 'axios'
import toast from 'react-hot-toast'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as securityApi from '../../api/security'
import { SECURITY_STATUS_QUERY_KEY, useDeleteRuleSet, useUpdateWafRulesSource } from '../useSecurity'

vi.mock('../../api/security')
vi.mock('react-hot-toast')

const codedError = (code: string): AxiosError => {
  const response = { status: 409, statusText: 'Conflict', headers: {}, config: {}, data: { error_code: code } } as AxiosResponse
  return new AxiosError('Request failed', 'ERR_BAD_REQUEST', undefined, undefined, response)
}

describe('useUpdateWafRulesSource', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    vi.clearAllMocks()
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )

  it('saves only the rules source on the default row', async () => {
    vi.mocked(securityApi.updateSecurityConfig).mockResolvedValue({})
    const { result } = renderHook(() => useUpdateWafRulesSource(), { wrapper })

    result.current.mutate('OWASP CRS')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(securityApi.updateSecurityConfig).toHaveBeenCalledWith({ name: 'default', waf_rules_source: 'OWASP CRS' })
    expect(toast.success).toHaveBeenCalledWith('WAF rule set source saved')
  })

  it('saves the empty string for "Choose automatically"', async () => {
    vi.mocked(securityApi.updateSecurityConfig).mockResolvedValue({})
    const { result } = renderHook(() => useUpdateWafRulesSource(), { wrapper })

    result.current.mutate('')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(securityApi.updateSecurityConfig).toHaveBeenCalledWith({ name: 'default', waf_rules_source: '' })
  })

  it('refreshes the stored config and the shared security status after a save', async () => {
    vi.mocked(securityApi.updateSecurityConfig).mockResolvedValue({})
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useUpdateWafRulesSource(), { wrapper })

    result.current.mutate('OWASP CRS')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['securityConfig'] })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: SECURITY_STATUS_QUERY_KEY })
  })

  it('shows the translated text for a coded refusal', async () => {
    vi.mocked(securityApi.updateSecurityConfig).mockRejectedValue(codedError('waf_rules_source_unknown'))
    const { result } = renderHook(() => useUpdateWafRulesSource(), { wrapper })

    result.current.mutate('missing')

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(toast.error).toHaveBeenCalledWith('That rule set does not exist. Choose one from the list.')
  })

  it('shows the generic save failure for an uncoded error', async () => {
    vi.mocked(securityApi.updateSecurityConfig).mockRejectedValue(new Error('network down'))
    const { result } = renderHook(() => useUpdateWafRulesSource(), { wrapper })

    result.current.mutate('OWASP CRS')

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(toast.error).toHaveBeenCalledWith('Could not save the WAF rule set source')
  })
})

describe('useDeleteRuleSet in-use refusal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('deletes by uuid and shows the in-use text for a coded refusal', async () => {
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    vi.mocked(securityApi.deleteRuleSet).mockRejectedValue(codedError('waf_ruleset_in_use'))
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )
    const { result } = renderHook(() => useDeleteRuleSet(), { wrapper })

    result.current.mutate('uuid-1')

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(securityApi.deleteRuleSet).toHaveBeenCalledWith('uuid-1')
    expect(toast.error).toHaveBeenCalledWith(
      'This rule set is the one the WAF uses. Choose another rule set first.'
    )
  })
})
