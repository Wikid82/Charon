import { Database } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from './ui/Alert'
import { Button } from './ui/Button'
import { Card, CardHeader, CardTitle, CardContent } from './ui/Card'
import type { DatabaseNotice, DatabaseStatus } from '../api/databaseMaintenance'
import { useCancelOptimize, useDatabaseStatus, useRequestOptimize } from '../hooks/useDatabaseMaintenance'
import { formatBytes } from '../utils/formatBytes'
import { toast } from '../utils/toast'

/** Matches the server's `MinReclaimableBytes` floor (100 MiB). */
const MIN_RECLAIMABLE_BYTES = 100 * 1024 * 1024

const INFO_NOTICE_KEYS: Partial<Record<DatabaseNotice['code'], string>> = {
  restart_to_optimize: 'systemSettings.database.notice.restartToOptimize',
  database_busy: 'systemSettings.database.notice.databaseBusy',
  disabled_by_env: 'systemSettings.database.notice.disabledByEnv',
}

/**
 * Page-top banner for `warning` notices (not enough disk, repeated failures).
 * Renders nothing in normal operation and for info notices (those stay in the card).
 */
export function DatabaseMaintenanceBanner() {
  const { t } = useTranslation()
  const { data } = useDatabaseStatus()
  const notice = data?.notice
  if (!notice || notice.severity !== 'warning') return null

  if (notice.code === 'insufficient_disk') {
    const required = notice.required_bytes ?? 0
    const available = notice.available_bytes ?? 0
    return (
      <Alert
        variant="warning"
        title={t('systemSettings.database.banner.insufficientDiskTitle')}
        aria-label={t('systemSettings.database.banner.insufficientDiskTitle')}
      >
        <AlertDescription>
          {t('systemSettings.database.banner.insufficientDisk', {
            shortfall: formatBytes(Math.max(required - available, 0)),
            required: formatBytes(required),
            available: formatBytes(available),
          })}
        </AlertDescription>
      </Alert>
    )
  }

  if (notice.code === 'too_many_failures') {
    return (
      <Alert
        variant="warning"
        title={t('systemSettings.database.banner.tooManyFailuresTitle')}
        aria-label={t('systemSettings.database.banner.tooManyFailuresTitle')}
      >
        <AlertDescription>{t('systemSettings.database.banner.tooManyFailures')}</AlertDescription>
      </Alert>
    )
  }

  return null
}

/** Whether the "Reclaim space on next restart" control should exist at all. */
function showReclaimControl(db: DatabaseStatus): boolean {
  if (db.auto_vacuum === 'incremental' || db.reclaimable_bytes < MIN_RECLAIMABLE_BYTES) return false
  // Only the env switch justifies a visible-but-disabled button.
  return db.can_request_optimize || db.env_mode === 'off'
}

function ReclaimControl({ db }: { db: DatabaseStatus }) {
  const { t } = useTranslation()
  const request = useRequestOptimize()
  const cancel = useCancelOptimize()
  const envOff = db.env_mode === 'off'

  const handleRequest = () =>
    request.mutate(undefined, {
      onError: (err: unknown) =>
        toast.error(
          t('systemSettings.database.requestFailed', { error: err instanceof Error ? err.message : String(err) })
        ),
    })

  const handleUndo = () =>
    cancel.mutate(undefined, {
      onError: (err: unknown) =>
        toast.error(
          t('systemSettings.database.undoFailed', { error: err instanceof Error ? err.message : String(err) })
        ),
    })

  if (db.compact_requested) {
    return (
      <div className="flex flex-wrap items-center gap-3">
        {/* With the env off the info notice already explains why nothing will happen. */}
        {!envOff && (
          <p className="text-sm text-content-secondary">{t('systemSettings.database.scheduled')}</p>
        )}
        <Button variant="secondary" size="sm" onClick={handleUndo} isLoading={cancel.isPending}>
          {t('systemSettings.database.undo')}
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <Button
        variant="secondary"
        onClick={handleRequest}
        isLoading={request.isPending}
        disabled={!db.can_request_optimize}
      >
        {t('systemSettings.database.reclaimButton')}
      </Button>
      {envOff && (
        <p className="text-sm text-content-muted">{t('systemSettings.database.disabledReason')}</p>
      )}
    </div>
  )
}

/**
 * "Database" card in System Settings. Quiet by default: the size, plus a notice
 * line and the reclaim control only when there is something to say or do.
 * Renders nothing when the status is unavailable (for example non-admin users).
 */
export function DatabaseMaintenanceCard() {
  const { t } = useTranslation()
  const { data: db } = useDatabaseStatus()
  if (!db) return null

  const infoNoticeKey = db.notice?.severity === 'info' ? INFO_NOTICE_KEYS[db.notice.code] : undefined
  const reclaimable = db.auto_vacuum !== 'incremental' && db.reclaimable_bytes >= MIN_RECLAIMABLE_BYTES

  return (
    <Card role="region" aria-labelledby="database-card-title">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Database className="h-5 w-5 text-content-muted" aria-hidden="true" />
          <CardTitle id="database-card-title">{t('systemSettings.database.title')}</CardTitle>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-content-secondary">
          {t('systemSettings.database.size', { size: formatBytes(db.size_bytes) })}
          {reclaimable && (
            <>
              {' '}
              {t('systemSettings.database.reclaimable', { size: formatBytes(db.reclaimable_bytes) })}
            </>
          )}
        </p>
        {infoNoticeKey && db.notice && (
          <p className="text-sm text-content-secondary">
            {t(infoNoticeKey, { size: formatBytes(db.notice.reclaimable_bytes) })}
          </p>
        )}
        {showReclaimControl(db) && <ReclaimControl db={db} />}
      </CardContent>
    </Card>
  )
}
