import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as securityApi from '../../api/security'
import * as systemApi from '../../api/system'
import { GeoIPLookupForm } from '../GeoIPLookupForm'

vi.mock('../../api/security')
vi.mock('../../api/system')

const refusal = (httpStatus: number, errorCode?: string): Error =>
  Object.assign(new Error('request failed'), {
    isAxiosError: true,
    response: { status: httpStatus, data: errorCode ? { error: 'refused', error_code: errorCode } : { error: 'refused' } },
  })

const renderForm = (loaded = true) => {
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  render(
    <QueryClientProvider client={queryClient}>
      <GeoIPLookupForm loaded={loaded} />
    </QueryClientProvider>
  )
  return {
    input: screen.getByRole('textbox', { name: 'IP address' }),
    lookup: screen.getByRole('button', { name: 'Look up' }),
    myAddress: screen.getByRole('button', { name: 'Use my address' }),
    result: () => screen.getByTestId('geoip-lookup-result'),
  }
}

describe('<GeoIPLookupForm />', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('disables the field and lookup with a hint while no database is loaded', () => {
    const view = renderForm(false)

    expect(view.input).toBeDisabled()
    expect(view.lookup).toBeDisabled()
    expect(screen.getByText('Lookup is available once a database is loaded.')).toBeInTheDocument()
  })

  it('keeps Look up disabled until an address is entered', () => {
    const view = renderForm()
    expect(view.lookup).toBeDisabled()

    fireEvent.change(view.input, { target: { value: '   ' } })
    expect(view.lookup).toBeDisabled()

    fireEvent.change(view.input, { target: { value: '203.0.113.7' } })
    expect(view.lookup).toBeEnabled()
  })

  it('looks up the trimmed address and names the country', async () => {
    vi.mocked(securityApi.lookupGeoIP).mockResolvedValue({ ip_address: '203.0.113.7', country_code: 'DE', found: true })
    const view = renderForm()

    fireEvent.change(view.input, { target: { value: '  203.0.113.7 ' } })
    fireEvent.click(view.lookup)

    await waitFor(() => expect(securityApi.lookupGeoIP).toHaveBeenCalledWith('203.0.113.7'))
    expect(await screen.findByText('203.0.113.7 is in Germany.')).toBeInTheDocument()
    expect(view.result()).toHaveAttribute('aria-live', 'polite')
  })

  it('falls back to the code when the country name cannot be resolved', async () => {
    vi.mocked(securityApi.lookupGeoIP).mockResolvedValue({ ip_address: '198.51.100.1', country_code: 'X1', found: true })
    const view = renderForm()

    fireEvent.change(view.input, { target: { value: '198.51.100.1' } })
    fireEvent.click(view.lookup)

    expect(await screen.findByText(/198\.51\.100\.1 is in/)).toBeInTheDocument()
  })

  it('reports that no country was found', async () => {
    vi.mocked(securityApi.lookupGeoIP).mockResolvedValue({
      ip_address: '127.0.0.1',
      country_code: '',
      found: false,
      message: 'No country found for this IP address',
    })
    const view = renderForm()

    fireEvent.change(view.input, { target: { value: '127.0.0.1' } })
    fireEvent.click(view.lookup)

    expect(await screen.findByText('No country found for 127.0.0.1.')).toBeInTheDocument()
  })

  it('reports an unparseable address (400)', async () => {
    vi.mocked(securityApi.lookupGeoIP).mockRejectedValue(refusal(400))
    const view = renderForm()

    fireEvent.change(view.input, { target: { value: 'not-an-ip' } })
    fireEvent.click(view.lookup)

    expect(await screen.findByText('Enter a valid IP address.')).toBeInTheDocument()
  })

  it('reports that no database is loaded (503)', async () => {
    vi.mocked(securityApi.lookupGeoIP).mockRejectedValue(refusal(503))
    const view = renderForm()

    fireEvent.change(view.input, { target: { value: '203.0.113.7' } })
    fireEvent.click(view.lookup)

    expect(await screen.findByText('No database is loaded, so addresses cannot be looked up.')).toBeInTheDocument()
  })

  it('maps a coded refusal to its text', async () => {
    vi.mocked(securityApi.lookupGeoIP).mockRejectedValue(refusal(409, 'geoip_busy'))
    const view = renderForm()

    fireEvent.change(view.input, { target: { value: '203.0.113.7' } })
    fireEvent.click(view.lookup)

    expect(await screen.findByText('Another database change is in progress. Try again in a moment.')).toBeInTheDocument()
  })

  it('fills the field with the address Charon sees for this browser', async () => {
    vi.mocked(systemApi.getMyIP).mockResolvedValue({ ip: '203.0.113.9', source: 'remote' })
    const view = renderForm()

    fireEvent.click(view.myAddress)

    await waitFor(() => expect(view.input).toHaveValue('203.0.113.9'))
    expect(systemApi.getMyIP).toHaveBeenCalledTimes(1)
  })

  it('reports a failure to read the browser address', async () => {
    vi.mocked(systemApi.getMyIP).mockRejectedValue(new Error('offline'))
    const view = renderForm()

    fireEvent.click(view.myAddress)

    expect(await screen.findByText('Charon could not read your address.')).toBeInTheDocument()
    expect(view.input).toHaveValue('')
  })
})
