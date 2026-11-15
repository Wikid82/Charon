import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'

import * as securityApi from '../../api/security'
import { useAuth } from '../../hooks/useAuth'
import { ManagementAccessCard } from '../ManagementAccessCard'

import type { AccessList } from '../../api/accessLists'

vi.mock('../../api/security')
vi.mock('../../hooks/useAuth', () => ({ useAuth: vi.fn() }))

const list = (overrides: Partial<AccessList> = {}): AccessList =>
  ({
    uuid: 'list-office',
    name: 'Office only',
    type: 'whitelist',
    enabled: true,
    description: '',
    ip_rules: '[]',
    country_codes: '',
    local_network_only: false,
    created_at: '',
    updated_at: '',
    ...overrides,
  }) as AccessList

const ALL_LISTS = [list(), list({ uuid: 'list-block', name: 'Scanners', type: 'blacklist', enabled: false })]

const stored = (overrides: Partial<securityApi.ManagementAccessResponse> = {}): securityApi.ManagementAccessResponse => ({
  lists: [],
  enforced: false,
  ...overrides,
})

/** An HTTP refusal the way the shared client surfaces it: an AxiosError-shaped error with a response. */
const refusal = (status: number, data: Record<string, unknown>) =>
  Object.assign(new Error(`Request failed with status code ${status}`), {
    isAxiosError: true,
    response: { status, data },
  })

const asAdmin = () => vi.mocked(useAuth).mockReturnValue({ user: { role: 'admin' } } as unknown as ReturnType<typeof useAuth>)
const asStandardUser = () => vi.mocked(useAuth).mockReturnValue({ user: { role: 'user' } } as unknown as ReturnType<typeof useAuth>)

const renderCard = (accessLists: AccessList[] = ALL_LISTS) => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <ManagementAccessCard accessLists={accessLists} />
    </QueryClientProvider>,
  )
}

/** The card region once the stored set has loaded (the region exists while the skeleton shows). */
const findCard = async () => {
  const card = await screen.findByRole('region', { name: 'Management access' })
  await waitFor(() => expect(within(card).queryByTestId('management-access-skeleton')).not.toBeInTheDocument())
  return card
}

describe('<ManagementAccessCard />', () => {
  beforeEach(() => {
    vi.mocked(securityApi.getManagementAccess).mockReset()
    vi.mocked(securityApi.saveManagementAccess).mockReset()
    asAdmin()
  })

  it('shows a skeleton while loading', () => {
    vi.mocked(securityApi.getManagementAccess).mockReturnValue(new Promise(() => {}))
    renderCard()

    expect(screen.getByTestId('management-access-skeleton')).toBeInTheDocument()
  })

  it('shows a load error when the set cannot be read', async () => {
    vi.mocked(securityApi.getManagementAccess).mockRejectedValue(new Error('down'))
    renderCard()

    expect(await screen.findByText('Management access could not be loaded.')).toBeInTheDocument()
  })

  it('shows not restricted with every access list unchecked when none is chosen', async () => {
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(stored())
    renderCard()

    const card = await findCard()
    expect(within(card).getByText('Not restricted')).toBeInTheDocument()
    expect(within(card).getByRole('checkbox', { name: /Office only/ })).not.toBeChecked()
    expect(within(card).getByRole('checkbox', { name: /Scanners/ })).not.toBeChecked()
    expect(within(card).getByRole('button', { name: 'Save' })).toBeInTheDocument()
  })

  it('shows the chosen lists as checked and explains a set that is not applied', async () => {
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(
      stored({ lists: [{ uuid: 'list-office', name: 'Office only', type: 'whitelist', enabled: true }], enforced: false }),
    )
    renderCard()

    const card = await findCard()
    expect(within(card).getByRole('checkbox', { name: /Office only/ })).toBeChecked()
    expect(within(card).queryByText('Not restricted')).not.toBeInTheDocument()
    expect(within(card).getByText('Not applied while access control is off or no chosen list is enabled.')).toBeInTheDocument()
  })

  it('does not explain anything when the chosen set is applied', async () => {
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(
      stored({ lists: [{ uuid: 'list-office', name: 'Office only', type: 'whitelist', enabled: true }], enforced: true }),
    )
    renderCard()

    const card = await findCard()
    expect(within(card).queryByText(/Not applied/)).not.toBeInTheDocument()
  })

  it('labels a disabled list and a deny list in the choice', async () => {
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(stored())
    renderCard()

    const card = await findCard()
    expect(card).toHaveTextContent(/Scanners\s*Deny\s*·\s*Disabled/)
  })

  it('shows the empty-lists hint when there are no access lists', async () => {
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(stored())
    renderCard([])

    const card = await findCard()
    expect(within(card).getByText('No access lists exist yet. Create one first, then choose it here.')).toBeInTheDocument()
  })

  it('saves the chosen UUIDs and reports the stored set', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(stored())
    vi.mocked(securityApi.saveManagementAccess).mockResolvedValue(
      stored({ lists: [{ uuid: 'list-office', name: 'Office only', type: 'whitelist', enabled: true }], enforced: true }),
    )
    renderCard()

    const card = await findCard()
    await user.click(within(card).getByRole('checkbox', { name: /Office only/ }))
    expect(within(card).getByText('Only visitors allowed by the chosen lists can use Charon\'s own interface.')).toBeInTheDocument()
    await user.click(within(card).getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(securityApi.saveManagementAccess).toHaveBeenCalledWith(['list-office']))
    expect(await within(card).findByRole('status')).toHaveTextContent('Management access saved.')
  })

  it('removes a list again when it is unchecked before saving', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(
      stored({ lists: [{ uuid: 'list-office', name: 'Office only', type: 'whitelist', enabled: true }], enforced: true }),
    )
    vi.mocked(securityApi.saveManagementAccess).mockResolvedValue(stored())
    renderCard()

    const card = await findCard()
    await user.click(within(card).getByRole('checkbox', { name: /Office only/ }))
    await user.click(within(card).getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(securityApi.saveManagementAccess).toHaveBeenCalledWith([]))
  })

  it('explains a choice that would lock the administrator out', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(stored())
    vi.mocked(securityApi.saveManagementAccess).mockRejectedValue(
      refusal(403, { error: 'refused', error_code: 'management_access_would_lock_out' }),
    )
    renderCard()

    const card = await findCard()
    await user.click(within(card).getByRole('checkbox', { name: /Office only/ }))
    await user.click(within(card).getByRole('button', { name: 'Save' }))

    expect(await within(card).findByRole('alert')).toHaveTextContent(/would lock you out/i)
    expect(within(card).queryByRole('status')).not.toBeInTheDocument()
  })

  it('explains a list that no longer exists', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(stored())
    vi.mocked(securityApi.saveManagementAccess).mockRejectedValue(
      refusal(400, { error: 'access list not found', error_code: 'management_list_unknown', uuid: 'list-office' }),
    )
    renderCard()

    const card = await findCard()
    await user.click(within(card).getByRole('checkbox', { name: /Office only/ }))
    await user.click(within(card).getByRole('button', { name: 'Save' }))

    expect(await within(card).findByRole('alert')).toHaveTextContent('One of the chosen access lists no longer exists.')
  })

  it('shows the generic failure text for an unexpected refusal', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(stored())
    vi.mocked(securityApi.saveManagementAccess).mockRejectedValue(refusal(500, { error: 'failed to save management access' }))
    renderCard()

    const card = await findCard()
    await user.click(within(card).getByRole('button', { name: 'Save' }))

    expect(await within(card).findByRole('alert')).toHaveTextContent('The management access choice could not be saved.')
  })

  it('shows the chosen names read-only and the administrators-only note to a standard user', async () => {
    asStandardUser()
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(
      stored({ lists: [{ uuid: 'list-office', name: 'Office only', type: 'whitelist', enabled: true }], enforced: true }),
    )
    renderCard()

    const card = await findCard()
    expect(within(card).queryByRole('checkbox')).not.toBeInTheDocument()
    expect(within(card).queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
    expect(within(card).getByText('Office only')).toBeInTheDocument()
    expect(within(card).getByText('Only administrators can change this setting.')).toBeInTheDocument()
  })

  it('shows not restricted to a standard user when no list is chosen', async () => {
    asStandardUser()
    vi.mocked(securityApi.getManagementAccess).mockResolvedValue(stored())
    renderCard()

    const card = await findCard()
    expect(within(card).getByText('Not restricted')).toBeInTheDocument()
  })
})
