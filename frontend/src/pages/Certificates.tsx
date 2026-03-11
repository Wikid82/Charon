import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, ShieldCheck } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { uploadCertificate } from '../api/certificates'
import CertificateList from '../components/CertificateList'
import { PageShell } from '../components/layout/PageShell'
import {
  Button,
  Input,
  Alert,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Label,
} from '../components/ui'
import { toast } from '../utils/toast'

export default function Certificates() {
  const { t } = useTranslation()
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [name, setName] = useState('')
  const [certFile, setCertFile] = useState<File | null>(null)
  const [keyFile, setKeyFile] = useState<File | null>(null)
  const queryClient = useQueryClient()

  const uploadMutation = useMutation({
    mutationFn: async () => {
      if (!certFile || !keyFile) throw new Error('Files required')
      await uploadCertificate(name, certFile, keyFile)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['certificates'] })
      setIsModalOpen(false)
      setName('')
      setCertFile(null)
      setKeyFile(null)
      toast.success(t('certificates.uploadSuccess'))
    },
    onError: (error: Error) => {
      toast.error(`${t('certificates.uploadFailed')}: ${error.message}`)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    uploadMutation.mutate()
  }

  // Header actions
  const headerActions = (
    <Button onClick={() => setIsModalOpen(true)} data-testid="add-certificate-btn">
      <Plus className="w-4 h-4 mr-2" />
      {t('certificates.addCertificate')}
    </Button>
  )

  return (
    <PageShell
      title={t('certificates.title')}
      description={t('certificates.description')}
      actions={headerActions}
    >
      <Alert variant="info" icon={ShieldCheck}>
        <strong>{t('certificates.note')}:</strong> {t('certificates.noteText')}
      </Alert>

      <CertificateList />

      {/* Upload Certificate Dialog */}
      <Dialog open={isModalOpen} onOpenChange={setIsModalOpen}>
        <DialogContent data-testid="certificate-upload-dialog">
          <DialogHeader>
            <DialogTitle>{t('certificates.uploadCertificate')}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit} className="space-y-4 py-4">
            <Input
              id="certificate-name"
              label={t('certificates.friendlyName')}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. My Custom Cert"
              required
            />
            <div>
              <Label htmlFor="cert-file">{t('certificates.certificatePem')}</Label>
              <input
                id="cert-file"
                data-testid="certificate-file-input"
                type="file"
                accept=".pem,.crt,.cer"
                onChange={(e) => setCertFile(e.target.files?.[0] || null)}
                className="mt-1.5 block w-full text-sm text-content-secondary file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-brand-500 file:text-white hover:file:bg-brand-600 cursor-pointer"
                required
              />
            </div>
            <div>
              <Label htmlFor="key-file">{t('certificates.privateKeyPem')}</Label>
              <input
                id="key-file"
                data-testid="certificate-key-input"
                type="file"
                accept=".pem,.key"
                onChange={(e) => setKeyFile(e.target.files?.[0] || null)}
                className="mt-1.5 block w-full text-sm text-content-secondary file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-brand-500 file:text-white hover:file:bg-brand-600 cursor-pointer"
                required
              />
            </div>
            <DialogFooter className="pt-4">
              <Button type="button" variant="secondary" onClick={() => setIsModalOpen(false)}>
                {t('common.cancel')}
              </Button>
              <Button type="submit" isLoading={uploadMutation.isPending}>
                {t('common.upload')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </PageShell>
  )
}
