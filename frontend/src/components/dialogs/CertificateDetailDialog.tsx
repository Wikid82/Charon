import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { Certificate } from '../../api/certificates'
import { useCertificateDetail } from '../../hooks/useCertificates'
import CertificateChainViewer from '../CertificateChainViewer'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '../ui'

interface CertificateDetailDialogProps {
  certificate: Certificate | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export default function CertificateDetailDialog({
  certificate,
  open,
  onOpenChange,
}: CertificateDetailDialogProps) {
  const { t } = useTranslation()

  const { detail, isLoading } = useCertificateDetail(
    open && certificate ? certificate.uuid : null,
  )

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        data-testid="certificate-detail-dialog"
        className="max-w-lg max-h-[85vh] overflow-y-auto"
      >
        <DialogHeader>
          <DialogTitle>{t('certificates.detailTitle')}</DialogTitle>
        </DialogHeader>

        {isLoading && (
          <div className="flex justify-center py-8">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-hidden="true" />
          </div>
        )}

        {detail && (
          <div className="space-y-6 py-2">
            <section>
              <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
                <dt className="text-content-muted">{t('certificates.friendlyName')}</dt>
                <dd className="text-content-primary">{detail.name || '-'}</dd>

                <dt className="text-content-muted">{t('certificates.commonName')}</dt>
                <dd className="text-content-primary">{detail.common_name || '-'}</dd>

                <dt className="text-content-muted">{t('certificates.domains')}</dt>
                <dd className="text-content-primary">{detail.domains || '-'}</dd>

                <dt className="text-content-muted">{t('certificates.issuerOrg')}</dt>
                <dd className="text-content-primary">{detail.issuer_org || detail.issuer || '-'}</dd>

                <dt className="text-content-muted">{t('certificates.fingerprint')}</dt>
                <dd className="text-content-primary font-mono text-xs break-all">
                  {detail.fingerprint || '-'}
                </dd>

                <dt className="text-content-muted">{t('certificates.serialNumber')}</dt>
                <dd className="text-content-primary font-mono text-xs break-all">
                  {detail.serial_number || '-'}
                </dd>

                <dt className="text-content-muted">{t('certificates.keyType')}</dt>
                <dd className="text-content-primary">{detail.key_type || '-'}</dd>

                <dt className="text-content-muted">{t('certificates.status')}</dt>
                <dd className="text-content-primary capitalize">{detail.status}</dd>

                <dt className="text-content-muted">{t('certificates.provider')}</dt>
                <dd className="text-content-primary">{detail.provider}</dd>

                <dt className="text-content-muted">{t('certificates.notBefore')}</dt>
                <dd className="text-content-primary">
                  {detail.not_before ? new Date(detail.not_before).toLocaleDateString() : '-'}
                </dd>

                <dt className="text-content-muted">{t('certificates.expiresAt')}</dt>
                <dd className="text-content-primary">
                  {detail.expires_at ? new Date(detail.expires_at).toLocaleDateString() : '-'}
                </dd>

                <dt className="text-content-muted">{t('certificates.autoRenew')}</dt>
                <dd className="text-content-primary">
                  {detail.auto_renew ? t('common.yes') : t('common.no')}
                </dd>

                <dt className="text-content-muted">{t('certificates.createdAt')}</dt>
                <dd className="text-content-primary">
                  {detail.created_at ? new Date(detail.created_at).toLocaleDateString() : '-'}
                </dd>

                <dt className="text-content-muted">{t('certificates.updatedAt')}</dt>
                <dd className="text-content-primary">
                  {detail.updated_at ? new Date(detail.updated_at).toLocaleDateString() : '-'}
                </dd>
              </dl>
            </section>

            <section>
              <h3 className="text-sm font-medium text-content-primary mb-3">
                {t('certificates.assignedHosts')}
              </h3>
              {detail.assigned_hosts?.length > 0 ? (
                <ul className="space-y-1.5">
                  {detail.assigned_hosts.map((host) => (
                    <li
                      key={host.uuid}
                      className="flex items-center justify-between rounded-md border border-gray-700 bg-surface-muted/30 px-3 py-2 text-sm"
                    >
                      <span className="text-content-primary font-medium">{host.name}</span>
                      <span className="text-content-muted text-xs">{host.domain_names}</span>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-sm text-content-muted italic">
                  {t('certificates.noAssignedHosts')}
                </p>
              )}
            </section>

            <section>
              <h3 className="text-sm font-medium text-content-primary mb-3">
                {t('certificates.certificateChain')}
              </h3>
              <CertificateChainViewer chain={detail.chain || []} />
            </section>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
