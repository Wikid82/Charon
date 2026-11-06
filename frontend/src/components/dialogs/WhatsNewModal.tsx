import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { useAuth } from '../../hooks/useAuth'
import { useAckChangelog, useChangelogAll, useChangelogStatus } from '../../hooks/useChangelog'
import { Button } from '../ui/Button'
import { Checkbox } from '../ui/Checkbox'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/Dialog'
import { Label } from '../ui/Label'

import type { ChangelogAckAction, ChangelogEntry } from '../../api/changelog'

const GITHUB_COMMIT_URL = 'https://github.com/Wikid82/Charon/commit/'

/** Commit shas are lowercase hex from `git log`; anything else gets no link (F17). */
const COMMIT_SHA_PATTERN = /^[0-9a-f]{7,40}$/

/** Browse mode renders the newest versions first and reveals this many more per "Show older releases" click (F20). */
const BROWSE_PAGE_SIZE = 10

/** Decorative icon shown before each group heading. It is `aria-hidden` so screen readers skip it (F21). */
const SECTION_ICONS = {
  features: '✨',
  fixes: '🐛',
  security: '🔒',
} as const

/**
 * Props for {@link WhatsNewModal}.
 *
 * - `status` mode is the post-auth, self-gating auto-check: it takes no
 *   props besides `mode`, fetches its own visibility via
 *   `useChangelogStatus()`, and manages its own dismiss/ack flow
 *   internally. Mount it unconditionally (e.g. once in `Layout.tsx`) — it
 *   renders `null` until there is something to show.
 * - `browse` mode is the voluntary "revisit changelog" entry point (e.g.
 *   the Appearance Settings "What's New" link): visibility is fully
 *   controlled by the caller via `open`/`onClose`, and it never calls the
 *   `ack` endpoint.
 */
export type WhatsNewModalProps =
  | { mode: 'status' }
  | { mode: 'browse'; open: boolean; onClose: () => void }

/** True when a commit sha is safe to place in a GitHub commit URL. */
function isCommitSha(sha: string): boolean {
  return COMMIT_SHA_PATTERN.test(sha)
}

interface SectionHeadingProps {
  icon: string
  label: string
}

function SectionHeading({ icon, label }: SectionHeadingProps) {
  return (
    <h4 className="mb-1 text-sm font-medium text-content-secondary">
      <span aria-hidden="true">{icon}</span> {label}
    </h4>
  )
}

interface ChangelogEntrySectionProps {
  entry: ChangelogEntry
}

function ChangelogEntrySection({ entry }: ChangelogEntrySectionProps) {
  const { t } = useTranslation()
  // Defensive hardening: the four group fields are typed as nullable (see
  // `ChangelogEntry` in `api/changelog.ts`) because the backend can
  // legitimately serialize a nil slice as JSON `null`, and hand-edited local
  // dev fixture data can omit a key outright. The API contract guarantees
  // these are populated arrays in practice, but this fallback keeps a
  // contract drift (or a stale/hand-edited fixture) from crashing the
  // modal — a null group is simply treated as empty and omitted.
  const features = entry.features ?? []
  const fixes = entry.fixes ?? []
  const security = entry.security ?? []
  const other = entry.other ?? []

  return (
    <section className="space-y-3" aria-label={entry.version}>
      <h3 className="text-base font-semibold text-content-primary">
        {entry.version}
        <span className="ml-2 text-sm font-normal text-content-secondary">{entry.date}</span>
      </h3>

      {features.length > 0 && (
        <div>
          <SectionHeading icon={SECTION_ICONS.features} label={t('whatsNew.newFeatures')} />
          <ul className="list-inside list-disc space-y-1 text-sm text-content-primary">
            {/* Index-prefixed: the same text can legitimately repeat across versions' lists (F18). */}
            {features.map((feature, index) => (
              <li key={`${index}-${feature}`}>{feature}</li>
            ))}
          </ul>
        </div>
      )}

      {fixes.length > 0 && (
        <div>
          <SectionHeading icon={SECTION_ICONS.fixes} label={t('whatsNew.fixes')} />
          <ul className="list-inside list-disc space-y-1 text-sm text-content-primary">
            {fixes.map((fix, index) => (
              <li key={`${index}-${fix}`}>{fix}</li>
            ))}
          </ul>
        </div>
      )}

      {security.length > 0 && (
        <div>
          <SectionHeading icon={SECTION_ICONS.security} label={t('whatsNew.security')} />
          <ul className="list-inside list-disc space-y-1 text-sm text-content-primary">
            {security.map((item, index) => (
              <li key={`${item.sha}-${index}`}>
                {item.summary}
                {isCommitSha(item.sha) && (
                  <>
                    {' '}
                    <a
                      href={`${GITHUB_COMMIT_URL}${item.sha}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-sm font-normal text-content-secondary underline"
                    >
                      {t('whatsNew.viewCommit')}
                    </a>
                  </>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      {other.length > 0 && (
        <details>
          <summary className="cursor-pointer text-sm font-medium text-content-secondary">
            {t('whatsNew.showMaintenanceDetails')}
          </summary>
          <ul className="mt-2 list-inside list-disc space-y-1 text-sm text-content-secondary">
            {other.map((item, index) => (
              <li key={`${index}-${item}`}>{item}</li>
            ))}
          </ul>
        </details>
      )}
    </section>
  )
}

/**
 * The voluntary "revisit changelog" dialog. Mounted only while open, so its
 * paging state resets every time it is reopened.
 */
function BrowseChangelogDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const allQuery = useChangelogAll(true)
  const [visibleCount, setVisibleCount] = useState(BROWSE_PAGE_SIZE)
  const entries = allQuery.data?.versions ?? []

  let content: ReactNode
  if (allQuery.isLoading) {
    content = <p role="status" className="text-sm text-content-muted">{t('common.loading')}</p>
  } else if (allQuery.isError) {
    content = <p role="alert" className="text-sm text-error">{t('whatsNew.loadError')}</p>
  } else if (entries.length === 0) {
    content = <p className="text-sm text-content-muted">{t('whatsNew.empty')}</p>
  } else {
    content = (
      <>
        {entries.slice(0, visibleCount).map((entry) => (
          <ChangelogEntrySection key={entry.version} entry={entry} />
        ))}
        {entries.length > visibleCount && (
          <div className="flex justify-center">
            <Button
              variant="secondary"
              onClick={() => setVisibleCount((count) => count + BROWSE_PAGE_SIZE)}
            >
              {t('whatsNew.showOlder')}
            </Button>
          </div>
        )}
      </>
    )
  }

  return (
    <Dialog open onOpenChange={(nextOpen) => { if (!nextOpen) onClose() }}>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t('whatsNew.title')}</DialogTitle>
        </DialogHeader>

        <div className="space-y-6 px-6">{content}</div>

        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            {t('whatsNew.closeButton')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function WhatsNewModal(props: WhatsNewModalProps) {
  const { t } = useTranslation()
  const [optOut, setOptOut] = useState(false)
  // Per-session dismissal flag, `status` mode only. `useChangelogStatus()`
  // is the ONLY source `status` mode uses to decide visibility, and both
  // `dismiss_temporary` and re-opting-in via Settings (`useOptInChangelog`)
  // invalidate that query. `dismiss_temporary` intentionally leaves
  // `last_seen_version` unchanged server-side ("modal reappears next
  // login" — see design spec), so the refetch it triggers still resolves
  // `show_changelog: true` and, without this flag, the modal would pop
  // right back open in the same session instead of staying closed until a
  // real future login. The same invalidation pattern lets a Settings
  // re-opt-in resurface the modal mid-interaction on the Settings page.
  // This flag short-circuits `status` mode's render once any dismissal
  // (temporary or permanent) has happened, regardless of what later
  // refetches the query returns.
  //
  // Plain React state (not `sessionStorage`) is deliberate: `status` mode
  // is mounted once, unconditionally, for the app session (see the mode
  // docstring above), so this state naturally persists across navigation
  // and naturally resets only on an actual fresh mount — i.e. a real page
  // load / re-login — which is exactly what "next login" means here.
  // `sessionStorage` would additionally survive an in-tab hard refresh
  // with no real re-login (arguably not "next login"), and would need
  // explicit clearing on logout to avoid suppressing the modal for a
  // different user on a shared browser/tab. Plain state avoids both
  // problems for free.
  const [dismissedThisSession, setDismissedThisSession] = useState(false)
  const ackMutation = useAckChangelog()
  // Only needed for `status` mode's opt-out refetch below, but hooks must
  // run unconditionally on every render (see the `browse`-mode early return
  // further down), so it's grabbed here alongside the other hooks.
  const { refetchUser } = useAuth()

  // Hooks must run unconditionally on every render regardless of mode.
  const statusQuery = useChangelogStatus()

  if (props.mode === 'browse') {
    return props.open ? <BrowseChangelogDialog onClose={props.onClose} /> : null
  }

  // status mode: self-gated, non-blocking. No toast on fetch failure — a
  // failed background check should not interrupt the user (§3.11).
  if (statusQuery.isError || dismissedThisSession || !statusQuery.data?.show_changelog) {
    return null
  }

  const entries = statusQuery.data.versions

  const dismiss = (action: ChangelogAckAction) => {
    // Close immediately (and stay closed for the rest of this session)
    // rather than waiting on the mutation — see `dismissedThisSession`
    // above for why this must not depend on the subsequent query refetch.
    setDismissedThisSession(true)
    // `ack` updates `user.changelog_opt_out` server-side whenever the
    // checkbox was checked (any of the three dismiss paths). AuthContext's
    // `user` is separate query state that `ack`'s own cache invalidation
    // (`changelog-status`) does not touch, so without this the Appearance
    // Settings toggle would show a stale value until a full reload — same
    // fix AppearanceSettings.tsx already applies for its own opt-in/opt-out
    // mutations. Scoped to `optOut` (not unconditional) because the user
    // record only actually changes on that path; refetching on every plain
    // dismiss would be a wasted request the common case doesn't need.
    if (optOut) {
      ackMutation.mutate({ action, opt_out: optOut }, { onSuccess: () => { void refetchUser() } })
    } else {
      ackMutation.mutate({ action, opt_out: optOut })
    }
  }

  return (
    <Dialog open onOpenChange={(nextOpen) => { if (!nextOpen) dismiss('dismiss_temporary') }}>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t('whatsNew.title')}</DialogTitle>
        </DialogHeader>

        <div className="space-y-6 px-6">
          {entries.map((entry) => (
            <ChangelogEntrySection key={entry.version} entry={entry} />
          ))}
        </div>

        <DialogFooter className="sm:items-center sm:justify-between">
          <div className="flex items-center gap-2">
            <Checkbox
              id="whats-new-opt-out"
              checked={optOut}
              onCheckedChange={(checked) => setOptOut(checked === true)}
            />
            <Label htmlFor="whats-new-opt-out">{t('whatsNew.dontShowAgain')}</Label>
          </div>
          <div className="flex gap-3">
            <Button variant="secondary" onClick={() => dismiss('dismiss_temporary')}>
              {t('whatsNew.remindLater')}
            </Button>
            <Button variant="primary" onClick={() => dismiss('dismiss_permanent')}>
              {t('whatsNew.gotIt')}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
