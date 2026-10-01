import { Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from './ui/Button'
import type {
  DatabaseNotice as DatabaseNoticeData,
  DatabaseStatus,
} from '../api/databaseMaintenance'
import { useCancelOptimize, useRequestOptimize } from '../hooks/useDatabaseMaintenance'
import { formatBytes } from '../utils/formatBytes'
import { toast } from '../utils/toast'

/** Matches the server's `MinReclaimableBytes` floor (100 MiB). */
const MIN_RECLAIMABLE_BYTES = 100 * 1024 * 1024

const INFO_NOTICE_KEYS: Partial<Record<DatabaseNoticeData['code'], string>> = {
  restart_to_optimize: 'databaseMaintenance.notice.restartToOptimize',
  database_busy: 'databaseMaintenance.notice.databaseBusy',
  disabled_by_env: 'databaseMaintenance.notice.disabledByEnv',
}

/**
 * Passive status text for the notice the server currently reports. Always a plain
 * `role="status"` region (never an alert): these are things to read when the user
 * visits the Database page, not interruptions. Renders nothing without a notice.
 */
export function DatabaseNotice({ notice }: { notice: DatabaseNoticeData | null | undefined }) {
  const { t } = useTranslation()
  if (!notice) return null

  let title: string | undefined
  let body: string | undefined
  if (notice.code === 'insufficient_disk') {
    title = t('databaseMaintenance.notice.insufficientDiskTitle')
    body = t('databaseMaintenance.notice.insufficientDisk', {
      required: formatBytes(notice.required_bytes ?? 0),
      available: formatBytes(notice.available_bytes ?? 0),
    })
  } else if (notice.code === 'too_many_failures') {
    title = t('databaseMaintenance.notice.tooManyFailuresTitle')
    body = t('databaseMaintenance.notice.tooManyFailures')
  } else {
    const key = INFO_NOTICE_KEYS[notice.code]
    if (key) body = t(key, { size: formatBytes(notice.reclaimable_bytes) })
  }
  if (!body) return null

  return (
    <div role="status" className="flex gap-2 text-sm text-content-secondary">
      <Info className="mt-0.5 h-4 w-4 shrink-0 text-content-muted" aria-hidden="true" />
      <div className="space-y-1">
        {title && <p className="font-medium text-content-primary">{title}</p>}
        <p>{body}</p>
      </div>
    </div>
  )
}

const MODE_KEYS: Record<DatabaseStatus['auto_vacuum'], string> = {
  none: 'databaseMaintenance.mode.none',
  incremental: 'databaseMaintenance.mode.incremental',
  full: 'databaseMaintenance.mode.full',
}

/** Size, write-ahead log, free disk, reclaimable space and the optimization mode in plain words. */
export function DatabaseStatusList({ db }: { db: DatabaseStatus }) {
  const { t } = useTranslation()
  const modeKey = MODE_KEYS[db.auto_vacuum] as string | undefined

  const rows: Array<{ label: string; hint?: string; value: string }> = [
    { label: t('databaseMaintenance.fields.size'), value: formatBytes(db.size_bytes) },
    {
      label: t('databaseMaintenance.fields.wal'),
      hint: t('databaseMaintenance.fields.walHint'),
      value: formatBytes(db.wal_bytes),
    },
    { label: t('databaseMaintenance.fields.diskFree'), value: formatBytes(db.disk_free_bytes) },
    { label: t('databaseMaintenance.fields.reclaimable'), value: formatBytes(db.reclaimable_bytes) },
    // An unknown mode is omitted rather than shown as a raw value.
    ...(modeKey ? [{ label: t('databaseMaintenance.fields.mode'), value: t(modeKey) }] : []),
  ]

  return (
    <dl className="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
      {rows.map((row) => (
        <div key={row.label}>
          <dt className="text-content-muted">{row.label}</dt>
          <dd className="text-content-primary">{row.value}</dd>
          {row.hint && <dd className="text-xs text-content-muted">{row.hint}</dd>}
        </div>
      ))}
    </dl>
  )
}

const KNOWN_SKIP_REASONS = new Set([
  'integrity_check_failed',
  'database_busy',
  'caddy_not_ready',
  'startup_timeout',
  'shutting_down',
  'conversion_failed',
  'internal_error',
  'insufficient_disk',
  'too_many_failures',
  'disabled_by_env',
  'already_optimized',
  'nothing_to_reclaim',
  'below_threshold',
])

/** Locale date for an ISO timestamp, or an empty string when it cannot be parsed (never "Invalid Date"). */
function formatDate(at: string, locale: string): string {
  const date = new Date(at)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString(locale)
}

/** What the last automatic or manual optimization did, in plain words with no raw codes. */
export function LastOptimization({ db }: { db: DatabaseStatus }) {
  const { t, i18n } = useTranslation()
  const last = db.last_result

  let text: string
  if (!last) {
    // A legacy database that can still be reclaimed has not been optimized yet,
    // even when the button is disabled only because the server switched it off.
    const reclaimableLater =
      db.auto_vacuum !== 'incremental' && db.reclaimable_bytes >= MIN_RECLAIMABLE_BYTES
    text =
      db.can_request_optimize || db.compact_requested || reclaimableLater
        ? t('databaseMaintenance.last.notYet')
        : t('databaseMaintenance.last.none')
  } else {
    const date = formatDate(last.at, i18n.language)
    const when = date ? t('databaseMaintenance.last.on', { date }) : ''
    const sizes = {
      when,
      before: formatBytes(last.bytes_before),
      after: formatBytes(last.bytes_after),
      saved: formatBytes(Math.max(last.bytes_before - last.bytes_after, 0)),
    }
    switch (last.outcome) {
      case 'converted':
        text = t('databaseMaintenance.last.converted', sizes)
        break
      case 'converted_pending_checkpoint':
        text = t('databaseMaintenance.last.convertedPending', sizes)
        break
      case 'skipped': {
        const reasonKey = KNOWN_SKIP_REASONS.has(last.reason) ? last.reason : 'unknown'
        text = t('databaseMaintenance.last.skipped', {
          when,
          reason: t(`databaseMaintenance.reason.${reasonKey}`),
        })
        break
      }
      case 'failed':
      case 'interrupted':
      case 'cancelled':
        text = t(`databaseMaintenance.last.${last.outcome}`, { when })
        break
      default:
        text = t('databaseMaintenance.last.unknown', { when })
    }
  }

  return (
    <div className="space-y-1">
      <h4 className="text-sm font-medium text-content-primary">{t('databaseMaintenance.last.title')}</h4>
      <p className="text-sm text-content-secondary">{text}</p>
    </div>
  )
}

/** Why the reclaim button is disabled; the server's `can_request_optimize` stays the only thing that enables it. */
function disabledReasonKey(db: DatabaseStatus): string {
  if (db.auto_vacuum === 'incremental') return 'databaseMaintenance.reasonIncremental'
  if (db.reclaimable_bytes < MIN_RECLAIMABLE_BYTES) return 'databaseMaintenance.reasonTooSmall'
  if (db.env_mode === 'off') return 'databaseMaintenance.disabledReason'
  return 'databaseMaintenance.reasonUnavailable'
}

/**
 * Optional "reclaim space" control. Always rendered; the button is enabled only when
 * the server allows it and otherwise carries the reason it is disabled.
 */
export function ReclaimControl({ db }: { db: DatabaseStatus }) {
  const { t } = useTranslation()
  const request = useRequestOptimize()
  const cancel = useCancelOptimize()
  const envOff = db.env_mode === 'off'
  const reasonId = 'database-reclaim-reason'

  const handleRequest = () =>
    request.mutate(undefined, {
      onError: (err: unknown) =>
        toast.error(
          t('databaseMaintenance.requestFailed', { error: err instanceof Error ? err.message : String(err) })
        ),
    })

  const handleUndo = () =>
    cancel.mutate(undefined, {
      onError: (err: unknown) =>
        toast.error(
          t('databaseMaintenance.undoFailed', { error: err instanceof Error ? err.message : String(err) })
        ),
    })

  return (
    <div className="space-y-2">
      <div>
        <h4 className="text-sm font-medium text-content-primary">{t('databaseMaintenance.reclaimTitle')}</h4>
        <p className="text-sm text-content-muted">{t('databaseMaintenance.reclaimOptional')}</p>
      </div>
      {db.compact_requested ? (
        <div className="flex flex-wrap items-center gap-3">
          {/* With the env off the info notice already explains why nothing will happen. */}
          {!envOff && <p className="text-sm text-content-secondary">{t('databaseMaintenance.scheduled')}</p>}
          <Button variant="secondary" size="sm" onClick={handleUndo} isLoading={cancel.isPending}>
            {t('databaseMaintenance.undo')}
          </Button>
        </div>
      ) : (
        <>
          <Button
            variant="secondary"
            onClick={handleRequest}
            isLoading={request.isPending}
            disabled={!db.can_request_optimize}
            aria-describedby={db.can_request_optimize ? undefined : reasonId}
          >
            {t('databaseMaintenance.reclaimButton')}
          </Button>
          <p id={db.can_request_optimize ? undefined : reasonId} className="text-sm text-content-muted">
            {db.can_request_optimize
              ? t('databaseMaintenance.reclaimHelper', { size: formatBytes(db.reclaimable_bytes) })
              : t(disabledReasonKey(db))}
          </p>
        </>
      )}
    </div>
  )
}
