import { Database, RefreshCw, RotateCcw, Upload } from 'lucide-react'
import { useId, useRef, useState, type ChangeEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { getApiErrorCode, geoipErrorKey, GEOIP_ERROR_CODES } from '../api/errorBody'
import { GEOIP_TROUBLESHOOTING_DOCS_URL } from '../constants/docs'
import {
  useGeoIPStatus,
  useRemoveGeoIPDatabase,
  useReloadGeoIP,
  useReplaceGeoIPDatabase,
} from '../hooks/useGeoIP'
import { GeoIPLookupForm } from './GeoIPLookupForm'
import { Badge } from './ui/Badge'
import { Button } from './ui/Button'
import { Card } from './ui/Card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/Dialog'
import { Skeleton } from './ui/Skeleton'

import type { GeoIPStatus } from '../api/security'
import type { TFunction } from 'i18next'

/** Largest database the server accepts. The UI shows it as a hint; the server enforces it. */
const GEOIP_UPLOAD_MAX_MIB = 32

/** Outcome of the last card action, shown inline in the polite status region. */
interface Feedback {
  tone: 'success' | 'error'
  text: string
}

/** Formats an RFC 3339 build date, or returns null when it is missing or unreadable. */
function formatBuildDate(iso: string | null, language: string): string | null {
  if (!iso) return null
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? null : new Intl.DateTimeFormat(language, { dateStyle: 'medium' }).format(date)
}

/** Formats a file size in megabytes, or returns null when the size is unknown. */
function formatSize(bytes: number | null, language: string): string | null {
  if (bytes === null) return null
  return new Intl.NumberFormat(language, {
    style: 'unit',
    unit: 'megabyte',
    maximumFractionDigits: 1,
  }).format(bytes / 1_000_000)
}

/** Text of a refused card action, from the server's error_code. */
function errorText(error: unknown, t: TFunction): string {
  return t(geoipErrorKey(getApiErrorCode(error, GEOIP_ERROR_CODES)))
}

/**
 * Admin-only card on the Security page. It shows which GeoIP country database the proxy uses, reloads it, replaces
 * the data-folder copy from an uploaded file and returns to the included copy. Results appear inline, not as toasts.
 */
export function GeoIPDatabaseCard() {
  const { t, i18n } = useTranslation()
  const headingId = useId()
  const { data: status, isLoading, isError } = useGeoIPStatus()
  const reload = useReloadGeoIP()
  const replace = useReplaceGeoIPDatabase()
  const remove = useRemoveGeoIPDatabase()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [feedback, setFeedback] = useState<Feedback | null>(null)
  const busy = reload.isPending || replace.isPending || remove.isPending

  const succeed = (text: string) => setFeedback({ tone: 'success', text })
  const fail = (error: unknown) => setFeedback({ tone: 'error', text: errorText(error, t) })

  const onReload = () => {
    setFeedback(null)
    reload.mutate(undefined, {
      onSuccess: () => succeed(t('security.geoip.result.reloaded')),
      onError: fail,
    })
  }

  const onFileChosen = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    // Clear the input so choosing the same file again still fires a change.
    event.target.value = ''
    if (!file) return
    setFeedback(null)
    replace.mutate(file, {
      onSuccess: () => succeed(t('security.geoip.result.replaced')),
      onError: fail,
    })
  }

  const onConfirmRemove = () => {
    remove.mutate(undefined, {
      onSuccess: () => {
        setConfirmOpen(false)
        succeed(t('security.geoip.result.removed'))
      },
      onError: (error) => {
        setConfirmOpen(false)
        fail(error)
      },
    })
  }

  let body: ReactNode
  if (isLoading) {
    body = <Skeleton className="h-24 w-full" data-testid="geoip-card-skeleton" />
  } else if (isError || !status) {
    body = <p className="text-sm text-content-secondary">{t('security.geoip.loadError')}</p>
  } else {
    body = (
      <>
        <GeoIPDetails status={status} language={i18n.language} t={t} />
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="secondary" onClick={onReload} disabled={busy}>
            <RefreshCw className="w-4 h-4 mr-2" aria-hidden="true" />
            {t('security.geoip.actions.reload')}
          </Button>
          {status.can_replace && (
            <>
              <Button variant="secondary" onClick={() => fileInputRef.current?.click()} disabled={busy}>
                <Upload className="w-4 h-4 mr-2" aria-hidden="true" />
                {t('security.geoip.actions.replace')}
              </Button>
              <input
                ref={fileInputRef}
                data-testid="geoip-file-input"
                type="file"
                accept=".mmdb"
                hidden
                onChange={onFileChosen}
              />
            </>
          )}
          {status.source === 'data' && status.can_replace && (
            <Button variant="ghost" onClick={() => setConfirmOpen(true)} disabled={busy}>
              <RotateCcw className="w-4 h-4 mr-2" aria-hidden="true" />
              {t('security.geoip.actions.useIncluded')}
            </Button>
          )}
        </div>
        {status.can_replace && (
          <p className="text-xs text-content-muted">
            {t('security.geoip.actions.replaceHint', { max: GEOIP_UPLOAD_MAX_MIB })}
          </p>
        )}
        <p role="status" aria-live="polite" className={feedbackClass(feedback)}>
          {feedback?.text}
        </p>
        <GeoIPLookupForm loaded={status.loaded} />
        <UseIncludedDialog
          open={confirmOpen}
          onOpenChange={setConfirmOpen}
          onConfirm={onConfirmRemove}
          pending={remove.isPending}
          noBundledCopy={status.included_build_date === null}
          t={t}
        />
      </>
    )
  }

  return (
    <section aria-labelledby={headingId}>
      <Card className="p-6 space-y-4">
        <div className="flex flex-wrap items-center gap-3">
          <Database className="w-6 h-6 text-content-secondary" aria-hidden="true" />
          <h2 id={headingId} className="text-lg font-semibold text-content-primary">
            {t('security.geoip.title')}
          </h2>
          {status && (
            <Badge variant={status.available ? 'success' : 'warning'}>
              {status.available ? t('security.geoip.status.ready') : t('security.geoip.status.unavailable')}
            </Badge>
          )}
        </div>
        <p className="text-sm text-content-secondary">{t('security.geoip.description')}</p>
        {body}
      </Card>
    </section>
  )
}

function feedbackClass(feedback: Feedback | null): string {
  const base = 'text-sm min-h-[1.25rem]'
  return feedback?.tone === 'error' ? `${base} text-error` : `${base} text-content-secondary`
}

interface GeoIPDetailsProps {
  status: GeoIPStatus
  language: string
  t: TFunction
}

/** Source, dates, size and path of the database in use, with the reason it cannot be used or replaced. */
function GeoIPDetails({ status, language, t }: GeoIPDetailsProps) {
  const buildDate = formatBuildDate(status.build_date, language)
  const size = formatSize(status.size_bytes, language)
  const unknown = t('security.geoip.unknownValue')

  return (
    <>
      <dl className="grid grid-cols-1 sm:grid-cols-[max-content_1fr] gap-x-6 gap-y-2 text-sm">
        <dt className="text-content-muted">{t('security.geoip.source.label')}</dt>
        <dd className="text-content-primary">{t(`security.geoip.source.${status.source}`)}</dd>
        <dt className="text-content-muted">{t('security.geoip.buildDate')}</dt>
        <dd className="text-content-primary">{buildDate ?? unknown}</dd>
        <dt className="text-content-muted">{t('security.geoip.size')}</dt>
        <dd className="text-content-primary">{size ?? unknown}</dd>
        <dt className="text-content-muted">{t('security.geoip.path')}</dt>
        <dd className="font-mono text-xs text-content-muted break-all">{status.db_path || unknown}</dd>
      </dl>
      {status.unenforced_lists_total > 0 && (
        <p className="text-sm text-warning">
          {t('security.geoip.unenforcedLists', { count: status.unenforced_lists_total })}
        </p>
      )}
      {!status.available && (
        <>
          <p className="text-sm text-content-muted">{t('security.geoip.unavailableReason')}</p>
          <a
            href={GEOIP_TROUBLESHOOTING_DOCS_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-block text-sm text-blue-400 hover:text-blue-300 underline"
          >
            {t('security.geoip.docsLink')}
          </a>
        </>
      )}
      {status.replace_blocked_reason === 'custom_path_set' && (
        <p className="text-sm text-content-muted">{t('security.geoip.customPathNote')}</p>
      )}
      {status.replace_blocked_reason === 'data_folder_read_only' && (
        <p className="text-sm text-content-muted">{t('security.geoip.readOnlyNote')}</p>
      )}
      <p className="text-xs text-content-muted">{t('security.geoip.releaseNote')}</p>
    </>
  )
}

interface UseIncludedDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
  pending: boolean
  /** True when the image ships no database, so the copy removed leaves no country matching. */
  noBundledCopy: boolean
  t: TFunction
}

/** Confirmation before the data-folder copy is removed. */
function UseIncludedDialog({ open, onOpenChange, onConfirm, pending, noBundledCopy, t }: UseIncludedDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('security.geoip.actions.useIncludedConfirmTitle')}</DialogTitle>
          <DialogDescription>
            {noBundledCopy
              ? t('security.geoip.actions.useIncludedConfirmBodyNoBundled')
              : t('security.geoip.actions.useIncludedConfirmBody')}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={pending}>
            {t('common.cancel')}
          </Button>
          <Button variant="danger" onClick={onConfirm} disabled={pending}>
            {t('security.geoip.actions.useIncluded')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
