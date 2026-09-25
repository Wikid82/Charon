import { AlertTriangle, Info, ShieldCheck } from 'lucide-react'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert } from './ui/Alert'
import { Badge } from './ui/Badge'
import { Card } from './ui/Card'
import { Skeleton } from './ui/Skeleton'
import { LOGIN_PROTECTION_DOCS_URL, TRUSTED_PROXIES_DOCS_URL } from '../constants/docs'
import { useLoginProtectionStatus } from '../hooks/useSecurity'

import type {
  AddrScope,
  ForwardedHeaderObservation,
  LoginProtectionBudget,
} from '../api/security'

/** Observations older than this no longer drive a warning or note. */
const RECENT_WINDOW_MS = 24 * 60 * 60 * 1000

function isRecent(observation: ForwardedHeaderObservation, nowMs: number): boolean {
  if (observation.count <= 0 || !observation.last_seen) return false
  const seenMs = Date.parse(observation.last_seen)
  return !Number.isNaN(seenMs) && nowMs - seenMs <= RECENT_WINDOW_MS
}

/** "/32" for IPv4 peers, "/128" for IPv6 peers. */
function trustedProxySuggestion(peer: string): string {
  return `${peer}${peer.includes(':') ? '/128' : '/32'}`
}

function formatWindow(seconds: number, language: string): string {
  const useMinutes = seconds >= 60 && seconds % 60 === 0
  return new Intl.NumberFormat(language, {
    style: 'unit',
    unit: useMinutes ? 'minute' : 'second',
    unitDisplay: 'long',
  }).format(useMinutes ? seconds / 60 : seconds)
}

function formatRelative(iso: string, nowMs: number, language: string): string {
  const diffSeconds = Math.round((Date.parse(iso) - nowMs) / 1000)
  const formatter = new Intl.RelativeTimeFormat(language, { numeric: 'auto' })
  const abs = Math.abs(diffSeconds)
  if (abs < 60) return formatter.format(diffSeconds, 'second')
  return abs < 3600 ? formatter.format(Math.round(diffSeconds / 60), 'minute') : formatter.format(Math.round(diffSeconds / 3600), 'hour');
}

/**
 * Admin-only card showing whether sign-in throttling is on, what address Charon
 * sees for the viewer, and whether a reverse proxy appears to be misconfigured.
 */
export function LoginProtectionCard() {
  const { t, i18n } = useTranslation()
  const headingId = useId()
  const { data, isLoading, isError } = useLoginProtectionStatus({ enabled: true })
  const language = i18n.language
  const nowMs = Date.now()

  const scopeLabel = (scope: AddrScope) => t(`security.loginProtection.scope.${scope}`)
  const budgetLine = (label: string, budget: LoginProtectionBudget) =>
    t(label, { requests: budget.requests, window: formatWindow(budget.window_seconds, language) })

  let body: React.ReactNode
  if (isLoading) {
    body = <Skeleton className="h-16 w-full" data-testid="login-protection-skeleton" />
  } else if (isError || !data) {
    body = <p className="text-sm text-content-secondary">{t('security.loginProtection.loadError')}</p>
  } else if (!data.enabled) {
    body = (
      <>
        <p className="text-sm text-content-secondary">{t('security.loginProtection.disabledMessage')}</p>
        <DocsLink href={LOGIN_PROTECTION_DOCS_URL} label={t('security.loginProtection.docsLink')} />
      </>
    )
  } else {
    const { local, public: publicObs } = data.untrusted_forwarded_headers
    const showWarning = isRecent(local, nowMs)
    const showInfo = isRecent(publicObs, nowMs)
    body = (
      <>
        <ul className="text-sm text-content-secondary space-y-1">
          <li>{budgetLine('security.loginProtection.budgetLogin', data.login)}</li>
          <li>{budgetLine('security.loginProtection.budgetSession', data.session)}</li>
          <li>
            {t('security.loginProtection.callerAddress', {
              address: data.caller_client_key,
              scope: scopeLabel(data.caller_client_scope),
            })}
          </li>
        </ul>
        <p className="text-sm text-content-muted">{t('security.loginProtection.runtimeNatHint')}</p>
        {showWarning && (
          <Alert variant="warning" icon={AlertTriangle}>
            {t('security.loginProtection.untrustedPrivate', {
              count: local.count,
              peer: local.last_peer,
              when: formatRelative(local.last_seen as string, nowMs, language),
              suggestion: trustedProxySuggestion(local.last_peer),
            })}
          </Alert>
        )}
        {showInfo && (
          <Alert variant="info" icon={Info}>
            {t('security.loginProtection.untrustedPublic', {
              count: publicObs.count,
              peer: publicObs.last_peer,
              when: formatRelative(publicObs.last_seen as string, nowMs, language),
            })}
          </Alert>
        )}
        <DocsLink href={TRUSTED_PROXIES_DOCS_URL} label={t('security.loginProtection.docsLink')} />
      </>
    )
  }

  return (
    <section aria-labelledby={headingId}>
      <Card className="p-6 space-y-3">
        <div className="flex items-center gap-3">
          <ShieldCheck className="w-6 h-6 text-content-secondary" aria-hidden="true" />
          <h2 id={headingId} className="text-lg font-semibold text-content-primary">
            {t('security.loginProtection.title')}
          </h2>
          {data && (
            <Badge variant={data.enabled ? 'success' : 'default'}>
              {data.enabled ? t('security.loginProtection.statusOn') : t('security.loginProtection.statusOff')}
            </Badge>
          )}
        </div>
        {body}
      </Card>
    </section>
  )
}

function DocsLink({ href, label }: { href: string; label: string }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-block text-sm text-blue-400 hover:text-blue-300 underline"
    >
      {label}
    </a>
  )
}
