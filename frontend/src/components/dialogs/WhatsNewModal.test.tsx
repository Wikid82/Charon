import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import WhatsNewModal from './WhatsNewModal'
import { useAckChangelog, useChangelogAll, useChangelogStatus } from '../../hooks/useChangelog'

import type { ChangelogEntry, ChangelogStatus, ChangelogAll } from '../../api/changelog'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))

vi.mock('../../hooks/useChangelog', () => ({
  useChangelogStatus: vi.fn(),
  useChangelogAll: vi.fn(),
  useAckChangelog: vi.fn(),
}))

const mockUseChangelogStatus = vi.mocked(useChangelogStatus)
const mockUseChangelogAll = vi.mocked(useChangelogAll)
const mockUseAckChangelog = vi.mocked(useAckChangelog)

const entryWithEverything: ChangelogEntry = {
  version: '1.3.0',
  date: '2026-07-20',
  features: ['Added the What\'s New modal'],
  fixes: ['Fixed a login redirect bug'],
  other: ['Bumped internal dependency versions'],
}

const entryFeaturesOnly: ChangelogEntry = {
  version: '1.2.0',
  date: '2026-06-01',
  features: ['Added dark mode'],
  fixes: [],
  other: [],
}

type AckMutationMock = ReturnType<typeof useAckChangelog>

function makeAckMutation(overrides: Record<string, unknown> = {}): AckMutationMock {
  return {
    mutate: vi.fn(),
    mutateAsync: vi.fn(),
    isPending: false,
    isError: false,
    isSuccess: false,
    isIdle: true,
    status: 'idle',
    data: undefined,
    error: null,
    variables: undefined,
    reset: vi.fn(),
    context: undefined,
    failureCount: 0,
    failureReason: null,
    isPaused: false,
    submittedAt: 0,
    ...overrides,
  } as unknown as AckMutationMock
}

function setStatusData(data: ChangelogStatus | undefined, isError = false) {
  mockUseChangelogStatus.mockReturnValue({
    data,
    isError,
    isPending: false,
    isSuccess: !isError && !!data,
  } as unknown as ReturnType<typeof useChangelogStatus>)
}

function setAllData(data: ChangelogAll | undefined) {
  mockUseChangelogAll.mockReturnValue({
    data,
    isError: false,
    isPending: false,
    isSuccess: !!data,
  } as unknown as ReturnType<typeof useChangelogAll>)
}

describe('WhatsNewModal', () => {
  let ackMutate: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.clearAllMocks()
    ackMutate = vi.fn()
    mockUseAckChangelog.mockReturnValue(makeAckMutation({ mutate: ackMutate }))
    setAllData(undefined)
  })

  describe('status mode', () => {
    it('renders nothing when show_changelog is false', () => {
      setStatusData({ show_changelog: false, versions: [] })
      const { container } = render(<WhatsNewModal mode="status" />)
      expect(container.innerHTML).toBe('')
    })

    it('renders nothing when the status fetch errors', () => {
      setStatusData(undefined, true)
      const { container } = render(<WhatsNewModal mode="status" />)
      expect(container.innerHTML).toBe('')
    })

    it('renders entries when show_changelog is true', () => {
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      expect(screen.getByText('1.3.0')).toBeInTheDocument()
      expect(screen.getByText('2026-07-20')).toBeInTheDocument()
      expect(screen.getByText('Added the What\'s New modal')).toBeInTheDocument()
      expect(screen.getByText('Fixed a login redirect bug')).toBeInTheDocument()
    })

    it('omits the features group when the entry has no features', () => {
      const entry: ChangelogEntry = { ...entryFeaturesOnly, features: [], fixes: ['A fix'] }
      setStatusData({ show_changelog: true, versions: [entry] })
      render(<WhatsNewModal mode="status" />)

      expect(screen.queryByText('whatsNew.newFeatures')).not.toBeInTheDocument()
      expect(screen.getByText('whatsNew.fixes')).toBeInTheDocument()
    })

    it('omits the fixes group when the entry has no fixes', () => {
      setStatusData({ show_changelog: true, versions: [entryFeaturesOnly] })
      render(<WhatsNewModal mode="status" />)

      expect(screen.getByText('whatsNew.newFeatures')).toBeInTheDocument()
      expect(screen.queryByText('whatsNew.fixes')).not.toBeInTheDocument()
    })

    it('renders the Other section collapsed by default behind a details disclosure', () => {
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      const details = screen.getByText('whatsNew.showMaintenanceDetails').closest('details')
      expect(details).not.toBeNull()
      expect(details).not.toHaveAttribute('open')
      expect(screen.getByText('Bumped internal dependency versions')).toBeInTheDocument()
    })

    it('omits the Other disclosure entirely when there is nothing in it', () => {
      setStatusData({ show_changelog: true, versions: [entryFeaturesOnly] })
      render(<WhatsNewModal mode="status" />)

      expect(screen.queryByText('whatsNew.showMaintenanceDetails')).not.toBeInTheDocument()
    })

    it('"Remind Me Next Time" sends dismiss_temporary with opt_out=false by default', async () => {
      const user = userEvent.setup()
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      await user.click(screen.getByRole('button', { name: 'whatsNew.remindLater' }))

      expect(ackMutate).toHaveBeenCalledWith({ action: 'dismiss_temporary', opt_out: false })
    })

    it('"Got It, Thanks" sends dismiss_permanent with opt_out=false by default', async () => {
      const user = userEvent.setup()
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      await user.click(screen.getByRole('button', { name: 'whatsNew.gotIt' }))

      expect(ackMutate).toHaveBeenCalledWith({ action: 'dismiss_permanent', opt_out: false })
    })

    it('honors a checked opt-out checkbox on "Remind Me Next Time"', async () => {
      const user = userEvent.setup()
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      await user.click(screen.getByRole('checkbox', { name: 'whatsNew.dontShowAgain' }))
      await user.click(screen.getByRole('button', { name: 'whatsNew.remindLater' }))

      expect(ackMutate).toHaveBeenCalledWith({ action: 'dismiss_temporary', opt_out: true })
    })

    it('honors a checked opt-out checkbox on "Got It, Thanks"', async () => {
      const user = userEvent.setup()
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      await user.click(screen.getByRole('checkbox', { name: 'whatsNew.dontShowAgain' }))
      await user.click(screen.getByRole('button', { name: 'whatsNew.gotIt' }))

      expect(ackMutate).toHaveBeenCalledWith({ action: 'dismiss_permanent', opt_out: true })
    })

    it('the X close icon triggers the same effect as "Remind Me Next Time", honoring the checkbox', async () => {
      const user = userEvent.setup()
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      await user.click(screen.getByRole('checkbox', { name: 'whatsNew.dontShowAgain' }))
      await user.click(screen.getByRole('button', { name: 'Close' }))

      expect(ackMutate).toHaveBeenCalledWith({ action: 'dismiss_temporary', opt_out: true })
    })
  })

  describe('browse mode', () => {
    it('renders nothing when open is false', () => {
      const { container } = render(<WhatsNewModal mode="browse" open={false} onClose={vi.fn()} />)
      expect(container.innerHTML).toBe('')
    })

    it('renders full history entries when open', () => {
      setAllData({ versions: [entryWithEverything, entryFeaturesOnly] })
      render(<WhatsNewModal mode="browse" open onClose={vi.fn()} />)

      expect(screen.getByText('1.3.0')).toBeInTheDocument()
      expect(screen.getByText('1.2.0')).toBeInTheDocument()
    })

    it('calls onClose (and never ack) when Close is clicked', async () => {
      const user = userEvent.setup()
      const onClose = vi.fn()
      setAllData({ versions: [entryWithEverything] })
      render(<WhatsNewModal mode="browse" open onClose={onClose} />)

      await user.click(screen.getByRole('button', { name: 'whatsNew.closeButton' }))

      expect(onClose).toHaveBeenCalledTimes(1)
      expect(ackMutate).not.toHaveBeenCalled()
    })

    it('calls onClose (and never ack) when the X icon is clicked', async () => {
      const user = userEvent.setup()
      const onClose = vi.fn()
      setAllData({ versions: [entryWithEverything] })
      render(<WhatsNewModal mode="browse" open onClose={onClose} />)

      await user.click(screen.getByRole('button', { name: 'Close' }))

      expect(onClose).toHaveBeenCalledTimes(1)
      expect(ackMutate).not.toHaveBeenCalled()
    })
  })
})
