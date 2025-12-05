import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  useSecurityStatus,
  useSecurityConfig,
  useUpdateSecurityConfig,
  useGenerateBreakGlassToken,
  useDecisions,
  useCreateDecision,
  useRuleSets,
  useUpsertRuleSet,
  useDeleteRuleSet,
  useEnableCerberus,
  useDisableCerberus,
} from '../useSecurity'
import * as securityApi from '../../api/security'
import toast from 'react-hot-toast'

vi.mock('../../api/security')
vi.mock('react-hot-toast')

describe('useSecurity hooks', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    vi.clearAllMocks()
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )

  describe('useSecurityStatus', () => {
    it('should fetch security status', async () => {
      const mockStatus = {
        cerberus: { enabled: true },
        crowdsec: { mode: 'local' as const, api_url: 'http://localhost', enabled: true },
        waf: { mode: 'enabled' as const, enabled: true },
        rate_limit: { enabled: true },
        acl: { enabled: true }
      }
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatus)

      const { result } = renderHook(() => useSecurityStatus(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).toEqual(mockStatus)
    })
  })

  describe('useSecurityConfig', () => {
    it('should fetch security config', async () => {
      const mockConfig = { config: { admin_whitelist: '10.0.0.0/8' } }
      vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockConfig)

      const { result } = renderHook(() => useSecurityConfig(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).toEqual(mockConfig)
    })
  })

  describe('useUpdateSecurityConfig', () => {
    it('should update security config and invalidate queries on success', async () => {
      const payload = { admin_whitelist: '192.168.0.0/16' }
      vi.mocked(securityApi.updateSecurityConfig).mockResolvedValue({ success: true })

      const { result } = renderHook(() => useUpdateSecurityConfig(), { wrapper })

      result.current.mutate(payload)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(securityApi.updateSecurityConfig).toHaveBeenCalledWith(payload)
      expect(toast.success).toHaveBeenCalledWith('Security configuration updated')
    })

    it('should show error toast on failure', async () => {
      const error = new Error('Update failed')
      vi.mocked(securityApi.updateSecurityConfig).mockRejectedValue(error)

      const { result } = renderHook(() => useUpdateSecurityConfig(), { wrapper })

      result.current.mutate({ admin_whitelist: 'invalid' })

      await waitFor(() => expect(result.current.isError).toBe(true))
      expect(toast.error).toHaveBeenCalledWith('Failed to update security settings: Update failed')
    })
  })

  describe('useGenerateBreakGlassToken', () => {
    it('should generate break glass token', async () => {
      const mockToken = { token: 'abc123' }
      vi.mocked(securityApi.generateBreakGlassToken).mockResolvedValue(mockToken)

      const { result } = renderHook(() => useGenerateBreakGlassToken(), { wrapper })

      result.current.mutate(undefined)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).toEqual(mockToken)
    })
  })

  describe('useDecisions', () => {
    it('should fetch decisions with default limit', async () => {
      const mockDecisions = { decisions: [{ ip: '1.2.3.4', type: 'ban' }] }
      vi.mocked(securityApi.getDecisions).mockResolvedValue(mockDecisions)

      const { result } = renderHook(() => useDecisions(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(securityApi.getDecisions).toHaveBeenCalledWith(50)
      expect(result.current.data).toEqual(mockDecisions)
    })

    it('should fetch decisions with custom limit', async () => {
      const mockDecisions = { decisions: [] }
      vi.mocked(securityApi.getDecisions).mockResolvedValue(mockDecisions)

      const { result } = renderHook(() => useDecisions(100), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(securityApi.getDecisions).toHaveBeenCalledWith(100)
    })
  })

  describe('useCreateDecision', () => {
    it('should create decision and invalidate queries', async () => {
      const payload = { ip: '1.2.3.4', duration: '4h', type: 'ban' }
      vi.mocked(securityApi.createDecision).mockResolvedValue({ success: true })

      const { result } = renderHook(() => useCreateDecision(), { wrapper })

      result.current.mutate(payload)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(securityApi.createDecision).toHaveBeenCalledWith(payload)
    })
  })

  describe('useRuleSets', () => {
    it('should fetch rule sets', async () => {
      const mockRuleSets = {
        rulesets: [{
          id: 1,
          uuid: 'abc-123',
          name: 'OWASP CRS',
          source_url: 'https://example.com',
          mode: 'blocking',
          last_updated: '2025-12-04',
          content: 'rules'
        }]
      }
      vi.mocked(securityApi.getRuleSets).mockResolvedValue(mockRuleSets)

      const { result } = renderHook(() => useRuleSets(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).toEqual(mockRuleSets)
    })
  })

  describe('useUpsertRuleSet', () => {
    it('should upsert rule set and show success toast', async () => {
      const payload = { name: 'Custom Rules', content: 'rule data', mode: 'blocking' as const }
      vi.mocked(securityApi.upsertRuleSet).mockResolvedValue({ success: true })

      const { result } = renderHook(() => useUpsertRuleSet(), { wrapper })

      result.current.mutate(payload)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(securityApi.upsertRuleSet).toHaveBeenCalledWith(payload)
      expect(toast.success).toHaveBeenCalledWith('Rule set saved successfully')
    })

    it('should show error toast on failure', async () => {
      const error = new Error('Save failed')
      vi.mocked(securityApi.upsertRuleSet).mockRejectedValue(error)

      const { result } = renderHook(() => useUpsertRuleSet(), { wrapper })

      result.current.mutate({ name: 'Test', content: 'data', mode: 'blocking' })

      await waitFor(() => expect(result.current.isError).toBe(true))
      expect(toast.error).toHaveBeenCalledWith('Failed to save rule set: Save failed')
    })
  })

  describe('useDeleteRuleSet', () => {
    it('should delete rule set and show success toast', async () => {
      vi.mocked(securityApi.deleteRuleSet).mockResolvedValue({ success: true })

      const { result } = renderHook(() => useDeleteRuleSet(), { wrapper })

      result.current.mutate(1)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(securityApi.deleteRuleSet).toHaveBeenCalledWith(1)
      expect(toast.success).toHaveBeenCalledWith('Rule set deleted')
    })

    it('should show error toast on failure', async () => {
      const error = new Error('Delete failed')
      vi.mocked(securityApi.deleteRuleSet).mockRejectedValue(error)

      const { result } = renderHook(() => useDeleteRuleSet(), { wrapper })

      result.current.mutate(1)

      await waitFor(() => expect(result.current.isError).toBe(true))
      expect(toast.error).toHaveBeenCalledWith('Failed to delete rule set: Delete failed')
    })
  })

  describe('useEnableCerberus', () => {
    it('should enable Cerberus and show success toast', async () => {
      vi.mocked(securityApi.enableCerberus).mockResolvedValue({ success: true })

      const { result } = renderHook(() => useEnableCerberus(), { wrapper })

      result.current.mutate(undefined)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(securityApi.enableCerberus).toHaveBeenCalledWith(undefined)
      expect(toast.success).toHaveBeenCalledWith('Cerberus enabled')
    })

    it('should enable Cerberus with payload', async () => {
      const payload = { mode: 'full' }
      vi.mocked(securityApi.enableCerberus).mockResolvedValue({ success: true })

      const { result } = renderHook(() => useEnableCerberus(), { wrapper })

      result.current.mutate(payload)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(securityApi.enableCerberus).toHaveBeenCalledWith(payload)
    })

    it('should show error toast on failure', async () => {
      const error = new Error('Enable failed')
      vi.mocked(securityApi.enableCerberus).mockRejectedValue(error)

      const { result } = renderHook(() => useEnableCerberus(), { wrapper })

      result.current.mutate(undefined)

      await waitFor(() => expect(result.current.isError).toBe(true))
      expect(toast.error).toHaveBeenCalledWith('Failed to enable Cerberus: Enable failed')
    })
  })

  describe('useDisableCerberus', () => {
    it('should disable Cerberus and show success toast', async () => {
      vi.mocked(securityApi.disableCerberus).mockResolvedValue({ success: true })

      const { result } = renderHook(() => useDisableCerberus(), { wrapper })

      result.current.mutate(undefined)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(securityApi.disableCerberus).toHaveBeenCalledWith(undefined)
      expect(toast.success).toHaveBeenCalledWith('Cerberus disabled')
    })

    it('should disable Cerberus with payload', async () => {
      const payload = { reason: 'maintenance' }
      vi.mocked(securityApi.disableCerberus).mockResolvedValue({ success: true })

      const { result } = renderHook(() => useDisableCerberus(), { wrapper })

      result.current.mutate(payload)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(securityApi.disableCerberus).toHaveBeenCalledWith(payload)
    })

    it('should show error toast on failure', async () => {
      const error = new Error('Disable failed')
      vi.mocked(securityApi.disableCerberus).mockRejectedValue(error)

      const { result } = renderHook(() => useDisableCerberus(), { wrapper })

      result.current.mutate(undefined)

      await waitFor(() => expect(result.current.isError).toBe(true))
      expect(toast.error).toHaveBeenCalledWith('Failed to disable Cerberus: Disable failed')
    })
  })
})
