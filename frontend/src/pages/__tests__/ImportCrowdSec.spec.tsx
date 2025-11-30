import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import userEvent from '@testing-library/user-event'
import ImportCrowdSec from '../ImportCrowdSec'
import * as api from '../../api/crowdsec'
import * as backups from '../../api/backups'

vi.mock('../../api/crowdsec')
vi.mock('../../api/backups')

const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })

const renderWithProviders = (ui: React.ReactNode) => {
  const qc = createQueryClient()
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        {ui}
      </BrowserRouter>
    </QueryClientProvider>
  )
}

describe('ImportCrowdSec page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('creates a backup then imports crowdsec', async () => {
    const file = new File(['fake'], 'crowdsec.zip', { type: 'application/zip' })
    vi.mocked(backups.createBackup).mockResolvedValue({ filename: 'b1' })
    vi.mocked(api.importCrowdsecConfig).mockResolvedValue({ success: true })

    renderWithProviders(<ImportCrowdSec />)
    const fileInput = document.querySelector('input[type="file"]')
    expect(fileInput).toBeTruthy()
    fireEvent.change(fileInput!, { target: { files: [file] } })
    const importBtn = screen.getByText('Import')
    await userEvent.click(importBtn)

    await waitFor(() => expect(backups.createBackup).toHaveBeenCalled())
    await waitFor(() => expect(api.importCrowdsecConfig).toHaveBeenCalledWith(file))
  })
})
