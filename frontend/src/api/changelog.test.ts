import { describe, it, expect, vi, beforeEach } from 'vitest'

import {
  getChangelogStatus,
  getChangelogAll,
  ackChangelog,
  optInChangelog,
  type ChangelogEntry,
} from './changelog'
import client from './client'

vi.mock('./client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

const mockedClient = client as unknown as {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
}

const sampleEntry: ChangelogEntry = {
  version: '1.2.0',
  date: '2026-07-01',
  features: ['Added dark mode'],
  fixes: ['Fixed login redirect'],
  other: ['Bumped dependencies'],
  security: [{ summary: 'Hardened input validation in the API layer', sha: 'deadbeef' }],
}

describe('changelog api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches changelog status and returns response data', async () => {
    mockedClient.get.mockResolvedValue({
      data: { show_changelog: true, versions: [sampleEntry] },
    })

    const result = await getChangelogStatus()

    expect(mockedClient.get).toHaveBeenCalledWith('/changelog/status')
    expect(result).toEqual({ show_changelog: true, versions: [sampleEntry] })
  })

  it('fetches full changelog history and returns response data', async () => {
    mockedClient.get.mockResolvedValue({
      data: { versions: [sampleEntry] },
    })

    const result = await getChangelogAll()

    expect(mockedClient.get).toHaveBeenCalledWith('/changelog/all')
    expect(result).toEqual({ versions: [sampleEntry] })
  })

  it('posts a dismiss_temporary ack with the opt-out flag', async () => {
    mockedClient.post.mockResolvedValue({ data: { message: 'Acknowledged' } })

    await ackChangelog({ action: 'dismiss_temporary', opt_out: true })

    expect(mockedClient.post).toHaveBeenCalledWith('/changelog/ack', {
      action: 'dismiss_temporary',
      opt_out: true,
    })
  })

  it('posts a dismiss_permanent ack with the opt-out flag', async () => {
    mockedClient.post.mockResolvedValue({ data: { message: 'Acknowledged' } })

    await ackChangelog({ action: 'dismiss_permanent', opt_out: false })

    expect(mockedClient.post).toHaveBeenCalledWith('/changelog/ack', {
      action: 'dismiss_permanent',
      opt_out: false,
    })
  })

  it('posts to the opt-in endpoint with no body', async () => {
    mockedClient.post.mockResolvedValue({ data: { message: 'Opted in' } })

    await optInChangelog()

    expect(mockedClient.post).toHaveBeenCalledWith('/changelog/opt-in')
  })

  it('propagates errors from the underlying client', async () => {
    mockedClient.get.mockRejectedValue(new Error('Network error'))

    await expect(getChangelogStatus()).rejects.toThrow('Network error')
  })
})
