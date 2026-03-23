import { AlertTriangle } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '../ui/Button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/Dialog'

import type { Certificate } from '../../api/certificates'

interface DeleteCertificateDialogProps {
  certificate: Certificate | null
  open: boolean
  onConfirm: () => void
  onCancel: () => void
  isDeleting: boolean
}

function getWarningKey(cert: Certificate): string {
  if (cert.status === 'expired') return 'certificates.deleteConfirmExpired'
  if (cert.status === 'expiring') return 'certificates.deleteConfirmExpiring'
  if (cert.provider === 'letsencrypt-staging') return 'certificates.deleteConfirmStaging'
  return 'certificates.deleteConfirmCustom'
}

export default function DeleteCertificateDialog({
  certificate,
  open,
  onConfirm,
  onCancel,
  isDeleting,
}: DeleteCertificateDialogProps) {
  const { t } = useTranslation()

  if (!certificate) return null

  return (
    <Dialog open={open} onOpenChange={(isOpen) => { if (!isOpen) onCancel() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('certificates.deleteTitle')}</DialogTitle>
          <DialogDescription>
            {certificate.name || certificate.domain}
          </DialogDescription>
        </DialogHeader>

        <div className="px-6 space-y-4">
          <div className="flex items-start gap-3 rounded-lg border border-red-900/50 bg-red-900/10 p-4">
            <AlertTriangle className="h-5 w-5 shrink-0 text-red-400 mt-0.5" />
            <p className="text-sm text-gray-300">
              {t(getWarningKey(certificate))}
            </p>
          </div>

          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
            <dt className="text-gray-500">{t('certificates.domain')}</dt>
            <dd className="text-white">{certificate.domain}</dd>
            <dt className="text-gray-500">{t('certificates.status')}</dt>
            <dd className="text-white capitalize">{certificate.status}</dd>
            <dt className="text-gray-500">{t('certificates.provider')}</dt>
            <dd className="text-white">{certificate.provider}</dd>
          </dl>
        </div>

        <DialogFooter>
          <Button variant="secondary" onClick={onCancel} disabled={isDeleting}>
            {t('common.cancel')}
          </Button>
          <Button variant="danger" onClick={onConfirm} isLoading={isDeleting}>
            {t('certificates.deleteButton')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
