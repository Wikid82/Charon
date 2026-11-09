import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { useAuth } from '../../hooks/useAuth'
import { useAckChangelog, useChangelogAll, useChangelogStatus } from '../../hooks/useChangelog'
import WhatsNewModal from './WhatsNewModal'

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

vi.mock('../../hooks/useAuth', () => ({
  useAuth: vi.fn(),
}))

const mockUseChangelogStatus = vi.mocked(useChangelogStatus)
const mockUseChangelogAll = vi.mocked(useChangelogAll)
const mockUseAckChangelog = vi.mocked(useAckChangelog)
const mockUseAuth = vi.mocked(useAuth)

const entryWithEverything: ChangelogEntry = {
  version: '1.3.0',
  date: '2026-07-20',
  features: ['Added the What\'s New modal'],
  fixes: ['Fixed a login redirect bug'],
  other: ['Bumped internal dependency versions'],
  security: [{ summary: 'Hardened input validation in the API layer', sha: 'a1b2c3d4e5f6' }],
}

const entryFeaturesOnly: ChangelogEntry = {
  version: '1.2.0',
  date: '2026-06-01',
  features: ['Added dark mode'],
  fixes: [],
  other: [],
  security: [],
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
  let refetchUser: () => Promise<void>

  beforeEach(() => {
    vi.clearAllMocks()
    ackMutate = vi.fn()
    refetchUser = vi.fn(() => Promise.resolve())
    mockUseAckChangelog.mockReturnValue(makeAckMutation({ mutate: ackMutate }))
    mockUseAuth.mockReturnValue({
      user: null,
      login: vi.fn(),
      logout: vi.fn(),
      changePassword: vi.fn(),
      refetchUser,
      isAuthenticated: false,
      isLoading: false,
    })
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

      expect(screen.queryByRole('heading', { name: 'whatsNew.newFeatures' })).not.toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'whatsNew.fixes' })).toBeInTheDocument()
    })

    it('omits the fixes group when the entry has no fixes', () => {
      setStatusData({ show_changelog: true, versions: [entryFeaturesOnly] })
      render(<WhatsNewModal mode="status" />)

      expect(screen.getByRole('heading', { name: 'whatsNew.newFeatures' })).toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: 'whatsNew.fixes' })).not.toBeInTheDocument()
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

    it('renders the Security group expanded by default, NOT behind a details disclosure', () => {
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      expect(screen.getByRole('heading', { name: 'whatsNew.security' })).toBeInTheDocument()
      expect(
        screen.getByText('Hardened input validation in the API layer', { exact: false })
      ).toBeInTheDocument()
      // Unlike the "Other" group (asserted above as collapsed behind
      // <details>), the Security group must not be wrapped in any
      // <details>/<summary> disclosure at all — it renders like the
      // features/fixes groups, always visible when non-empty. A native
      // <details> element has an implicit "group" role, so with this entry's
      // one "Other" item present, exactly one such role should exist on the
      // page — none contributed by Security.
      expect(screen.getAllByRole('group')).toHaveLength(1)
    })

    it('renders a "view commit" link for each security item with security-conscious attributes', () => {
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      const link = screen.getByRole('link', { name: 'whatsNew.viewCommit' })
      expect(link).toHaveAttribute(
        'href',
        'https://github.com/Wikid82/Charon/commit/a1b2c3d4e5f6'
      )
      expect(link).toHaveAttribute('target', '_blank')
      expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    })

    it('omits the Security group entirely when the entry has no security entries', () => {
      setStatusData({ show_changelog: true, versions: [entryFeaturesOnly] })
      render(<WhatsNewModal mode="status" />)

      expect(screen.queryByRole('heading', { name: 'whatsNew.security' })).not.toBeInTheDocument()
      expect(screen.queryByRole('link', { name: 'whatsNew.viewCommit' })).not.toBeInTheDocument()
    })

    describe('regression: null group fields must not crash the modal', () => {
      // The backend can serialize a nil Go slice as JSON `null` (rather than
      // omitting the key or sending `[]`), and hand-edited local dev fixture
      // data can omit a group key outright. Confirms the modal treats a
      // `null` group the same as an empty array instead of throwing on
      // `.length`.

      it('renders without throwing and omits the group when `security` is null', () => {
        const entry: ChangelogEntry = { ...entryWithEverything, security: null }
        setStatusData({ show_changelog: true, versions: [entry] })

        expect(() => render(<WhatsNewModal mode="status" />)).not.toThrow()
        expect(screen.queryByRole('heading', { name: 'whatsNew.security' })).not.toBeInTheDocument()
        // Sibling groups still render normally.
        expect(screen.getByRole('heading', { name: 'whatsNew.newFeatures' })).toBeInTheDocument()
      })

      it('renders without throwing and omits the group when `features` is null', () => {
        const entry: ChangelogEntry = { ...entryWithEverything, features: null }
        setStatusData({ show_changelog: true, versions: [entry] })

        expect(() => render(<WhatsNewModal mode="status" />)).not.toThrow()
        expect(screen.queryByRole('heading', { name: 'whatsNew.newFeatures' })).not.toBeInTheDocument()
        expect(screen.getByRole('heading', { name: 'whatsNew.fixes' })).toBeInTheDocument()
      })

      it('renders without throwing and omits the group when `fixes` is null', () => {
        const entry: ChangelogEntry = { ...entryWithEverything, fixes: null }
        setStatusData({ show_changelog: true, versions: [entry] })

        expect(() => render(<WhatsNewModal mode="status" />)).not.toThrow()
        expect(screen.queryByRole('heading', { name: 'whatsNew.fixes' })).not.toBeInTheDocument()
        expect(screen.getByRole('heading', { name: 'whatsNew.newFeatures' })).toBeInTheDocument()
      })

      it('renders without throwing and omits the disclosure when `other` is null', () => {
        const entry: ChangelogEntry = { ...entryWithEverything, other: null }
        setStatusData({ show_changelog: true, versions: [entry] })

        expect(() => render(<WhatsNewModal mode="status" />)).not.toThrow()
        expect(screen.queryByText('whatsNew.showMaintenanceDetails')).not.toBeInTheDocument()
        expect(screen.getByRole('heading', { name: 'whatsNew.newFeatures' })).toBeInTheDocument()
      })

      it('renders without throwing when all four group fields are null', () => {
        const entry: ChangelogEntry = {
          version: '1.4.0',
          date: '2026-07-25',
          features: null,
          fixes: null,
          other: null,
          security: null,
        }
        setStatusData({ show_changelog: true, versions: [entry] })

        expect(() => render(<WhatsNewModal mode="status" />)).not.toThrow()
        expect(screen.getByText('1.4.0')).toBeInTheDocument()
        expect(screen.queryByRole('heading', { name: 'whatsNew.newFeatures' })).not.toBeInTheDocument()
        expect(screen.queryByRole('heading', { name: 'whatsNew.fixes' })).not.toBeInTheDocument()
        expect(screen.queryByRole('heading', { name: 'whatsNew.security' })).not.toBeInTheDocument()
        expect(screen.queryByText('whatsNew.showMaintenanceDetails')).not.toBeInTheDocument()
      })
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

      // Opting out also passes an onSuccess that refetches the AuthContext
      // user (see "AuthContext refetch on opt-out" below) — asserted here
      // via objectContaining since this test's focus is the ack payload.
      expect(ackMutate).toHaveBeenCalledWith(
        { action: 'dismiss_temporary', opt_out: true },
        expect.objectContaining({ onSuccess: expect.any(Function) })
      )
    })

    it('honors a checked opt-out checkbox on "Got It, Thanks"', async () => {
      const user = userEvent.setup()
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      await user.click(screen.getByRole('checkbox', { name: 'whatsNew.dontShowAgain' }))
      await user.click(screen.getByRole('button', { name: 'whatsNew.gotIt' }))

      expect(ackMutate).toHaveBeenCalledWith(
        { action: 'dismiss_permanent', opt_out: true },
        expect.objectContaining({ onSuccess: expect.any(Function) })
      )
    })

    it('the X close icon triggers the same effect as "Remind Me Next Time", honoring the checkbox', async () => {
      const user = userEvent.setup()
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      await user.click(screen.getByRole('checkbox', { name: 'whatsNew.dontShowAgain' }))
      await user.click(screen.getByRole('button', { name: 'Close' }))

      expect(ackMutate).toHaveBeenCalledWith(
        { action: 'dismiss_temporary', opt_out: true },
        expect.objectContaining({ onSuccess: expect.any(Function) })
      )
    })

    describe('AuthContext refetch on opt-out (regression: Appearance Settings toggle went stale after opting out via the modal checkbox)', () => {
      // The modal's own "Don't show me update notifications" checkbox sends
      // opt_out: true to `ack`, which updates `user.changelog_opt_out`
      // server-side. AuthContext's `user` is separate query state that the
      // `ack` mutation's own cache invalidation (`changelog-status`) does
      // not touch, so without an explicit `refetchUser()` call, navigating
      // to Appearance Settings in the same session showed the stale
      // (pre-opt-out) toggle value until a full page reload.

      it('refetches the user after opting out via "Remind Me Next Time"', async () => {
        const user = userEvent.setup()
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        render(<WhatsNewModal mode="status" />)

        await user.click(screen.getByRole('checkbox', { name: 'whatsNew.dontShowAgain' }))
        await user.click(screen.getByRole('button', { name: 'whatsNew.remindLater' }))

        expect(ackMutate).toHaveBeenCalledWith(
          { action: 'dismiss_temporary', opt_out: true },
          expect.objectContaining({ onSuccess: expect.any(Function) })
        )

        // Simulate the mutation succeeding and invoking the onSuccess
        // callback the component supplied.
        const [, options] = ackMutate.mock.calls[0] as [unknown, { onSuccess?: () => void }]
        options.onSuccess?.()

        expect(refetchUser).toHaveBeenCalledTimes(1)
      })

      it('refetches the user after opting out via "Got It, Thanks"', async () => {
        const user = userEvent.setup()
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        render(<WhatsNewModal mode="status" />)

        await user.click(screen.getByRole('checkbox', { name: 'whatsNew.dontShowAgain' }))
        await user.click(screen.getByRole('button', { name: 'whatsNew.gotIt' }))

        const [, options] = ackMutate.mock.calls[0] as [unknown, { onSuccess?: () => void }]
        options.onSuccess?.()

        expect(refetchUser).toHaveBeenCalledTimes(1)
      })

      it('refetches the user after opting out via the X icon/backdrop dismissal', async () => {
        const user = userEvent.setup()
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        render(<WhatsNewModal mode="status" />)

        await user.click(screen.getByRole('checkbox', { name: 'whatsNew.dontShowAgain' }))
        await user.click(screen.getByRole('button', { name: 'Close' }))

        const [, options] = ackMutate.mock.calls[0] as [unknown, { onSuccess?: () => void }]
        options.onSuccess?.()

        expect(refetchUser).toHaveBeenCalledTimes(1)
      })

      it('does NOT refetch the user when the opt-out checkbox is left unchecked', async () => {
        const user = userEvent.setup()
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        render(<WhatsNewModal mode="status" />)

        await user.click(screen.getByRole('button', { name: 'whatsNew.remindLater' }))

        expect(ackMutate).toHaveBeenCalledWith({ action: 'dismiss_temporary', opt_out: false })
        expect(refetchUser).not.toHaveBeenCalled()
      })
    })

    describe('per-session dismissal (regression: modal must not pop back open within the same session)', () => {
      // `dismiss_temporary` intentionally leaves `last_seen_version`
      // unchanged server-side, so `useAckChangelog`'s onSuccess
      // invalidate-and-refetch of `changelog-status` legitimately resolves
      // `show_changelog: true` again. Without client-side session
      // dismissal state, the modal would immediately reopen instead of
      // staying closed until a real future login.

      it('"Remind Me Next Time" closes the modal and it stays closed across a refetch that still returns show_changelog: true', async () => {
        const user = userEvent.setup()
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        const { rerender } = render(<WhatsNewModal mode="status" />)

        expect(screen.getByText('1.3.0')).toBeInTheDocument()

        await user.click(screen.getByRole('button', { name: 'whatsNew.remindLater' }))

        // The dialog closes immediately, without waiting on the mutation.
        expect(screen.queryByText('1.3.0')).not.toBeInTheDocument()

        // Simulate the query-invalidation refetch triggered by the ack
        // mutation's onSuccess: same backend answer (show_changelog still
        // true, since last_seen_version never changed for a temporary
        // dismissal), fresh object reference (a real refetch).
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        rerender(<WhatsNewModal mode="status" />)

        expect(screen.queryByText('1.3.0')).not.toBeInTheDocument()
      })

      it('the X icon dismissal stays closed across a subsequent show_changelog: true refetch', async () => {
        const user = userEvent.setup()
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        const { rerender } = render(<WhatsNewModal mode="status" />)

        await user.click(screen.getByRole('button', { name: 'Close' }))
        expect(screen.queryByText('1.3.0')).not.toBeInTheDocument()

        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        rerender(<WhatsNewModal mode="status" />)

        expect(screen.queryByText('1.3.0')).not.toBeInTheDocument()
      })

      it('backdrop dismissal (Dialog onOpenChange close) stays closed across a subsequent show_changelog: true refetch', async () => {
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        const { rerender } = render(<WhatsNewModal mode="status" />)

        // Backdrop click / Escape both route through Radix's
        // onOpenChange(false), same as the X icon.
        await userEvent.setup().keyboard('{Escape}')
        expect(screen.queryByText('1.3.0')).not.toBeInTheDocument()

        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        rerender(<WhatsNewModal mode="status" />)

        expect(screen.queryByText('1.3.0')).not.toBeInTheDocument()
      })

      it('"Got It, Thanks" (dismiss_permanent) stays closed even if a later refetch still returns show_changelog: true', async () => {
        // Covers the Settings re-opt-in scenario: user permanently
        // dismisses (optionally with opt-out checked), then later
        // re-opts-in via Settings. useOptInChangelog's onSuccess
        // invalidates the same `changelog-status` query, which can
        // legitimately resolve show_changelog: true again if unseen
        // entries remain. The modal must not pop up over the Settings
        // page mid-interaction — it should wait for the next login.
        const user = userEvent.setup()
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        const { rerender } = render(<WhatsNewModal mode="status" />)

        await user.click(screen.getByRole('checkbox', { name: 'whatsNew.dontShowAgain' }))
        await user.click(screen.getByRole('button', { name: 'whatsNew.gotIt' }))
        expect(screen.queryByText('1.3.0')).not.toBeInTheDocument()

        // Simulate: user re-opts-in via Appearance Settings, which flips
        // changelog_opt_out server-side and invalidates changelog-status;
        // the refetch still finds unseen entries.
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        rerender(<WhatsNewModal mode="status" />)

        expect(screen.queryByText('1.3.0')).not.toBeInTheDocument()
      })

      it('a fresh mount (real login / page load) is unaffected by a prior instance\'s dismissal and shows the modal again', () => {
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        const { unmount } = render(<WhatsNewModal mode="status" />)
        expect(screen.getByText('1.3.0')).toBeInTheDocument()
        unmount()

        // A brand new mount is a new component instance with fresh state
        // — this is what a real re-login / full page load looks like.
        setStatusData({ show_changelog: true, versions: [entryWithEverything] })
        render(<WhatsNewModal mode="status" />)

        expect(screen.getByText('1.3.0')).toBeInTheDocument()
      })
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

  describe('browse mode loading, error and empty states', () => {
    function setAllQueryState(state: Record<string, unknown>) {
      mockUseChangelogAll.mockReturnValue({
        data: undefined,
        isError: false,
        isLoading: false,
        isPending: false,
        isSuccess: false,
        ...state,
      } as unknown as ReturnType<typeof useChangelogAll>)
    }

    it('shows a loading status while the history is fetching', () => {
      setAllQueryState({ isLoading: true })
      render(<WhatsNewModal mode="browse" open onClose={vi.fn()} />)

      expect(screen.getByRole('status')).toHaveTextContent('common.loading')
      expect(screen.queryByRole('region')).not.toBeInTheDocument()
    })

    it('shows an alert when the history fails to load', () => {
      setAllQueryState({ isError: true })
      render(<WhatsNewModal mode="browse" open onClose={vi.fn()} />)

      expect(screen.getByRole('alert')).toHaveTextContent('whatsNew.loadError')
      expect(screen.queryByRole('region')).not.toBeInTheDocument()
    })

    it('shows the empty message when there are no versions', () => {
      setAllData({ versions: [] })
      render(<WhatsNewModal mode="browse" open onClose={vi.fn()} />)

      expect(screen.getByText('whatsNew.empty')).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'whatsNew.showOlder' })).not.toBeInTheDocument()
    })

    it('shows the empty message when the response carries no data', () => {
      setAllQueryState({ isSuccess: true })
      render(<WhatsNewModal mode="browse" open onClose={vi.fn()} />)

      expect(screen.getByText('whatsNew.empty')).toBeInTheDocument()
    })

    it('still shows the Close button in every state', () => {
      setAllQueryState({ isError: true })
      render(<WhatsNewModal mode="browse" open onClose={vi.fn()} />)

      expect(screen.getByRole('button', { name: 'whatsNew.closeButton' })).toBeInTheDocument()
    })
  })

  describe('browse mode paging', () => {
    const manyVersions: ChangelogAll = {
      versions: Array.from({ length: 25 }, (_, i) => ({
        version: `2.${i}.0`,
        date: '2026-01-01',
        features: [`Feature ${i}`],
        fixes: [],
        other: [],
        security: [],
      })),
    }

    it('renders only the newest 10 versions initially', () => {
      setAllData(manyVersions)
      render(<WhatsNewModal mode="browse" open onClose={vi.fn()} />)

      expect(screen.getAllByRole('region')).toHaveLength(10)
      expect(screen.getByRole('region', { name: '2.0.0' })).toBeInTheDocument()
      expect(screen.getByRole('region', { name: '2.9.0' })).toBeInTheDocument()
      expect(screen.queryByRole('region', { name: '2.10.0' })).not.toBeInTheDocument()
    })

    it('reveals 10 more versions each time "Show older releases" is clicked', async () => {
      const user = userEvent.setup()
      setAllData(manyVersions)
      render(<WhatsNewModal mode="browse" open onClose={vi.fn()} />)

      const showOlder = screen.getByRole('button', { name: 'whatsNew.showOlder' })
      await user.click(showOlder)
      expect(screen.getAllByRole('region')).toHaveLength(20)
      expect(screen.getByRole('region', { name: '2.19.0' })).toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: 'whatsNew.showOlder' }))
      expect(screen.getAllByRole('region')).toHaveLength(25)
      expect(screen.queryByRole('button', { name: 'whatsNew.showOlder' })).not.toBeInTheDocument()
    })

    it('omits the "Show older releases" button when everything fits on one page', () => {
      setAllData({ versions: manyVersions.versions.slice(0, 10) })
      render(<WhatsNewModal mode="browse" open onClose={vi.fn()} />)

      expect(screen.getAllByRole('region')).toHaveLength(10)
      expect(screen.queryByRole('button', { name: 'whatsNew.showOlder' })).not.toBeInTheDocument()
    })

    it('resets to the first page each time the dialog is reopened', async () => {
      const user = userEvent.setup()
      const onClose = vi.fn()
      setAllData(manyVersions)
      const { rerender } = render(<WhatsNewModal mode="browse" open onClose={onClose} />)

      await user.click(screen.getByRole('button', { name: 'whatsNew.showOlder' }))
      expect(screen.getAllByRole('region')).toHaveLength(20)

      rerender(<WhatsNewModal mode="browse" open={false} onClose={onClose} />)
      rerender(<WhatsNewModal mode="browse" open onClose={onClose} />)

      expect(screen.getAllByRole('region')).toHaveLength(10)
    })
  })

  describe('rendering safety', () => {
    it('renders duplicate strings across groups without React duplicate-key warnings', () => {
      const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
      const duplicated: ChangelogEntry = {
        version: '1.5.0',
        date: '2026-08-01',
        features: ['Same text', 'Same text'],
        fixes: ['Same text'],
        other: ['Same text', 'Same text'],
        security: [
          { summary: 'Same security item', sha: 'abcdef1' },
          { summary: 'Same security item', sha: 'abcdef1' },
        ],
      }
      setStatusData({ show_changelog: true, versions: [duplicated] })
      render(<WhatsNewModal mode="status" />)

      expect(screen.getAllByText('Same text')).toHaveLength(5)
      expect(screen.getAllByText('Same security item', { exact: false })).toHaveLength(2)

      const keyWarnings = consoleError.mock.calls.filter((args: unknown[]) =>
        /same key|unique "key"/i.test(String(args[0]))
      )
      expect(keyWarnings).toHaveLength(0)
      consoleError.mockRestore()
    })

    it.each(['not-a-sha', 'ABCDEF1234', 'abc12', 'abc123g', 'a'.repeat(41), 'abc1234 onclick=x'])(
      'renders no commit link for the non-hex or out-of-range sha %j',
      (sha) => {
        const entry: ChangelogEntry = {
          ...entryFeaturesOnly,
          security: [{ summary: 'Hardened the API layer', sha }],
        }
        setStatusData({ show_changelog: true, versions: [entry] })
        render(<WhatsNewModal mode="status" />)

        expect(screen.getByText('Hardened the API layer')).toBeInTheDocument()
        expect(screen.queryByRole('link', { name: 'whatsNew.viewCommit' })).not.toBeInTheDocument()
      }
    )

    it.each(['abc1234', 'a'.repeat(40)])('links a valid commit sha %s at the length boundaries', (sha) => {
      const entry: ChangelogEntry = {
        ...entryFeaturesOnly,
        security: [{ summary: 'Hardened the API layer', sha }],
      }
      setStatusData({ show_changelog: true, versions: [entry] })
      render(<WhatsNewModal mode="status" />)

      expect(screen.getByRole('link', { name: 'whatsNew.viewCommit' })).toHaveAttribute(
        'href',
        `https://github.com/Wikid82/Charon/commit/${sha}`
      )
    })

    it('hides the decorative heading icons from assistive technology', () => {
      setStatusData({ show_changelog: true, versions: [entryWithEverything] })
      render(<WhatsNewModal mode="status" />)

      const fixesHeading = screen.getByRole('heading', { name: 'whatsNew.fixes' })
      expect(fixesHeading.querySelector('[aria-hidden="true"]')).toHaveTextContent('🐛')
      expect(screen.getByRole('heading', { name: 'whatsNew.newFeatures' }).querySelector('[aria-hidden="true"]'))
        .toHaveTextContent('✨')
      expect(screen.getByRole('heading', { name: 'whatsNew.security' }).querySelector('[aria-hidden="true"]'))
        .toHaveTextContent('🔒')
    })
  })
})
