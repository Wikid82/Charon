import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { act } from 'react'
import type { ProxyHost } from '../../api/proxyHosts'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// We'll use per-test module mocks via `vi.doMock` and dynamic imports to avoid
// leaking mocks into other tests. Each test creates its own QueryClient.

describe('ProxyHosts page - coverage targets (isolated)', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  const renderPage = async () => {
    // Dynamic mocks
    const mockUpdateHost = vi.fn()

    vi.doMock('react-hot-toast', () => ({ toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() } }))

    vi.doMock('../../hooks/useProxyHosts', () => ({
      useProxyHosts: vi.fn(() => ({
        hosts: [
          {
            uuid: 'host-1',
            name: 'StagingHost',
            domain_names: 'staging.example.com',
            forward_scheme: 'http',
            forward_host: '10.0.0.1',
            forward_port: 80,
            ssl_forced: true,
            websocket_support: true,
            certificate: undefined,
            enabled: true,
            created_at: '2025-01-01',
            updated_at: '2025-01-01',
          },
          {
            uuid: 'host-2',
            name: 'CustomCertHost',
            domain_names: 'custom.example.com',
            forward_scheme: 'http',
            forward_host: '10.0.0.2',
            forward_port: 8080,
            ssl_forced: false,
            websocket_support: false,
            certificate: { provider: 'custom', name: 'ACME-CUSTOM' },
            enabled: false,
            created_at: '2025-01-01',
            updated_at: '2025-01-01',
          }
        ],
        loading: false,
        isFetching: false,
        error: null,
        createHost: vi.fn(),
        updateHost: (uuid: string, data: Partial<ProxyHost>) => mockUpdateHost(uuid, data),
        deleteHost: vi.fn(),
        bulkUpdateACL: vi.fn(),
        isBulkUpdating: false,
      }))
    }))

    vi.doMock('../../hooks/useCertificates', () => ({
      useCertificates: vi.fn(() => ({
        certificates: [
          { id: 1, name: 'StagingCert', domain: 'staging.example.com', status: 'untrusted', provider: 'letsencrypt-staging' }
        ],
        isLoading: false,
        error: null,
      }))
    }))

    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))

    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({ 'ui.domain_link_behavior': 'new_window' })) }))

    // Import page after mocks are in place
    const { default: ProxyHosts } = await import('../ProxyHosts')

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const wrapper = (ui: React.ReactNode) => (
      <QueryClientProvider client={qc}>{ui}</QueryClientProvider>
    )

    return { ProxyHosts, mockUpdateHost, wrapper }
  }

  it('renders SSL staging badge, websocket badge and custom cert text', async () => {
    const { ProxyHosts } = await renderPage()

    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <ProxyHosts />
      </QueryClientProvider>
    )

    await waitFor(() => expect(screen.getByText('StagingHost')).toBeInTheDocument())

    expect(screen.getByText(/SSL \(Staging\)/)).toBeInTheDocument()
    expect(screen.getByText('WS')).toBeInTheDocument()
    expect(screen.getByText('ACME-CUSTOM (Custom)')).toBeInTheDocument()
  })

  it('opens domain link in new window when linkBehavior is new_window', async () => {
    const { ProxyHosts } = await renderPage()

    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)

    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <ProxyHosts />
      </QueryClientProvider>
    )

    await waitFor(() => expect(screen.getByText('staging.example.com')).toBeInTheDocument())
    const link = screen.getByText('staging.example.com').closest('a') as HTMLAnchorElement
    await act(async () => {
      await userEvent.click(link!)
    })

    expect(openSpy).toHaveBeenCalled()
    openSpy.mockRestore()
  })

  it('bulk apply merges host data and calls updateHost', async () => {
    const { ProxyHosts, mockUpdateHost } = await renderPage()

    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <ProxyHosts />
      </QueryClientProvider>
    )

    await waitFor(() => expect(screen.getByText('StagingHost')).toBeInTheDocument())

    const selectBtn1 = screen.getByLabelText('Select StagingHost')
    const selectBtn2 = screen.getByLabelText('Select CustomCertHost')
    await userEvent.click(selectBtn1)
    await userEvent.click(selectBtn2)

    const bulkBtn = screen.getByText('Bulk Apply')
    await userEvent.click(bulkBtn)

    const modal = screen.getByText('Bulk Apply Settings').closest('div')!
    const modalWithin = within(modal)

    const checkboxes = modal.querySelectorAll('input[type="checkbox"]')
    expect(checkboxes.length).toBeGreaterThan(0)
    await userEvent.click(checkboxes[0])

    const applyBtn = modalWithin.getByRole('button', { name: /Apply/ })
    await userEvent.click(applyBtn)

    await waitFor(() => {
      expect(mockUpdateHost).toHaveBeenCalled()
    })

    const calls = vi.mocked(mockUpdateHost).mock.calls
    expect(calls.length).toBeGreaterThanOrEqual(1)
    const [calledUuid, calledData] = calls[0]
    expect(typeof calledUuid).toBe('string')
    expect(Object.prototype.hasOwnProperty.call(calledData, 'ssl_forced')).toBe(true)
  })
})

export {}
