import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ImportCrowdSec from '../ImportCrowdSec'
import * as crowdsecApi from '../../api/crowdsec'
import * as backupsApi from '../../api/backups'
import { toast } from 'react-hot-toast'

vi.mock('../../api/crowdsec')
vi.mock('../../api/backups')
vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    loading: vi.fn(),
    dismiss: vi.fn(),
  },
}))

describe('ImportCrowdSec', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.tar.gz' })
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockResolvedValue({})
  })

  const renderPage = () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    return render(
      <QueryClientProvider client={qc}>
        <MemoryRouter>
          <ImportCrowdSec />
        </MemoryRouter>
      </QueryClientProvider>
    )
  }

  it('renders configuration packages heading', async () => {
    renderPage()

    await waitFor(() => screen.getByText('CrowdSec Configuration Packages'))
    expect(screen.getByText('CrowdSec Configuration Packages')).toBeInTheDocument()
  })

  it('creates a backup before importing selected package', async () => {
    renderPage()

    const fileInput = screen.getByTestId('crowdsec-import-file') as HTMLInputElement
    const file = new File(['config'], 'config.tar.gz', { type: 'application/gzip' })

    await userEvent.upload(fileInput, file)

    const importButton = screen.getByRole('button', { name: /Import/i })
    await userEvent.click(importButton)

    await waitFor(() => {
      expect(backupsApi.createBackup).toHaveBeenCalled()
      expect(crowdsecApi.importCrowdsecConfig).toHaveBeenCalledWith(file)
      expect(toast.success).toHaveBeenCalledWith('CrowdSec config imported')
    })
  })
})
