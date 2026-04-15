import { Download } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Certificate } from '../../api/certificates'
import { useExportCertificate } from '../../hooks/useCertificates'
import { toast } from '../../utils/toast'
import {
  Button,
  Input,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Label,
} from '../ui'

interface CertificateExportDialogProps {
  certificate: Certificate | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

const FORMAT_OPTIONS = [
  { value: 'pem', label: 'exportFormatPem' },
  { value: 'pfx', label: 'exportFormatPfx' },
  { value: 'der', label: 'exportFormatDer' },
] as const

export default function CertificateExportDialog({
  certificate,
  open,
  onOpenChange,
}: CertificateExportDialogProps) {
  const { t } = useTranslation()

  const [format, setFormat] = useState('pem')
  const [includeKey, setIncludeKey] = useState(false)
  const [password, setPassword] = useState('')
  const [pfxPassword, setPfxPassword] = useState('')

  const exportMutation = useExportCertificate()

  function resetForm() {
    setFormat('pem')
    setIncludeKey(false)
    setPassword('')
    setPfxPassword('')
  }

  function handleClose(nextOpen: boolean) {
    if (!nextOpen) resetForm()
    onOpenChange(nextOpen)
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!certificate) return

    exportMutation.mutate(
      {
        uuid: certificate.uuid,
        format,
        includeKey,
        password: includeKey ? password : undefined,
        pfxPassword: format === 'pfx' ? pfxPassword : undefined,
      },
      {
        onSuccess: (blob) => {
          const ext = format === 'pfx' ? 'pfx' : format === 'der' ? 'der' : 'pem'
          const filename = `${certificate.name || 'certificate'}.${ext}`
          const url = URL.createObjectURL(blob)
          const a = document.createElement('a')
          a.href = url
          a.download = filename
          document.body.appendChild(a)
          a.click()
          URL.revokeObjectURL(url)
          a.remove()
          toast.success(t('certificates.exportSuccess'))
          handleClose(false)
        },
        onError: (error: Error) => {
          toast.error(`${t('certificates.exportFailed')}: ${error.message}`)
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent data-testid="certificate-export-dialog" className="max-w-md">
        <DialogHeader>
          <DialogTitle>
            <Download className="inline h-5 w-5 mr-2" aria-hidden="true" />
            {t('certificates.exportTitle')}
          </DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 py-2">
          <div>
            <Label htmlFor="export-format">{t('certificates.exportFormat')}</Label>
            <div className="flex gap-2 mt-1.5" role="radiogroup" aria-label={t('certificates.exportFormat')}>
              {FORMAT_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  type="button"
                  role="radio"
                  aria-checked={format === opt.value}
                  onClick={() => setFormat(opt.value)}
                  className={`px-3 py-1.5 text-sm rounded-md border transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 ${
                    format === opt.value
                      ? 'border-brand-500 bg-brand-500/20 text-brand-400'
                      : 'border-gray-700 text-content-muted hover:border-gray-600'
                  }`}
                >
                  {t(`certificates.${opt.label}`)}
                </button>
              ))}
            </div>
          </div>

          {certificate?.has_key && (
            <div className="flex items-start gap-3">
              <input
                id="include-key"
                type="checkbox"
                checked={includeKey}
                onChange={(e) => setIncludeKey(e.target.checked)}
                className="mt-1 h-4 w-4 rounded border-gray-700 bg-surface-muted text-brand-500 focus:ring-brand-500"
              />
              <div>
                <Label htmlFor="include-key" className="cursor-pointer">
                  {t('certificates.includePrivateKey')}
                </Label>
                {includeKey && (
                  <p className="text-xs text-yellow-400 mt-1">
                    {t('certificates.includePrivateKeyWarning')}
                  </p>
                )}
              </div>
            </div>
          )}

          {includeKey && (
            <Input
              id="export-password"
              label={t('certificates.exportPassword')}
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              aria-required="true"
              autoComplete="current-password"
            />
          )}

          {format === 'pfx' && (
            <Input
              id="pfx-password"
              label={t('certificates.exportPfxPassword')}
              type="password"
              value={pfxPassword}
              onChange={(e) => setPfxPassword(e.target.value)}
              autoComplete="off"
            />
          )}

          <DialogFooter className="pt-4">
            <Button type="button" variant="secondary" onClick={() => handleClose(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              isLoading={exportMutation.isPending}
              data-testid="export-certificate-submit"
            >
              <Download className="h-4 w-4 mr-2" aria-hidden="true" />
              {t('certificates.exportButton')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
