import { Plus, ShieldCheck } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import CertificateList from '../components/CertificateList'
import CertificateUploadDialog from '../components/dialogs/CertificateUploadDialog'
import { PageShell } from '../components/layout/PageShell'
import { Button, Alert } from '../components/ui'

export default function Certificates() {
  const { t } = useTranslation()
  const [isUploadOpen, setIsUploadOpen] = useState(false)

  const headerActions = (
    <Button onClick={() => setIsUploadOpen(true)} data-testid="add-certificate-btn">
      <Plus className="w-4 h-4 mr-2" aria-hidden="true" />
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

      <CertificateUploadDialog open={isUploadOpen} onOpenChange={setIsUploadOpen} />
    </PageShell>
  )
}
