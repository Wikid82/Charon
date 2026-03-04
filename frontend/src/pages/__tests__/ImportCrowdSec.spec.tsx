import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import userEvent from '@testing-library/user-event'
import ImportCrowdSec from '../ImportCrowdSec'
import * as api from '../../api/crowdsec'
import * as backups from '../../api/backups'
import { createTestQueryClient } from '../../test/createTestQueryClient'

vi.mock('../../api/crowdsec')
vi.mock('../../api/backups')

const renderWithProviders = (ui: React.ReactNode) => {
  const qc = createTestQueryClient()
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
    const user = userEvent.setup()
    await user.click(importBtn)

    await waitFor(() => expect(backups.createBackup).toHaveBeenCalled())
    await waitFor(() => expect(api.importCrowdsecConfig).toHaveBeenCalledWith(file))
  })
})
