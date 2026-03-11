import { QueryClientProvider } from '@tanstack/react-query'
import { renderHook, act, waitFor } from '@testing-library/react'
import React from 'react'
import { vi, describe, it, expect, beforeEach } from 'vitest'

import * as api from '../../api/import'
import { createTestQueryClient } from '../../test/createTestQueryClient'
import { useImport } from '../useImport'

import type { ImportSession, ImportPreview } from '../../api/import'

vi.mock('../../api/import', () => ({
  uploadCaddyfile: vi.fn(),
  getImportPreview: vi.fn(),
  commitImport: vi.fn(),
  getImportStatus: vi.fn(),
  cancelImport: vi.fn(),
}))

const wrapper = ({ children }: { children: React.ReactNode }) => {
  const qc = createTestQueryClient()
  return React.createElement(QueryClientProvider, { client: qc }, children)
}

describe('useImport (unit)', () => {
  beforeEach(() => vi.clearAllMocks())

  it('commit throws when there is no active session (guards)', async () => {
    vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: false })

    const { result } = renderHook(() => useImport(), { wrapper })

    await waitFor(() => expect(result.current.loading).toBe(false))

    await expect(result.current.commit({}, {})).rejects.toThrow(/No active session/)
  })

  it('does not surface preview error when there is no pending session', async () => {
    vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: false })
    vi.mocked(api.getImportPreview).mockRejectedValue(new Error('preview fail'))

    const { result } = renderHook(() => useImport(), { wrapper })

    await waitFor(() => expect(result.current.loading).toBe(false))
    // preview endpoint failed but there is no pending session => error should be null
    expect(result.current.error).toBeNull()
  })

  it('surfaces preview error when session is pending and commit not succeeded', async () => {
    vi.mocked(api.getImportStatus).mockResolvedValue({
      has_pending: true,
      session: { id: 's1', state: 'reviewing', created_at: '2026-01-31T00:00:00.000Z', updated_at: '2026-01-31T00:00:00.000Z' },
    })
    vi.mocked(api.getImportPreview).mockRejectedValue(new Error('preview fail'))

    const { result } = renderHook(() => useImport(), { wrapper })

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toContain('preview fail')
  })

  it('enables preview query when session state is pending', async () => {
    const session: ImportSession = {
      id: 's-pending',
      state: 'pending',
      created_at: '2026-01-31T00:00:00.000Z',
      updated_at: '2026-01-31T00:00:00.000Z',
    }
    vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: true, session })
    const mockPreviewResponse: ImportPreview = {
      session,
      preview: { hosts: [], conflicts: [], errors: [] }
    }
    vi.mocked(api.getImportPreview).mockResolvedValue(mockPreviewResponse)

    const { result } = renderHook(() => useImport(), { wrapper })

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(api.getImportPreview).toHaveBeenCalled()
  })

  it('upload stores immediate uploadPreview and exposes preview', async () => {
    const mockPreview: ImportPreview = {
      session: { id: 's1', state: 'reviewing', created_at: '2026-01-31T00:00:00.000Z', updated_at: '2026-01-31T00:00:00.000Z' },
      preview: { hosts: [], conflicts: [], errors: [] },
    }
    vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: false })
    vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockPreview)

    const { result } = renderHook(() => useImport(), { wrapper })

    await act(async () => {
      await result.current.upload('some content')
    })

    expect(api.uploadCaddyfile).toHaveBeenCalled()
    expect(result.current.preview).toEqual(mockPreview)
  })
})
