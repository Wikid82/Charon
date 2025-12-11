import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { importCrowdsecConfig } from '../api/crowdsec'
import { createBackup } from '../api/backups'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { toast } from 'react-hot-toast'

export default function ImportCrowdSec() {
  const [file, setFile] = useState<File | null>(null)

  const backupMutation = useMutation({
    mutationFn: () => createBackup(),
  })

  const importMutation = useMutation({
    mutationFn: async (file: File) => importCrowdsecConfig(file),
    onSuccess: () => {
      toast.success('CrowdSec config imported')
    },
    onError: (e: unknown) => {
      const msg = e instanceof Error ? e.message : String(e)
      toast.error(`Import failed: ${msg}`)
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
      toast.loading('Creating backup...')
      await backupMutation.mutateAsync()
      toast.dismiss()
      toast.loading('Importing CrowdSec...')
      await importMutation.mutateAsync(file)
      toast.dismiss()
    } catch {
      toast.dismiss()
      // importMutation onError handles toast
    }
  }

  return (
    <div className="p-8">
      <h1 className="text-3xl font-bold text-white mb-6">CrowdSec Configuration Packages</h1>
      <Card className="p-6">
        <div className="space-y-4">
          <p className="text-sm text-gray-400">Upload a tar.gz or zip package. A backup is created before importing so you can roll back if needed. Export the current package from the Cerberus dashboard or CrowdSec config page.</p>
          <input type="file" onChange={handleFile} accept=".tar.gz,.zip" data-testid="crowdsec-import-file" />
          <div className="flex gap-2">
            <Button onClick={() => handleImport()} isLoading={backupMutation.isPending || importMutation.isPending} disabled={!file}>Import</Button>
          </div>
        </div>
      </Card>
    </div>
  )
}
