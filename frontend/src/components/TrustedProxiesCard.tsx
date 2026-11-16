import { Network, XCircle } from 'lucide-react'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { trustedProxyErrorMessage } from '../api/errorBody'
import { TRUSTED_PROXIES_DOCS_URL } from '../constants/docs'
import { useAuth } from '../hooks/useAuth'
import { useResetTrustedProxies, useSaveTrustedProxies, useTrustedProxies } from '../hooks/useSecurity'
import { parseTrustedProxyLines } from '../utils/trustedProxyLines'
import { DocsLink } from './DocsLink'
import { Alert } from './ui/Alert'
import { Button } from './ui/Button'
import { Card } from './ui/Card'
import { Label } from './ui/Label'
import { Skeleton } from './ui/Skeleton'
import { Textarea } from './ui/Textarea'

import type { TrustedProxySource } from '../api/security'

/** Locale key of the sentence that says where the list in force comes from. */
const SOURCE_KEYS: Record<TrustedProxySource, string> = {
  setting: 'security.trustedProxies.sourceSetting',
  environment: 'security.trustedProxies.sourceEnvironment',
  default: 'security.trustedProxies.sourceDefault',
}

/**
 * Security page card for the proxy-layer trust list. Administrators edit the list; everyone else sees where it
 * comes from and an administrators-only note. Notes are passive: nothing here asks for action.
 */
export function TrustedProxiesCard() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const headingId = useId()
  const fieldId = useId()
  const hintId = useId()
  const { data, isLoading, isError } = useTrustedProxies()
  const saveMutation = useSaveTrustedProxies()
  const resetMutation = useResetTrustedProxies()
  // Null until the administrator types; the saved list is shown otherwise.
  const [draft, setDraft] = useState<string | null>(null)

  const busy = saveMutation.isPending || resetMutation.isPending
  const fieldValue = draft ?? (data?.proxies ?? []).join('\n')
  const failure = saveMutation.error ?? resetMutation.error
  const errorText = failure
    ? (trustedProxyErrorMessage(failure, t) ?? t('security.trustedProxies.saveFailed'))
    : null
  const statusText = saveMutation.isSuccess
    ? t('security.trustedProxies.saved')
    : resetMutation.isSuccess
      ? t('security.trustedProxies.resetDone')
      : null

  const handleSave = () => {
    resetMutation.reset()
    saveMutation.mutate(parseTrustedProxyLines(fieldValue), { onSuccess: () => setDraft(null) })
  }

  const handleReset = () => {
    saveMutation.reset()
    resetMutation.mutate(undefined, { onSuccess: () => setDraft(null) })
  }

  let body: React.ReactNode
  if (isLoading) {
    body = <Skeleton className="h-24 w-full" data-testid="trusted-proxies-skeleton" />
  } else if (isError || !data) {
    body = <p className="text-sm text-content-secondary">{t('security.trustedProxies.loadError')}</p>
  } else {
    const dropped = isAdmin ? (data.dropped_entries ?? []) : []
    const warnings = isAdmin ? (data.warnings ?? []) : []
    const apiLayer = isAdmin ? (data.api_layer ?? []) : []
    body = (
      <>
        <p className="text-sm text-content-secondary">{t('security.trustedProxies.description')}</p>
        <ul className="text-sm text-content-secondary space-y-1">
          <li>{t('security.trustedProxies.alwaysTrusted', { entries: data.always_trusted.join(', ') })}</li>
          {isAdmin && (
            <li>
              {t('security.trustedProxies.apiLayerNote', {
                entries: apiLayer.length > 0 ? apiLayer.join(', ') : t('security.trustedProxies.apiLayerNone'),
              })}
            </li>
          )}
          <li>{t(SOURCE_KEYS[data.source])}</li>
          {dropped.length > 0 && (
            <li>{t('security.trustedProxies.droppedEntries', { entries: dropped.join(', ') })}</li>
          )}
        </ul>

        {isAdmin ? (
          <form
            className="space-y-3"
            onSubmit={(event) => {
              event.preventDefault()
              handleSave()
            }}
          >
            <div className="space-y-1">
              <Label htmlFor={fieldId}>{t('security.trustedProxies.fieldLabel')}</Label>
              <Textarea
                id={fieldId}
                rows={4}
                spellCheck={false}
                className="font-mono"
                value={fieldValue}
                disabled={busy}
                error={errorText !== null}
                aria-describedby={hintId}
                onChange={(event) => setDraft(event.target.value)}
              />
              <p id={hintId} className="text-xs text-content-muted">
                {t('security.trustedProxies.fieldHint')}
              </p>
            </div>
            {warnings.length > 0 && (
              <ul className="text-sm text-content-secondary space-y-1">
                {warnings.map((warning) => (
                  <li key={warning}>{warning}</li>
                ))}
              </ul>
            )}
            {errorText && (
              <Alert variant="error" icon={XCircle}>
                {errorText}
              </Alert>
            )}
            {statusText && (
              <p role="status" className="text-sm text-content-secondary">
                {statusText}
              </p>
            )}
            <div className="flex flex-wrap gap-2">
              <Button type="submit" disabled={busy}>
                {t('common.save')}
              </Button>
              {data.source === 'setting' && (
                <Button type="button" variant="secondary" disabled={busy} onClick={handleReset}>
                  {t('security.trustedProxies.reset')}
                </Button>
              )}
            </div>
          </form>
        ) : (
          <p className="text-sm text-content-muted">{t('permissions.adminOnlyError')}</p>
        )}
        <DocsLink href={TRUSTED_PROXIES_DOCS_URL} label={t('security.trustedProxies.docsLink')} />
      </>
    )
  }

  return (
    <section aria-labelledby={headingId}>
      <Card className="p-6 space-y-3">
        <div className="flex items-center gap-3">
          <Network className="w-6 h-6 text-content-secondary" aria-hidden="true" />
          <h2 id={headingId} className="text-lg font-semibold text-content-primary">
            {t('security.trustedProxies.title')}
          </h2>
        </div>
        {body}
      </Card>
    </section>
  )
}
