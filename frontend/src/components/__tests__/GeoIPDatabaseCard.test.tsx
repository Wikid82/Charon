import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as securityApi from '../../api/security'
import { GEOIP_TROUBLESHOOTING_DOCS_URL } from '../../constants/docs'
import { GeoIPDatabaseCard } from '../GeoIPDatabaseCard'

import type { GeoIPStatus } from '../../api/security'

vi.mock('../../api/security')

const bundled: GeoIPStatus = {
  available: true,
  loaded: true,
  db_path: '/app/geoip/GeoLite2-Country.mmdb',
  message: 'GeoIP service available',
  source: 'bundled',
  database_type: 'GeoLite2-Country',
  build_date: '2026-11-04T07:12:33Z',
  size_bytes: 9_876_543,
  loaded_at: '2026-11-10T05:00:01Z',
  included_build_date: '2026-11-04T07:12:33Z',
  data_copy_present: false,
  can_replace: true,
  replace_blocked_reason: '',
  unenforced_lists_total: 0,
}

const status = (overrides: Partial<GeoIPStatus> = {}): GeoIPStatus => ({ ...bundled, ...overrides })

/** An error shaped like an Axios refusal carrying the server's error_code. */
const refusal = (errorCode: string | undefined, httpStatus = 400): Error =>
  Object.assign(new Error('request failed'), {
    isAxiosError: true,
    response: { status: httpStatus, data: errorCode ? { error: 'refused', error_code: errorCode } : { error: 'refused' } },
  })

const openCard = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={queryClient}>
      <GeoIPDatabaseCard />
    </QueryClientProvider>
  )
  return screen.findByRole('region', { name: 'Country database (GeoIP)' })
}

const statusRegion = (region: HTMLElement) => within(region).getByRole('status')

describe('<GeoIPDatabaseCard />', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(bundled)
  })

  describe('states', () => {
    it('shows a skeleton while the status loads', () => {
      vi.mocked(securityApi.getGeoIPStatus).mockReturnValue(new Promise(() => {}))
      const queryClient = new QueryClient()
      render(
        <QueryClientProvider client={queryClient}>
          <GeoIPDatabaseCard />
        </QueryClientProvider>
      )
      expect(screen.getByTestId('geoip-card-skeleton')).toBeInTheDocument()
    })

    it('shows a load error when the status request fails', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockRejectedValue(new Error('network'))
      const region = await openCard()
      expect(await within(region).findByText('The database status could not be loaded.')).toBeInTheDocument()
      expect(within(region).queryByRole('button', { name: 'Reload' })).not.toBeInTheDocument()
    })

    it('shows the bundled source, path, size and build date with Reload and Replace', async () => {
      const region = await openCard()

      expect(await within(region).findByText('Ready')).toBeInTheDocument()
      expect(region).toHaveTextContent('Included with Charon')
      expect(region).toHaveTextContent('/app/geoip/GeoLite2-Country.mmdb')
      expect(region).toHaveTextContent('9.9 MB')
      expect(region).toHaveTextContent('Nov 4, 2026')
      expect(within(region).getByRole('button', { name: 'Reload' })).toBeInTheDocument()
      expect(within(region).getByRole('button', { name: 'Replace database…' })).toBeInTheDocument()
    })

    it('offers no removal action for the bundled source', async () => {
      const region = await openCard()
      await within(region).findByText('Ready')
      expect(within(region).queryByRole('button', { name: 'Use the included copy' })).not.toBeInTheDocument()
    })

    it('shows unknown values and the not-enforced count when the database is unavailable', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(
        status({
          available: false,
          loaded: false,
          source: 'none',
          reason: 'geoip_unavailable',
          build_date: null,
          size_bytes: null,
          included_build_date: null,
          unenforced_lists_total: 2,
        })
      )
      const region = await openCard()

      expect(await within(region).findByText('Not available')).toBeInTheDocument()
      expect(region).toHaveTextContent('Unknown')
      expect(region).toHaveTextContent('Not available. Country lists are not enforced until a database is loaded.')
      expect(region).toHaveTextContent('2 country list(s) are not enforced')
      const docs = within(region).getByRole('link', { name: 'How to provide a database' })
      expect(docs).toHaveAttribute('href', GEOIP_TROUBLESHOOTING_DOCS_URL)
      expect(docs).toHaveAttribute('target', '_blank')
      expect(docs).toHaveAttribute('rel', 'noopener noreferrer')
    })

    it('shows the data-folder source with the removal action', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(status({ source: 'data', data_copy_present: true }))
      const region = await openCard()

      expect(await within(region).findByText('Your data folder')).toBeInTheDocument()
      expect(within(region).getByRole('button', { name: 'Use the included copy' })).toBeInTheDocument()
    })

    it('hides Replace and removal and explains a server-set custom path, keeping Reload', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(
        status({ source: 'custom', can_replace: false, replace_blocked_reason: 'custom_path_set' })
      )
      const region = await openCard()

      expect(await within(region).findByText('Custom path set on the server')).toBeInTheDocument()
      expect(region).toHaveTextContent('The database path is set by the server (CHARON_GEOIP_DB_PATH)')
      expect(within(region).getByRole('button', { name: 'Reload' })).toBeInTheDocument()
      expect(within(region).queryByRole('button', { name: 'Replace database…' })).not.toBeInTheDocument()
      expect(within(region).queryByRole('button', { name: 'Use the included copy' })).not.toBeInTheDocument()
      expect(within(region).queryByText(/up to 32 MiB/)).not.toBeInTheDocument()
    })

    it('hides Replace and removal on a read-only data folder and explains why', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(
        status({ source: 'data', can_replace: false, replace_blocked_reason: 'data_folder_read_only' })
      )
      const region = await openCard()

      expect(await within(region).findByText('The data folder is read-only, so the database cannot be replaced here.')).toBeInTheDocument()
      expect(within(region).queryByRole('button', { name: 'Replace database…' })).not.toBeInTheDocument()
      expect(within(region).queryByRole('button', { name: 'Use the included copy' })).not.toBeInTheDocument()
    })

    it('omits the size, date and path fields when they are unknown', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(
        status({ build_date: 'not a date', size_bytes: null, db_path: '' })
      )
      const region = await openCard()
      await within(region).findByText('Ready')
      expect(region).not.toHaveTextContent('Nov 4, 2026')
      expect(region).toHaveTextContent('Unknown')
    })

    it('uses the no-bundled confirmation text when the image ships no database', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(
        status({ source: 'data', data_copy_present: true, included_build_date: null })
      )
      const region = await openCard()
      fireEvent.click(await within(region).findByRole('button', { name: 'Use the included copy' }))

      const dialog = await screen.findByRole('dialog')
      expect(dialog).toHaveTextContent('Country lists will not be enforced until you provide a database.')
    })
  })

  describe('reload', () => {
    it('reports success inline in a polite status region, not as a toast', async () => {
      vi.mocked(securityApi.reloadGeoIP).mockResolvedValue({ message: 'ok' })
      const region = await openCard()
      await within(region).findByText('Ready')

      fireEvent.click(within(region).getByRole('button', { name: 'Reload' }))

      await waitFor(() => expect(statusRegion(region)).toHaveTextContent('Database reloaded.'))
      expect(statusRegion(region)).toHaveAttribute('aria-live', 'polite')
      expect(statusRegion(region)).toHaveClass('text-content-secondary')
    })

    it('reports a failed reload with the mapped error text', async () => {
      vi.mocked(securityApi.reloadGeoIP).mockRejectedValue(refusal('geoip_reload_failed', 500))
      const region = await openCard()
      await within(region).findByText('Ready')

      fireEvent.click(within(region).getByRole('button', { name: 'Reload' }))

      await waitFor(() => expect(statusRegion(region)).toHaveTextContent('Charon could not load the database file.'))
      expect(statusRegion(region)).toHaveClass('text-error')
    })
  })

  describe('replace', () => {
    it('states the 32 MiB size limit as a hint', async () => {
      const region = await openCard()
      expect(await within(region).findByText('Upload a GeoIP Country database (.mmdb), up to 32 MiB.')).toBeInTheDocument()
    })

    it('opens the file chooser from the Replace button', async () => {
      const region = await openCard()
      const replaceButton = await within(region).findByRole('button', { name: 'Replace database…' })
      const input = within(region).getByTestId('geoip-file-input')
      const clickSpy = vi.spyOn(input as HTMLInputElement, 'click')

      fireEvent.click(replaceButton)

      expect(clickSpy).toHaveBeenCalledTimes(1)
      expect(input).toHaveAttribute('accept', '.mmdb')
    })

    it('uploads the chosen file and reports success inline', async () => {
      vi.mocked(securityApi.replaceGeoIPDatabase).mockResolvedValue({ status: status({ source: 'data' }), replaced_previous: false })
      const region = await openCard()
      await within(region).findByText('Ready')
      const input = within(region).getByTestId('geoip-file-input') as HTMLInputElement
      const file = new File(['mmdb'], 'GeoLite2-Country.mmdb')

      fireEvent.change(input, { target: { files: [file] } })

      await waitFor(() => expect(securityApi.replaceGeoIPDatabase).toHaveBeenCalledWith(file))
      await waitFor(() =>
        expect(statusRegion(region)).toHaveTextContent('Database replaced. Charon is now using your copy.')
      )
    })

    it('ignores a change event without a file', async () => {
      const region = await openCard()
      await within(region).findByText('Ready')
      const input = within(region).getByTestId('geoip-file-input') as HTMLInputElement

      fireEvent.change(input, { target: { files: [] } })

      expect(securityApi.replaceGeoIPDatabase).not.toHaveBeenCalled()
    })

    it('maps a text-file refusal to its inline error', async () => {
      vi.mocked(securityApi.replaceGeoIPDatabase).mockRejectedValue(refusal('geoip_not_mmdb'))
      const region = await openCard()
      await within(region).findByText('Ready')
      const input = within(region).getByTestId('geoip-file-input') as HTMLInputElement

      fireEvent.change(input, { target: { files: [new File(['text'], 'notes.txt', { type: 'text/plain' })] } })

      await waitFor(() => expect(statusRegion(region)).toHaveTextContent('That file is not a GeoIP database.'))
      expect(statusRegion(region)).toHaveClass('text-error')
    })

    it.each([
      ['geoip_too_large', 'That file is larger than 32 MiB.'],
      ['geoip_wrong_type', 'That is a GeoIP database, but not a country database.'],
      ['geoip_older_than_current', 'That database is older than the one in use. Upload a newer one.'],
      ['geoip_build_date_in_future', 'That database has a build date in the future.'],
      ['geoip_busy', 'Another database change is in progress. Try again in a moment.'],
      ['proxy_apply_failed', 'Charon could not apply the proxy configuration.'],
    ])('maps %s to its text', async (code, text) => {
      vi.mocked(securityApi.replaceGeoIPDatabase).mockRejectedValue(refusal(code))
      const region = await openCard()
      await within(region).findByText('Ready')
      const input = within(region).getByTestId('geoip-file-input') as HTMLInputElement

      fireEvent.change(input, { target: { files: [new File(['x'], 'a.mmdb')] } })

      await waitFor(() => expect(statusRegion(region)).toHaveTextContent(text))
    })

    it('falls back to the generic text for an unknown error code', async () => {
      vi.mocked(securityApi.replaceGeoIPDatabase).mockRejectedValue(refusal('something_new'))
      const region = await openCard()
      await within(region).findByText('Ready')
      const input = within(region).getByTestId('geoip-file-input') as HTMLInputElement

      fireEvent.change(input, { target: { files: [new File(['x'], 'a.mmdb')] } })

      await waitFor(() => expect(statusRegion(region)).toHaveTextContent('Something went wrong. Try again.'))
    })

    it('falls back to the generic text for a refusal with no body', async () => {
      vi.mocked(securityApi.replaceGeoIPDatabase).mockRejectedValue(new Error('network down'))
      const region = await openCard()
      await within(region).findByText('Ready')
      const input = within(region).getByTestId('geoip-file-input') as HTMLInputElement

      fireEvent.change(input, { target: { files: [new File(['x'], 'a.mmdb')] } })

      await waitFor(() => expect(statusRegion(region)).toHaveTextContent('Something went wrong. Try again.'))
    })
  })

  describe('use the included copy', () => {
    const dataSource = () => status({ source: 'data', data_copy_present: true })

    it('asks for confirmation before removing anything', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(dataSource())
      const region = await openCard()

      fireEvent.click(await within(region).findByRole('button', { name: 'Use the included copy' }))

      const dialog = await screen.findByRole('dialog')
      expect(dialog).toHaveTextContent('Your copy in the data folder will be removed')
      expect(securityApi.removeGeoIPDatabase).not.toHaveBeenCalled()
    })

    it('closes the dialog without removing when cancelled', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(dataSource())
      const region = await openCard()
      fireEvent.click(await within(region).findByRole('button', { name: 'Use the included copy' }))
      const dialog = await screen.findByRole('dialog')

      fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
      expect(securityApi.removeGeoIPDatabase).not.toHaveBeenCalled()
    })

    it('removes the data-folder copy on confirm and reports it inline', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(dataSource())
      vi.mocked(securityApi.removeGeoIPDatabase).mockResolvedValue({ status: bundled })
      const region = await openCard()
      fireEvent.click(await within(region).findByRole('button', { name: 'Use the included copy' }))
      const dialog = await screen.findByRole('dialog')

      fireEvent.click(within(dialog).getByRole('button', { name: 'Use the included copy' }))

      await waitFor(() => expect(securityApi.removeGeoIPDatabase).toHaveBeenCalledTimes(1))
      await waitFor(() => expect(statusRegion(region)).toHaveTextContent('Charon is using the included copy again.'))
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    })

    it('keeps the dialog open with the mapped error when removal is refused', async () => {
      vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue(dataSource())
      vi.mocked(securityApi.removeGeoIPDatabase).mockRejectedValue(refusal('geoip_no_data_copy', 404))
      const region = await openCard()
      fireEvent.click(await within(region).findByRole('button', { name: 'Use the included copy' }))
      const dialog = await screen.findByRole('dialog')

      fireEvent.click(within(dialog).getByRole('button', { name: 'Use the included copy' }))

      await waitFor(() => expect(statusRegion(region)).toHaveTextContent('There is no copy in the data folder to remove.'))
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    })
  })
})
