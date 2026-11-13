import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest'

import * as accessListsApi from '../../api/accessLists'
import * as securityApi from '../../api/security'
import * as systemApi from '../../api/system'
import { ACTIVE_ALERTS_QUERY_KEY } from '../useActiveAlerts'
import {
  ACCESS_LIST_SELF_CHECK_DEBOUNCE_MS,
  GEOIP_STATUS_QUERY_KEY,
  useAccessListSelfCheck,
  useGeoIPLookup,
  useGeoIPStatus,
  useMyIPAddress,
  useRemoveGeoIPDatabase,
  useReloadGeoIP,
  useReplaceGeoIPDatabase,
} from '../useGeoIP'
import { SECURITY_STATUS_QUERY_KEY } from '../useSecurity'

vi.mock('../../api/accessLists')
vi.mock('../../api/security')
vi.mock('../../api/system')

const bundledStatus: securityApi.GeoIPStatus = {
  available: true,
  loaded: true,
  db_path: '/app/geoip/GeoLite2-Country.mmdb',
  message: 'GeoIP service available',
  source: 'bundled',
  database_type: 'GeoLite2-Country',
  build_date: null,
  size_bytes: null,
  loaded_at: null,
  included_build_date: null,
  data_copy_present: false,
  can_replace: true,
  replace_blocked_reason: '',
  unenforced_lists_total: 0,
}

/** Wraps a hook in a client and spies on invalidation so each test can check what was refreshed. */
function setup<T>(useHook: () => T) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
  const wrapper = ({ children }: { children: ReactNode }) => createElement(QueryClientProvider, { client: queryClient }, children)
  const view = renderHook(useHook, { wrapper })
  return { view, invalidate }
}

/** The query keys a spy saw invalidated. */
const invalidatedKeys = (spy: MockInstance) =>
  spy.mock.calls.map(([filters]) => (filters as { queryKey: readonly unknown[] }).queryKey)

/** The caches a database change must refresh. */
const REFRESHED_KEYS = [[...GEOIP_STATUS_QUERY_KEY], [...SECURITY_STATUS_QUERY_KEY], [...ACTIVE_ALERTS_QUERY_KEY]]

describe('GeoIP hooks', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('reads the status under the GeoIP cache key', async () => {
    vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(bundledStatus)
    const { view } = setup(() => useGeoIPStatus())

    await vi.waitFor(() => expect(view.result.current.data).toEqual(bundledStatus))
    expect(GEOIP_STATUS_QUERY_KEY).toEqual(['geoipStatus'])
  })

  it('reload refreshes the status, security status and active alerts on success', async () => {
    const { view, invalidate } = setup(() => useReloadGeoIP())
    vi.mocked(securityApi.reloadGeoIP).mockResolvedValue({})

    await act(async () => {
      await view.result.current.mutateAsync(undefined)
    })

    expect(invalidatedKeys(invalidate)).toEqual(expect.arrayContaining(REFRESHED_KEYS))
  })

  it('replace refreshes the status, security status and active alerts on success', async () => {
    const { view, invalidate } = setup(() => useReplaceGeoIPDatabase())
    vi.mocked(securityApi.replaceGeoIPDatabase).mockResolvedValue({ status: bundledStatus })

    await act(async () => {
      await view.result.current.mutateAsync(new File(['x'], 'GeoLite2-Country.mmdb'))
    })

    expect(securityApi.replaceGeoIPDatabase).toHaveBeenCalledTimes(1)
    expect(invalidatedKeys(invalidate)).toEqual(expect.arrayContaining(REFRESHED_KEYS))
  })

  it('remove refreshes the status, security status and active alerts on success', async () => {
    const { view, invalidate } = setup(() => useRemoveGeoIPDatabase())
    vi.mocked(securityApi.removeGeoIPDatabase).mockResolvedValue({ status: bundledStatus })

    await act(async () => {
      await view.result.current.mutateAsync(undefined)
    })

    expect(invalidatedKeys(invalidate)).toEqual(expect.arrayContaining(REFRESHED_KEYS))
  })

  it('still refreshes when a reload fails, because the proxy configuration is applied regardless', async () => {
    const { view, invalidate } = setup(() => useReloadGeoIP())
    vi.mocked(securityApi.reloadGeoIP).mockRejectedValue(new Error('reload failed'))

    await act(async () => {
      await expect(view.result.current.mutateAsync(undefined)).rejects.toThrow('reload failed')
    })

    expect(invalidatedKeys(invalidate)).toEqual(expect.arrayContaining(REFRESHED_KEYS))
  })

  it('does not refresh caches for a lookup, which changes nothing', async () => {
    vi.mocked(securityApi.lookupGeoIP).mockResolvedValue({ ip_address: '203.0.113.7', country_code: 'DE', found: true })
    const { view, invalidate } = setup(() => useGeoIPLookup())

    await act(async () => {
      await view.result.current.mutateAsync('203.0.113.7')
    })

    expect(securityApi.lookupGeoIP).toHaveBeenCalledWith('203.0.113.7')
    expect(invalidate).not.toHaveBeenCalled()
  })

  it('reads the address Charon sees for this browser', async () => {
    vi.mocked(systemApi.getMyIP).mockResolvedValue({ ip: '203.0.113.9', source: 'remote' })
    const { view } = setup(() => useMyIPAddress())

    let address = ''
    await act(async () => {
      address = (await view.result.current.mutateAsync()).ip
    })

    expect(address).toBe('203.0.113.9')
  })

  describe('useAccessListSelfCheck', () => {
    const draft = (countryCodes: string): accessListsApi.AccessListSelfCheckDraft => ({
      type: 'geo_blacklist',
      country_codes: countryCodes,
      ip_rules: '',
      local_network_only: false,
      enabled: true,
    })

    const result: accessListsApi.AccessListSelfCheckResult = {
      ip_address: '203.0.113.7',
      country_code: 'DE',
      allowed: true,
      admin_exempt: false,
      geoip_available: true,
      reason: 'allowed',
    }

    it('does not call the self-check while disabled', async () => {
      const { view } = setup(() => useAccessListSelfCheck(draft('DE'), false))

      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 0))
      })
      expect(view.result.current.fetchStatus).toBe('idle')
      expect(accessListsApi.selfCheckAccessList).not.toHaveBeenCalled()
    })

    it('checks the draft once it is enabled and resolves with the decision', async () => {
      vi.mocked(accessListsApi.selfCheckAccessList).mockResolvedValue(result)
      const { view } = setup(() => useAccessListSelfCheck(draft('DE'), true))

      await vi.waitFor(() => expect(view.result.current.data).toEqual(result))
      expect(accessListsApi.selfCheckAccessList).toHaveBeenCalledWith(draft('DE'))
    })

    it('does not check intermediate drafts typed within the debounce window', async () => {
      vi.mocked(accessListsApi.selfCheckAccessList).mockResolvedValue(result)
      const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
      const wrapper = ({ children }: { children: ReactNode }) => createElement(QueryClientProvider, { client: queryClient }, children)
      const { rerender, result: hook } = renderHook(
        ({ value }: { value: accessListsApi.AccessListSelfCheckDraft }) => useAccessListSelfCheck(value, true),
        { wrapper, initialProps: { value: draft('DE') } }
      )
      await vi.waitFor(() => expect(hook.current.data).toEqual(result))

      rerender({ value: draft('DE,FR') })
      rerender({ value: draft('DE,FR,IT') })

      await vi.waitFor(
        () => expect(accessListsApi.selfCheckAccessList).toHaveBeenLastCalledWith(draft('DE,FR,IT')),
        { timeout: ACCESS_LIST_SELF_CHECK_DEBOUNCE_MS * 4 }
      )
      const checkedCodes = vi.mocked(accessListsApi.selfCheckAccessList).mock.calls.map(([sent]) => sent.country_codes)
      expect(checkedCodes).toEqual(['DE', 'DE,FR,IT'])
    })
  })
})
