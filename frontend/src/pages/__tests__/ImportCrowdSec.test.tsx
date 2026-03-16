import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'react-hot-toast'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as backupsApi from '../../api/backups'
import * as crowdsecApi from '../../api/crowdsec'
import { createTestQueryClient } from '../../test/createTestQueryClient'
import ImportCrowdSec from '../ImportCrowdSec'

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
    const qc = createTestQueryClient()
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
    const user = userEvent.setup()

    await user.upload(fileInput, file)

    const importButton = screen.getByRole('button', { name: /Import/i })
    await user.click(importButton)

    await waitFor(() => {
      expect(backupsApi.createBackup).toHaveBeenCalled()
      expect(crowdsecApi.importCrowdsecConfig).toHaveBeenCalledWith(file)
      expect(toast.success).toHaveBeenCalledWith('CrowdSec config imported')
    })
  })
})
