import { Link2, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { ChainEntry } from '../api/certificates'

interface CertificateChainViewerProps {
  chain: ChainEntry[]
}

function getChainLabel(index: number, total: number, t: (key: string) => string): string {
  if (index === 0) return t('certificates.chainLeaf')
  if (index === total - 1 && total > 1) return t('certificates.chainRoot')
  return t('certificates.chainIntermediate')
}

export default function CertificateChainViewer({ chain }: CertificateChainViewerProps) {
  const { t } = useTranslation()

  if (!chain || chain.length === 0) {
    return (
      <p className="text-sm text-content-muted italic">{t('certificates.noChainData')}</p>
    )
  }

  return (
    <div
      className="space-y-0"
      role="list"
      aria-label={t('certificates.certificateChain')}
    >
      {chain.map((entry, index) => {
        const label = getChainLabel(index, chain.length, t)
        const isLast = index === chain.length - 1

        return (
          <div key={index} role="listitem">
            <div className="flex items-start gap-3">
              <div className="flex flex-col items-center">
                <div className="flex h-8 w-8 items-center justify-center rounded-full border border-gray-700 bg-surface-muted">
                  {index === 0 ? (
                    <ShieldCheck className="h-4 w-4 text-brand-400" aria-hidden="true" />
                  ) : (
                    <Link2 className="h-4 w-4 text-content-muted" aria-hidden="true" />
                  )}
                </div>
                {!isLast && (
                  <div className="w-px h-6 bg-gray-700" aria-hidden="true" />
                )}
              </div>
              <div className="min-w-0 flex-1 pb-2">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-medium uppercase tracking-wide text-content-muted">
                    {label}
                  </span>
                </div>
                <p className="text-sm font-medium text-content-primary truncate" title={entry.subject}>
                  {entry.subject}
                </p>
                <p className="text-xs text-content-muted truncate" title={entry.issuer}>
                  {t('certificates.issuerOrg')}: {entry.issuer}
                </p>
                <p className="text-xs text-content-muted">
                  {t('certificates.expiresAt')}: {new Date(entry.expires_at).toLocaleDateString()}
                </p>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
