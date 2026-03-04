import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { importCrowdsecConfig } from '../api/crowdsec'
import { createBackup } from '../api/backups'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { toast } from 'react-hot-toast'

export default function ImportCrowdSec() {
  const { t } = useTranslation()
  const [file, setFile] = useState<File | null>(null)

  const backupMutation = useMutation({
    mutationFn: () => createBackup(),
  })

  const importMutation = useMutation({
    mutationFn: async (file: File) => importCrowdsecConfig(file),
    onSuccess: () => {
      toast.success(t('importCrowdSec.configImported'))
    },
    onError: (e: unknown) => {
      const msg = e instanceof Error ? e.message : String(e)
      toast.error(t('importCrowdSec.importFailed', { error: msg }))
    }
  })

  const handleFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    if (!f) return
    setFile(f)
  }

  const handleImport = async () => {
    if (!file) return
    try {
      toast.loading(t('importCrowdSec.creatingBackup'))
      await backupMutation.mutateAsync()
      toast.dismiss()
      toast.loading(t('importCrowdSec.importing'))
      await importMutation.mutateAsync(file)
      toast.dismiss()
    } catch {
      toast.dismiss()
      // importMutation onError handles toast
    }
  }

  return (
    <div className="p-8">
      <h1 className="text-3xl font-bold text-white mb-6">{t('importCrowdSec.title')}</h1>
      <Card className="p-6">
        <div className="space-y-4">
          <p className="text-sm text-gray-400">{t('importCrowdSec.description')}</p>
          <input type="file" onChange={handleFile} accept=".tar.gz,.zip" data-testid="crowdsec-import-file" />
          <div className="flex gap-2" data-testid="import-progress">
            <Button onClick={() => handleImport()} isLoading={backupMutation.isPending || importMutation.isPending} disabled={!file}>{t('importCrowdSec.import')}</Button>
          </div>
        </div>
      </Card>
    </div>
  )
}
