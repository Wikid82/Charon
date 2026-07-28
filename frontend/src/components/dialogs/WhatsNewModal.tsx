import { useState } from 'react'
import { useTranslation } from 'react-i18next'

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
  const ackMutation = useAckChangelog()

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
  if (statusQuery.isError || !statusQuery.data?.show_changelog) return null

  const entries = statusQuery.data.versions

  const dismiss = (action: ChangelogAckAction) => {
    ackMutation.mutate({ action, opt_out: optOut })
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
