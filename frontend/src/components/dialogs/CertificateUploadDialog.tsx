import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { ValidationResult } from '../../api/certificates'
import CertificateValidationPreview from '../CertificateValidationPreview'
import {
  Button,
  Input,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../ui'
import { FileDropZone } from '../ui/FileDropZone'

import { useUploadCertificate, useValidateCertificate } from '../../hooks/useCertificates'
import { toast } from '../../utils/toast'

interface CertificateUploadDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

function detectFormat(file: File | null): string | null {
  if (!file) return null
  const ext = file.name.toLowerCase().split('.').pop()
  if (ext === 'pfx' || ext === 'p12') return 'PFX/PKCS#12'
  if (ext === 'pem' || ext === 'crt' || ext === 'cer') return 'PEM'
  if (ext === 'der') return 'DER'
  if (ext === 'key') return 'KEY'
  return null
}

export default function CertificateUploadDialog({
  open,
  onOpenChange,
}: CertificateUploadDialogProps) {
  const { t } = useTranslation()

  const [name, setName] = useState('')
  const [certFile, setCertFile] = useState<File | null>(null)
  const [keyFile, setKeyFile] = useState<File | null>(null)
  const [chainFile, setChainFile] = useState<File | null>(null)
  const [validationResult, setValidationResult] = useState<ValidationResult | null>(null)

  const uploadMutation = useUploadCertificate()
  const validateMutation = useValidateCertificate()

  const certFormat = detectFormat(certFile)
  const isPfx = certFormat === 'PFX/PKCS#12'

  function resetForm() {
    setName('')
    setCertFile(null)
    setKeyFile(null)
    setChainFile(null)
    setValidationResult(null)
  }

  function handleClose(nextOpen: boolean) {
    if (!nextOpen) resetForm()
    onOpenChange(nextOpen)
  }

  function handleValidate() {
    if (!certFile) return
    validateMutation.mutate(
      { certFile, keyFile: keyFile ?? undefined, chainFile: chainFile ?? undefined },
      {
        onSuccess: (result) => {
          setValidationResult(result)
        },
        onError: (error: Error) => {
          toast.error(error.message)
        },
      },
    )
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!certFile) return

    uploadMutation.mutate(
      {
        name,
        certFile,
        keyFile: keyFile ?? undefined,
        chainFile: chainFile ?? undefined,
      },
      {
        onSuccess: () => {
          toast.success(t('certificates.uploadSuccess'))
          handleClose(false)
        },
        onError: (error: Error) => {
          toast.error(`${t('certificates.uploadFailed')}: ${error.message}`)
        },
      },
    )
  }

  const canValidate = !!certFile && !validateMutation.isPending
  const needsKeyFile = !!certFile && !isPfx && !keyFile
  const canSubmit = !!certFile && !!name.trim() && !needsKeyFile

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent data-testid="certificate-upload-dialog" className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('certificates.uploadCertificate')}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4 py-2">
          <Input
            id="certificate-name"
            label={t('certificates.friendlyName')}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. My Custom Cert"
            required
            aria-required="true"
          />

          <FileDropZone
            id="cert-file"
            label={t('certificates.certificateFile')}
            accept=".pem,.crt,.cer,.pfx,.p12,.der"
            file={certFile}
            onFileChange={(f) => {
              setCertFile(f)
              setValidationResult(null)
            }}
            required
            formatBadge={certFormat}
          />

          {isPfx && (
            <p className="text-xs text-content-muted italic">
              {t('certificates.pfxDetected')}
            </p>
          )}

          {!isPfx && (
            <>
              <FileDropZone
                id="key-file"
                required={!!certFile}
                label={t('certificates.privateKeyFile')}
                accept=".pem,.key"
                file={keyFile}
                onFileChange={(f) => {
                  setKeyFile(f)
                  setValidationResult(null)
                }}
              />

              <FileDropZone
                id="chain-file"
                label={t('certificates.chainFile')}
                accept=".pem,.crt,.cer"
                file={chainFile}
                onFileChange={(f) => {
                  setChainFile(f)
                  setValidationResult(null)
                }}
              />
            </>
          )}

          {needsKeyFile && (
            <p role="alert" className="text-xs text-red-500">
              {t('certificates.keyFileRequired')}
            </p>
          )}

          {certFile && !validationResult && (
            <Button
              type="button"
              variant="secondary"
              onClick={handleValidate}
              disabled={!canValidate}
              isLoading={validateMutation.isPending}
              data-testid="validate-certificate-btn"
            >
              {validateMutation.isPending
                ? t('certificates.validating')
                : t('certificates.validate')}
            </Button>
          )}

          {validationResult && (
            <CertificateValidationPreview result={validationResult} />
          )}

          <DialogFooter className="pt-4">
            <Button type="button" variant="secondary" onClick={() => handleClose(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              disabled={!canSubmit}
              isLoading={uploadMutation.isPending}
              data-testid="upload-certificate-submit"
            >
              {t('certificates.uploadAndSave')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
