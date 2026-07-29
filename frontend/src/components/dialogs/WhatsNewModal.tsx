import { useState } from 'react'
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
import type { TFunction } from 'i18next'

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

interface ChangelogEntrySectionProps {
  entry: ChangelogEntry
  t: TFunction
}

function ChangelogEntrySection({ entry, t }: ChangelogEntrySectionProps) {
  return (
    <section className="space-y-3" aria-label={entry.version}>
      <h3 className="text-base font-semibold text-content-primary">
        {entry.version}
        <span className="ml-2 text-sm font-normal text-content-secondary">{entry.date}</span>
      </h3>

      {entry.features.length > 0 && (
        <div>
          <h4 className="mb-1 text-sm font-medium text-content-secondary">
            {t('whatsNew.newFeatures')}
          </h4>
          <ul className="list-inside list-disc space-y-1 text-sm text-content-primary">
            {entry.features.map((feature) => (
              <li key={feature}>{feature}</li>
            ))}
          </ul>
        </div>
      )}

      {entry.fixes.length > 0 && (
        <div>
          <h4 className="mb-1 text-sm font-medium text-content-secondary">{t('whatsNew.fixes')}</h4>
          <ul className="list-inside list-disc space-y-1 text-sm text-content-primary">
            {entry.fixes.map((fix) => (
              <li key={fix}>{fix}</li>
            ))}
          </ul>
        </div>
      )}

      {entry.other.length > 0 && (
        <details>
          <summary className="cursor-pointer text-sm font-medium text-content-secondary">
            {t('whatsNew.showMaintenanceDetails')}
          </summary>
          <ul className="mt-2 list-inside list-disc space-y-1 text-sm text-content-secondary">
            {entry.other.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </details>
      )}
    </section>
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

  const isBrowse = props.mode === 'browse'
  const browseOpen = isBrowse && props.open

  // Hooks must run unconditionally on every render regardless of mode.
  const statusQuery = useChangelogStatus()
  const allQuery = useChangelogAll(browseOpen)

  if (isBrowse) {
    if (!props.open) return null

    const entries = allQuery.data?.versions ?? []

    return (
      <Dialog open={props.open} onOpenChange={(nextOpen) => { if (!nextOpen) props.onClose() }}>
        <DialogContent className="max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t('whatsNew.title')}</DialogTitle>
          </DialogHeader>

          <div className="space-y-6 px-6">
            {entries.map((entry) => (
              <ChangelogEntrySection key={entry.version} entry={entry} t={t} />
            ))}
          </div>

          <DialogFooter>
            <Button variant="secondary" onClick={props.onClose}>
              {t('whatsNew.closeButton')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    )
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
            <ChangelogEntrySection key={entry.version} entry={entry} t={t} />
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
