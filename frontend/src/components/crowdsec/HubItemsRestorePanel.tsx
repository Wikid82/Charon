import { isAxiosError } from 'axios'
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { HubItem, ImportHubItems, RestoreHubItemsResult } from '../../api/crowdsec'
import { crowdsecBusyMessage } from '../../api/errorBody'
import { useRestoreHubItems } from '../../hooks/useCrowdsecHubItems'
import { Button } from '../ui/Button'

interface HubItemsRestorePanelProps {
  hubItems: ImportHubItems
}

const toItem = ({ type, name }: HubItem): HubItem => ({ type, name })

function ItemList({ items, label }: { items: HubItem[]; label: string }) {
  if (items.length === 0) return null
  return (
    <div>
      <h5 className="text-xs font-semibold text-gray-300">{label}</h5>
      <ul className="mt-1 space-y-0.5 text-sm">
        {items.map((item) => (
          <li key={`${item.type}/${item.name}`} className="flex gap-2">
            <span className="text-gray-500">{item.type}</span>
            <span className="font-mono">{item.name}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

/**
 * Passive follow-up to an import: lists the hub items the export had that this
 * host lacks and lets the user install them (or the remainder of a long run).
 */
export default function HubItemsRestorePanel({ hubItems }: HubItemsRestorePanelProps) {
  const { t } = useTranslation()
  const restore = useRestoreHubItems()
  // Kept across runs so a failed "Install remaining" does not hide the earlier result.
  const [result, setResult] = useState<RestoreHubItemsResult | null>(null)
  const run = (items: HubItem[]) => restore.mutate(items, { onSuccess: setResult })

  const status = restore.error && isAxiosError(restore.error) ? restore.error.response?.status : undefined
  let errorMessage: string | null = null
  if (restore.isError) {
    // Checked before the status: a 409 from a running install is not a restore conflict.
    const busy = crowdsecBusyMessage(restore.error, t)
    if (busy) errorMessage = busy
    else if (status === 502) errorMessage = t('crowdsecConfig.hubItems.errors.hubUnreachable')
    else if (status === 409) errorMessage = t('crowdsecConfig.hubItems.errors.alreadyRunning')
    else errorMessage = t('crowdsecConfig.hubItems.errors.generic')
  }

  const notAttempted = (result?.failed ?? []).filter((f) => f.reason === 'not_attempted').map(toItem)
  const failed = result?.failed ?? []

  return (
    <section
      className="rounded-lg border border-gray-700 p-4 space-y-3"
      aria-labelledby="hub-items-title"
      aria-busy={restore.isPending}
      data-testid="hub-items-panel"
    >
      <div className="space-y-1">
        <h4 id="hub-items-title" className="text-sm font-semibold">{t('crowdsecConfig.hubItems.title')}</h4>
        {!result && <p className="text-sm text-gray-400">{t('crowdsecConfig.hubItems.description')}</p>}
        {hubItems.skipped_invalid > 0 && (
          <p className="text-xs text-gray-500">{t('crowdsecConfig.hubItems.skippedInvalid', { count: hubItems.skipped_invalid })}</p>
        )}
      </div>

      {!result && <ItemList items={hubItems.missing} label={t('crowdsecConfig.hubItems.missingLabel')} />}

      <div role="status" aria-live="polite" className="space-y-3 text-sm">
        {restore.isPending && (
          <p className="flex items-center gap-2 text-gray-300">
            <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            {t('crowdsecConfig.hubItems.installing')}
          </p>
        )}
        {errorMessage && <p className="text-red-400">{errorMessage}</p>}
        {result && (
          <>
            <p>
              {t('crowdsecConfig.hubItems.summary', {
                installed: result.installed.length,
                alreadyInstalled: result.already_installed.length,
                failed: failed.length,
              })}
            </p>
            {result.remaining > 0 && (
              <p className="text-gray-400">{t('crowdsecConfig.hubItems.remaining', { count: result.remaining })}</p>
            )}
          </>
        )}
      </div>

      {result && (
        <div className="space-y-3">
          <ItemList items={result.installed} label={t('crowdsecConfig.hubItems.installedLabel')} />
          <ItemList items={result.already_installed} label={t('crowdsecConfig.hubItems.alreadyInstalledLabel')} />
          {failed.length > 0 && (
            <div>
              <h5 className="text-xs font-semibold text-gray-300">{t('crowdsecConfig.hubItems.failedLabel')}</h5>
              <ul className="mt-1 space-y-0.5 text-sm">
                {failed.map((item) => (
                  <li key={`${item.type}/${item.name}`} className="flex flex-wrap gap-2">
                    <span className="text-gray-500">{item.type}</span>
                    <span className="font-mono">{item.name}</span>
                    <span className="text-red-400">{t(`crowdsecConfig.hubItems.reasons.${item.reason}`, { defaultValue: t('crowdsecConfig.hubItems.reasons.install_failed') })}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}

      {!result && (
        <Button size="sm" onClick={() => run(hubItems.missing.map(toItem))} disabled={restore.isPending}>
          {t('crowdsecConfig.hubItems.install')}
        </Button>
      )}
      {result && result.remaining > 0 && notAttempted.length > 0 && (
        <Button size="sm" onClick={() => run(notAttempted)} disabled={restore.isPending}>
          {t('crowdsecConfig.hubItems.installRemaining')}
        </Button>
      )}
    </section>
  )
}
