import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import ProxyHosts from '../ProxyHosts'
import * as proxyHostsApi from '../../api/proxyHosts'
import * as certificatesApi from '../../api/certificates'
import * as accessListsApi from '../../api/accessLists'
import * as settingsApi from '../../api/settings'
import type { Certificate } from '../../api/certificates'
import type { AccessList } from '../../api/accessLists'
import { createMockProxyHost } from '../../testUtils/createMockProxyHost'
import type { ProxyHost } from '../../api/proxyHosts'

vi.mock('react-hot-toast', () => ({ toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() } }))
vi.mock('../../api/proxyHosts', () => ({ getProxyHosts: vi.fn(), createProxyHost: vi.fn(), updateProxyHost: vi.fn(), deleteProxyHost: vi.fn(), bulkUpdateACL: vi.fn(), testProxyHostConnection: vi.fn() }))
vi.mock('../../api/certificates', () => ({ getCertificates: vi.fn() }))
vi.mock('../../api/accessLists', () => ({ accessListsApi: { list: vi.fn() } }))
vi.mock('../../api/settings', () => ({ getSettings: vi.fn() }))

const hosts = [
  createMockProxyHost({ uuid: 'p1', name: 'Progress 1', domain_names: 'p1.example.com' }),
  createMockProxyHost({ uuid: 'p2', name: 'Progress 2', domain_names: 'p2.example.com' }),
]

const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })
const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

describe('ProxyHosts - Bulk Apply progress UI', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue(hosts as ProxyHost[])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([] as Certificate[])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([] as AccessList[])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({} as Record<string, string>)
  })

  it('shows applying progress while updateProxyHost resolves', async () => {
    // Make updateProxyHost return controllable promises so we can assert the progress UI
    const updateMock = vi.mocked(proxyHostsApi.updateProxyHost)
        const resolvers: Array<(v: ProxyHost) => void> = []
        updateMock.mockImplementation(() => new Promise((res: (v: ProxyHost) => void) => { resolvers.push(res) }))
    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Progress 1')).toBeTruthy())

    // Select all
    const selectAll = screen.getAllByRole('checkbox')[0]
    await userEvent.click(selectAll)

    // Open Bulk Apply
    await userEvent.click(screen.getByText('Bulk Apply'))
    await waitFor(() => expect(screen.getByText('Bulk Apply Settings')).toBeTruthy())

    // Enable one setting (Force SSL)
    const forceLabel = screen.getByText(/Force SSL/i) as HTMLElement
    let forceContainer: HTMLElement | null = forceLabel
    while (forceContainer && !forceContainer.querySelector('input[type="checkbox"]')) forceContainer = forceContainer.parentElement
    const forceCheckbox = forceContainer ? (forceContainer.querySelector('input[type="checkbox"]') as HTMLElement | null) : null
    if (forceCheckbox) await userEvent.click(forceCheckbox as HTMLElement)

    // Click Apply and assert progress UI appears
    const modalRoot = screen.getByText('Bulk Apply Settings').closest('div')
    const { within } = await import('@testing-library/react')
    const applyButton = modalRoot ? within(modalRoot).getByRole('button', { name: /^Apply$/i }) : screen.getByRole('button', { name: /^Apply$/i })
    await userEvent.click(applyButton)

    // During the small delay the progress text should appear (there are two matching nodes)
    await waitFor(() => expect(screen.getAllByText(/Applying settings/i).length).toBeGreaterThan(0))

    // Resolve both pending update promises to finish the operation
    resolvers.forEach(r => r(hosts[0]))
    // Ensure subsequent tests aren't blocked by the special mock: make updateProxyHost resolve normally
    updateMock.mockImplementation(() => Promise.resolve(hosts[0] as ProxyHost))

    // Wait for updates to complete
    await waitFor(() => expect(updateMock).toHaveBeenCalledTimes(2))
  })
})

export {}
