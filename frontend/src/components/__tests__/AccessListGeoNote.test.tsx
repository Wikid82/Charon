import { act, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as accessListsApi from '../../api/accessLists'
import * as securityApi from '../../api/security'
import { useAuth } from '../../hooks/useAuth'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { AccessListGeoNote } from '../AccessListGeoNote'

import type { AccessListSelfCheckDraft, AccessListSelfCheckResult } from '../../api/accessLists'
import type { GeoIPStatus } from '../../api/security'

vi.mock('../../api/accessLists')
vi.mock('../../api/security')
vi.mock('../../hooks/useAuth')

const draft: AccessListSelfCheckDraft = {
  type: 'geo_blacklist',
  country_codes: 'RU,CN',
  ip_rules: '',
  local_network_only: false,
  enabled: true,
}

const bundledStatus: GeoIPStatus = {
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

const unavailableStatus: GeoIPStatus = {
  ...bundledStatus,
  available: false,
  loaded: false,
  reason: 'geoip_unavailable',
  source: 'none',
  database_type: '',
  message: 'GeoIP database unavailable',
  unenforced_lists_total: 1,
}

const selfCheckResult = (overrides: Partial<AccessListSelfCheckResult> = {}): AccessListSelfCheckResult => ({
  ip_address: '203.0.113.7',
  country_code: 'RU',
  allowed: true,
  admin_exempt: false,
  geoip_available: true,
  reason: 'allowed',
  ...overrides,
})

const asAuth = (role: 'admin' | 'user' | null) =>
  ({ user: role === null ? null : { user_id: 1, role } }) as unknown as ReturnType<typeof useAuth>

/** Lets the status query and any debounced self-check settle before asserting that nothing is shown. */
const settle = async () => {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}

describe('<AccessListGeoNote />', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(useAuth).mockReturnValue(asAuth('admin'))
    vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(bundledStatus)
    vi.mocked(accessListsApi.selfCheckAccessList).mockResolvedValue(selfCheckResult())
  })

  describe('database not available (priority a)', () => {
    it('shows the not-enforced line for an admin', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(unavailableStatus)
      renderWithQueryClient(<AccessListGeoNote draft={draft} />)

      expect(await screen.findByRole('status')).toHaveTextContent(
        'This list will not be enforced until the GeoIP database is available.'
      )
    })

    it('shows the not-enforced line for a non-admin without running the self-check', async () => {
      vi.mocked(useAuth).mockReturnValue(asAuth('user'))
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(unavailableStatus)
      renderWithQueryClient(<AccessListGeoNote draft={draft} />)

      expect(await screen.findByRole('status')).toHaveTextContent('will not be enforced')
      await settle()
      expect(accessListsApi.selfCheckAccessList).not.toHaveBeenCalled()
    })

    it('takes priority over a self-check refusal', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(unavailableStatus)
      vi.mocked(accessListsApi.selfCheckAccessList).mockResolvedValue(
        selfCheckResult({ allowed: false, admin_exempt: false, reason: 'country_blocked' })
      )
      renderWithQueryClient(<AccessListGeoNote draft={draft} />)

      expect(await screen.findByRole('status')).toHaveTextContent('will not be enforced')
      await waitFor(() => expect(accessListsApi.selfCheckAccessList).toHaveBeenCalled())
      expect(screen.getByRole('status')).not.toHaveTextContent('would refuse you')
    })
  })

  describe('self-check (priority b, admin only)', () => {
    it('shows the would-refuse line when the viewer address is refused', async () => {
      vi.mocked(accessListsApi.selfCheckAccessList).mockResolvedValue(
        selfCheckResult({ allowed: false, admin_exempt: false, reason: 'country_blocked' })
      )
      renderWithQueryClient(<AccessListGeoNote draft={draft} />)

      expect(await screen.findByRole('status')).toHaveTextContent(
        'Based on the address you are using now, this list would refuse you. Saving is still allowed.'
      )
      expect(accessListsApi.selfCheckAccessList).toHaveBeenCalledWith(draft)
    })

    it('shows nothing when the refusal is admin-exempt', async () => {
      vi.mocked(accessListsApi.selfCheckAccessList).mockResolvedValue(
        selfCheckResult({ allowed: false, admin_exempt: true, reason: 'admin_exempt' })
      )
      renderWithQueryClient(<AccessListGeoNote draft={draft} />)

      await waitFor(() => expect(accessListsApi.selfCheckAccessList).toHaveBeenCalled())
      await settle()
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    })

    it('shows nothing when the address is allowed', async () => {
      renderWithQueryClient(<AccessListGeoNote draft={draft} />)

      await waitFor(() => expect(accessListsApi.selfCheckAccessList).toHaveBeenCalled())
      await settle()
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    })

    it('shows nothing when the self-check request fails', async () => {
      vi.mocked(accessListsApi.selfCheckAccessList).mockRejectedValue(new Error('request failed'))
      renderWithQueryClient(<AccessListGeoNote draft={draft} />)

      await waitFor(() => expect(accessListsApi.selfCheckAccessList).toHaveBeenCalled())
      await settle()
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    })

    it('does not run for a non-admin', async () => {
      vi.mocked(useAuth).mockReturnValue(asAuth('user'))
      vi.mocked(accessListsApi.selfCheckAccessList).mockResolvedValue(
        selfCheckResult({ allowed: false, admin_exempt: false })
      )
      renderWithQueryClient(<AccessListGeoNote draft={draft} />)

      await waitFor(() => expect(securityApi.getGeoIPStatus).toHaveBeenCalled())
      await settle()
      expect(accessListsApi.selfCheckAccessList).not.toHaveBeenCalled()
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    })

    it('does not run when there is no session user', async () => {
      vi.mocked(useAuth).mockReturnValue(asAuth(null))
      renderWithQueryClient(<AccessListGeoNote draft={draft} />)

      await waitFor(() => expect(securityApi.getGeoIPStatus).toHaveBeenCalled())
      await settle()
      expect(accessListsApi.selfCheckAccessList).not.toHaveBeenCalled()
    })

    it('does not run while the country list is empty', async () => {
      renderWithQueryClient(<AccessListGeoNote draft={{ ...draft, country_codes: '' }} />)

      await waitFor(() => expect(securityApi.getGeoIPStatus).toHaveBeenCalled())
      await settle()
      expect(accessListsApi.selfCheckAccessList).not.toHaveBeenCalled()
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    })
  })

  describe('status not known yet', () => {
    it('shows nothing while the status is loading', () => {
      vi.mocked(securityApi.getGeoIPStatus).mockReturnValue(new Promise(() => {}))
      renderWithQueryClient(<AccessListGeoNote draft={{ ...draft, country_codes: '' }} />)

      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    })

    it('shows nothing when the status request fails', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockRejectedValue(new Error('request failed'))
      renderWithQueryClient(<AccessListGeoNote draft={{ ...draft, country_codes: '' }} />)

      await waitFor(() => expect(securityApi.getGeoIPStatus).toHaveBeenCalled())
      await settle()
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    })
  })
})
