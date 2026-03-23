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

interface BulkDeleteCertificateDialogProps {
  certificates: Certificate[]
  open: boolean
  onConfirm: () => void
  onCancel: () => void
  isDeleting: boolean
}

function providerLabel(cert: Certificate): string {
  if (cert.provider === 'letsencrypt-staging') return 'Staging'
  if (cert.provider === 'custom') return 'Custom'
  if (cert.status === 'expired') return 'Expired LE'
  if (cert.status === 'expiring') return 'Expiring LE'
  return cert.provider
}

export default function BulkDeleteCertificateDialog({
  certificates,
  open,
  onConfirm,
  onCancel,
  isDeleting,
}: BulkDeleteCertificateDialogProps) {
  const { t } = useTranslation()

  if (certificates.length === 0) return null

  return (
    <Dialog open={open} onOpenChange={(isOpen) => { if (!isOpen) onCancel() }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('certificates.bulkDeleteTitle', { count: certificates.length })}</DialogTitle>
          <DialogDescription>
            {t('certificates.bulkDeleteDescription', { count: certificates.length })}
          </DialogDescription>
        </DialogHeader>

        <div className="px-6 space-y-4">
          <div className="flex items-start gap-3 rounded-lg border border-red-900/50 bg-red-900/10 p-4">
            <AlertTriangle className="h-5 w-5 shrink-0 text-red-400 mt-0.5" />
            <p className="text-sm text-gray-300">
              {t('certificates.bulkDeleteConfirm')}
            </p>
          </div>

          <ul
            aria-label="Certificates to be deleted"
            className="max-h-48 overflow-y-auto rounded-lg border border-gray-800 divide-y divide-gray-800"
          >
            {certificates.map((cert) => (
              <li
                key={cert.id ?? cert.domain}
                className="flex items-center justify-between px-4 py-2"
              >
                <span className="text-sm text-white">{cert.name || cert.domain}</span>
                <span className="text-xs text-gray-500">{providerLabel(cert)}</span>
              </li>
            ))}
          </ul>
        </div>

        <DialogFooter>
          <Button variant="secondary" onClick={onCancel} disabled={isDeleting}>
            {t('common.cancel')}
          </Button>
          <Button variant="danger" onClick={onConfirm} isLoading={isDeleting}>
            {t('certificates.bulkDeleteButton', { count: certificates.length })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
