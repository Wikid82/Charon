import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'

import * as securityApi from '../../api/security'
import { useAuth } from '../../hooks/useAuth'
import { parseTrustedProxyLines } from '../../utils/trustedProxyLines'
import { TrustedProxiesCard } from '../TrustedProxiesCard'

vi.mock('../../api/security')
vi.mock('../../hooks/useAuth', () => ({ useAuth: vi.fn() }))

type Response = securityApi.TrustedProxiesResponse

const adminResponse = (overrides: Partial<Response> = {}): Response => ({
  proxies: ['172.18.0.5/32'],
  source: 'setting',
  dropped_entries: [],
  always_trusted: ['127.0.0.1/32', '::1/128'],
  api_layer: [],
  warnings: [],
  ...overrides,
})

const standardResponse = (): Response => ({
  source: 'environment',
  always_trusted: ['127.0.0.1/32', '::1/128'],
  has_proxies: true,
})

/** An HTTP refusal the way the shared client surfaces it: an AxiosError-shaped error with a response. */
const refusal = (status: number, data: Record<string, unknown>) =>
  Object.assign(new Error(`Request failed with status code ${status}`), {
    isAxiosError: true,
    response: { status, data },
  })

const renderCard = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <TrustedProxiesCard />
    </QueryClientProvider>,
  )
}

const asAdmin = () => vi.mocked(useAuth).mockReturnValue({ user: { role: 'admin' } } as unknown as ReturnType<typeof useAuth>)
const asStandardUser = () =>
  vi.mocked(useAuth).mockReturnValue({ user: { role: 'user' } } as unknown as ReturnType<typeof useAuth>)

/** The card region once the list has loaded (the region exists while the skeleton shows). */
const findCard = async () => {
  const card = await screen.findByRole('region', { name: 'Trusted proxies' })
  await waitFor(() => expect(within(card).queryByTestId('trusted-proxies-skeleton')).not.toBeInTheDocument())
  return card
}

describe('parseTrustedProxyLines (shared with the card)', () => {
  it('splits on line breaks, trims, and drops blank lines', () => {
    expect(parseTrustedProxyLines(' 10.0.0.1 \r\n\n  fd00::5/128\n')).toEqual(['10.0.0.1', 'fd00::5/128'])
  })

  it('returns no entries for an empty field', () => {
    expect(parseTrustedProxyLines('   \n')).toEqual([])
  })
})

describe('<TrustedProxiesCard />', () => {
  beforeEach(() => {
    vi.mocked(securityApi.getTrustedProxies).mockReset()
    vi.mocked(securityApi.saveTrustedProxies).mockReset()
    vi.mocked(securityApi.resetTrustedProxies).mockReset()
    asAdmin()
  })

  it('shows a skeleton while loading', () => {
    vi.mocked(securityApi.getTrustedProxies).mockReturnValue(new Promise(() => {}))
    renderCard()

    expect(screen.getByTestId('trusted-proxies-skeleton')).toBeInTheDocument()
  })

  it('shows a load error when the list cannot be read', async () => {
    vi.mocked(securityApi.getTrustedProxies).mockRejectedValue(new Error('down'))
    renderCard()

    expect(await screen.findByText('The trusted proxy list could not be loaded.')).toBeInTheDocument()
  })

  it('shows the saved list, the loopback entries and the API-layer note to an administrator', async () => {
    vi.mocked(securityApi.getTrustedProxies).mockResolvedValue(adminResponse({ api_layer: ['10.9.0.0/16'] }))
    renderCard()

    const card = await findCard()
    expect(within(card).getByRole('textbox', { name: 'Trusted proxies' })).toHaveValue('172.18.0.5/32')
    expect(card).toHaveTextContent('Always trusted: 127.0.0.1/32, ::1/128')
    expect(card).toHaveTextContent('CHARON_TRUSTED_PROXIES')
    expect(card).toHaveTextContent('10.9.0.0/16')
    expect(within(card).getByRole('button', { name: 'Save' })).toBeInTheDocument()
    expect(within(card).getByRole('button', { name: 'Reset to default' })).toBeInTheDocument()
  })

  it('names the environment as the source, lists dropped entries, and offers no reset', async () => {
    vi.mocked(securityApi.getTrustedProxies).mockResolvedValue(
      adminResponse({ proxies: [], source: 'environment', dropped_entries: ['8.0.0.0/4'] }),
    )
    renderCard()

    const card = await findCard()
    expect(card).toHaveTextContent('Using CHARON_TRUSTED_PROXIES from the environment')
    expect(card).toHaveTextContent('8.0.0.0/4')
    expect(within(card).queryByRole('button', { name: 'Reset to default' })).not.toBeInTheDocument()
  })

  it('names the default when nothing is set anywhere', async () => {
    vi.mocked(securityApi.getTrustedProxies).mockResolvedValue(adminResponse({ proxies: [], source: 'default' }))
    renderCard()

    expect(await findCard()).toHaveTextContent('Using the default, loopback only')
  })

  it('saves each non-blank line as one entry and reports the saved list', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getTrustedProxies).mockResolvedValue(adminResponse())
    vi.mocked(securityApi.saveTrustedProxies).mockResolvedValue(
      adminResponse({ proxies: ['10.0.0.0/8', '172.18.0.5/32'], warnings: ['10.0.0.0/8 is a wide range; every address in it is trusted as a proxy'] }),
    )
    renderCard()

    const card = await findCard()
    const field = within(card).getByRole('textbox', { name: 'Trusted proxies' })
    await user.clear(field)
    await user.type(field, '10.0.0.1{enter}  172.18.0.5 {enter}{enter}')
    await user.click(within(card).getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(securityApi.saveTrustedProxies).toHaveBeenCalledWith(['10.0.0.1', '172.18.0.5']),
    )
    expect(await within(card).findByRole('status')).toHaveTextContent('Trusted proxies saved.')
    expect(card).toHaveTextContent('10.0.0.0/8 is a wide range')
  })

  it('names the refused entry under the field and keeps the typed text', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getTrustedProxies).mockResolvedValue(adminResponse())
    vi.mocked(securityApi.saveTrustedProxies).mockRejectedValue(
      refusal(400, { error_code: 'trusted_proxy_invalid', entry: 'not-an-address' }),
    )
    renderCard()

    const card = await findCard()
    const field = within(card).getByRole('textbox', { name: 'Trusted proxies' })
    await user.clear(field)
    await user.type(field, 'not-an-address')
    await user.click(within(card).getByRole('button', { name: 'Save' }))

    expect(await within(card).findByRole('alert')).toHaveTextContent('"not-an-address" is not an IP address or CIDR range.')
    expect(field).toHaveValue('not-an-address')
    expect(within(card).queryByRole('status')).not.toBeInTheDocument()
  })

  it('explains a too-broad entry', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getTrustedProxies).mockResolvedValue(adminResponse())
    vi.mocked(securityApi.saveTrustedProxies).mockRejectedValue(
      refusal(400, { error_code: 'trusted_proxy_too_broad', entry: '0.0.0.0/0' }),
    )
    renderCard()

    const card = await findCard()
    await user.click(within(card).getByRole('button', { name: 'Save' }))

    expect(await within(card).findByRole('alert')).toHaveTextContent('"0.0.0.0/0" covers too wide a range')
  })

  it('explains a list over the entry limit', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getTrustedProxies).mockResolvedValue(adminResponse())
    vi.mocked(securityApi.saveTrustedProxies).mockRejectedValue(refusal(400, { error_code: 'trusted_proxies_too_many' }))
    renderCard()

    const card = await findCard()
    await user.click(within(card).getByRole('button', { name: 'Save' }))

    expect(await within(card).findByRole('alert')).toHaveTextContent('at most 64 entries')
  })

  it('shows the generic failure text for an unexpected refusal', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getTrustedProxies).mockResolvedValue(adminResponse())
    vi.mocked(securityApi.saveTrustedProxies).mockRejectedValue(
      refusal(500, { error: 'Failed to reload configuration' }),
    )
    renderCard()

    const card = await findCard()
    await user.click(within(card).getByRole('button', { name: 'Save' }))

    expect(await within(card).findByRole('alert')).toHaveTextContent('The trusted proxy list could not be saved.')
  })

  it('clears the saved list on reset and reports it', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getTrustedProxies).mockResolvedValue(adminResponse())
    vi.mocked(securityApi.resetTrustedProxies).mockResolvedValue(
      adminResponse({ proxies: [], source: 'default' }),
    )
    renderCard()

    const card = await findCard()
    await user.click(within(card).getByRole('button', { name: 'Reset to default' }))

    expect(securityApi.resetTrustedProxies).toHaveBeenCalledTimes(1)
    expect(await within(card).findByRole('status')).toHaveTextContent('Saved list cleared.')
  })

  it('shows the reset failure through the generic text', async () => {
    const user = userEvent.setup()
    vi.mocked(securityApi.getTrustedProxies).mockResolvedValue(adminResponse())
    vi.mocked(securityApi.resetTrustedProxies).mockRejectedValue(refusal(500, { error: 'Failed to reload configuration' }))
    renderCard()

    const card = await findCard()
    await user.click(within(card).getByRole('button', { name: 'Reset to default' }))

    expect(await within(card).findByRole('alert')).toHaveTextContent('The trusted proxy list could not be saved.')
  })

  it('hides the editable list and shows the administrators-only note to a standard user', async () => {
    asStandardUser()
    vi.mocked(securityApi.getTrustedProxies).mockResolvedValue(standardResponse())
    renderCard()

    const card = await findCard()
    expect(within(card).queryByRole('textbox')).not.toBeInTheDocument()
    expect(within(card).queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
    expect(card).toHaveTextContent('Only administrators can change this setting.')
    expect(card).toHaveTextContent('Using CHARON_TRUSTED_PROXIES from the environment')
    expect(card).not.toHaveTextContent('CHARON_TRUSTED_PROXIES on the server')
  })
})
