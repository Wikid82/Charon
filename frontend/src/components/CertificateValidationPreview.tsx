import { AlertTriangle, CheckCircle, XCircle } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { ValidationResult } from '../api/certificates'

interface CertificateValidationPreviewProps {
  result: ValidationResult
}

export default function CertificateValidationPreview({
  result,
}: CertificateValidationPreviewProps) {
  const { t } = useTranslation()

  return (
    <div
      className="rounded-lg border border-gray-700 bg-surface-muted/50 p-4 space-y-3"
      data-testid="certificate-validation-preview"
      role="region"
      aria-label={t('certificates.validationPreview')}
    >
      <div className="flex items-center gap-2">
        {result.valid ? (
          <CheckCircle className="h-5 w-5 text-green-400" aria-hidden="true" />
        ) : (
          <XCircle className="h-5 w-5 text-red-400" aria-hidden="true" />
        )}
        <span className="font-medium text-content-primary">
          {result.valid
            ? t('certificates.validCertificate')
            : t('certificates.invalidCertificate')}
        </span>
      </div>

      <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-sm">
        <dt className="text-content-muted">{t('certificates.commonName')}</dt>
        <dd className="text-content-primary">{result.common_name || '-'}</dd>

        <dt className="text-content-muted">{t('certificates.domains')}</dt>
        <dd className="text-content-primary">
          {result.domains?.length ? result.domains.join(', ') : '-'}
        </dd>

        <dt className="text-content-muted">{t('certificates.issuerOrg')}</dt>
        <dd className="text-content-primary">{result.issuer_org || '-'}</dd>

        <dt className="text-content-muted">{t('certificates.expiresAt')}</dt>
        <dd className="text-content-primary">
          {result.expires_at ? new Date(result.expires_at).toLocaleDateString() : '-'}
        </dd>

        <dt className="text-content-muted">{t('certificates.keyMatch')}</dt>
        <dd>
          {result.key_match ? (
            <span className="text-green-400">Yes</span>
          ) : (
            <span className="text-yellow-400">No key provided</span>
          )}
        </dd>

        <dt className="text-content-muted">{t('certificates.chainValid')}</dt>
        <dd>
          {result.chain_valid ? (
            <span className="text-green-400">Yes</span>
          ) : (
            <span className="text-yellow-400">Not verified</span>
          )}
        </dd>

        {result.chain_depth > 0 && (
          <>
            <dt className="text-content-muted">{t('certificates.chainDepth')}</dt>
            <dd className="text-content-primary">{result.chain_depth}</dd>
          </>
        )}
      </dl>

      {result.warnings.length > 0 && (
        <div className="flex items-start gap-2 rounded-md border border-yellow-900/50 bg-yellow-900/10 p-3">
          <AlertTriangle className="h-4 w-4 text-yellow-400 mt-0.5 shrink-0" aria-hidden="true" />
          <div className="space-y-1">
            <p className="text-sm font-medium text-yellow-400">{t('certificates.warnings')}</p>
            <ul className="list-disc list-inside text-sm text-yellow-300/80 space-y-0.5">
              {result.warnings.map((w, i) => (
                <li key={i}>{w}</li>
              ))}
            </ul>
          </div>
        </div>
      )}

      {result.errors.length > 0 && (
        <div className="flex items-start gap-2 rounded-md border border-red-900/50 bg-red-900/10 p-3">
          <XCircle className="h-4 w-4 text-red-400 mt-0.5 shrink-0" aria-hidden="true" />
          <div className="space-y-1">
            <p className="text-sm font-medium text-red-400">{t('certificates.errors')}</p>
            <ul className="list-disc list-inside text-sm text-red-300/80 space-y-0.5">
              {result.errors.map((e, i) => (
                <li key={i}>{e}</li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </div>
  )
}
