import { useState } from 'react'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { getSecurityStatus } from '../api/security'
import { exportCrowdsecConfig, importCrowdsecConfig, listCrowdsecFiles, readCrowdsecFile, writeCrowdsecFile } from '../api/crowdsec'
import { createBackup } from '../api/backups'
import { updateSetting } from '../api/settings'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from '../utils/toast'

export default function CrowdSecConfig() {
  const { data: status } = useQuery({ queryKey: ['security-status'], queryFn: getSecurityStatus })
  const [file, setFile] = useState<File | null>(null)
  const [selectedPath, setSelectedPath] = useState<string | null>(null)
  const [fileContent, setFileContent] = useState<string | null>(null)
  const queryClient = useQueryClient()

  const backupMutation = useMutation({ mutationFn: () => createBackup() })
  const importMutation = useMutation({
    mutationFn: async (file: File) => {
      return await importCrowdsecConfig(file)
    },
    onSuccess: () => {
      toast.success('CrowdSec config imported (backup created)')
      queryClient.invalidateQueries({ queryKey: ['security-status'] })
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : 'Failed to import')
    }
  })

  const listMutation = useQuery({ queryKey: ['crowdsec-files'], queryFn: listCrowdsecFiles })
  const readMutation = useMutation({ mutationFn: (path: string) => readCrowdsecFile(path), onSuccess: (data) => setFileContent(data.content) })
  const writeMutation = useMutation({ mutationFn: async ({ path, content }: { path: string; content: string }) => writeCrowdsecFile(path, content), onSuccess: () => { toast.success('File saved'); queryClient.invalidateQueries({ queryKey: ['crowdsec-files'] }) } })
  const updateModeMutation = useMutation({ mutationFn: async (mode: string) => updateSetting('security.crowdsec.mode', mode, 'security', 'string'), onSuccess: () => queryClient.invalidateQueries({ queryKey: ['security-status'] }) })

  const handleExport = async () => {
    try {
      const blob = await exportCrowdsecConfig()
      const url = window.URL.createObjectURL(new Blob([blob]))
      const a = document.createElement('a')
      a.href = url
      a.download = `crowdsec-config-${new Date().toISOString().slice(0,19).replace(/[:T]/g, '-')}.tar.gz`
      document.body.appendChild(a)
      a.click()
      a.remove()
      window.URL.revokeObjectURL(url)
      toast.success('CrowdSec configuration exported')
    } catch {
      toast.error('Failed to export CrowdSec configuration')
    }
  }

  const handleImport = async () => {
    if (!file) return
    try {
      await backupMutation.mutateAsync()
      await importMutation.mutateAsync(file)
      setFile(null)
    } catch {
      // handled in onError
    }
  }

  const handleReadFile = async (path: string) => {
    setSelectedPath(path)
    await readMutation.mutateAsync(path)
  }

  const handleSaveFile = async () => {
    if (!selectedPath || fileContent === null) return
    try {
      await backupMutation.mutateAsync()
      await writeMutation.mutateAsync({ path: selectedPath, content: fileContent })
    } catch {
      // handled
    }
  }

  const handleModeChange = async (mode: string) => {
    updateModeMutation.mutate(mode)
    toast.success('CrowdSec mode saved (restart may be required)')
  }

  if (!status) return <div className="p-8 text-center">Loading...</div>

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">CrowdSec Configuration</h1>
      <Card>
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold">Mode</h2>
            <div className="flex items-center gap-4">
              <label className="text-sm text-gray-400">Mode:</label>
              <div className="flex items-center gap-3">
                <select value={status.crowdsec.mode} onChange={(e) => handleModeChange(e.target.value)} className="bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white">
                  <option value="disabled">Disabled</option>
                  <option value="local">Local</option>
                </select>
              </div>
            </div>
            {status.crowdsec.mode === 'disabled' && (
              <p className="text-xs text-yellow-500">CrowdSec is disabled</p>
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button variant="secondary" onClick={handleExport}>Export</Button>
          </div>
        </div>
      </Card>

      <Card>
        <div className="space-y-4">
          <h3 className="text-md font-semibold">Import Configuration</h3>
          <input type="file" onChange={(e) => setFile(e.target.files?.[0] ?? null)} data-testid="import-file" accept=".tar.gz,.zip" />
          <div className="flex gap-2">
            <Button onClick={handleImport} disabled={!file || importMutation.isPending} data-testid="import-btn">
              {importMutation.isPending ? 'Importing...' : 'Import'}
            </Button>
          </div>
        </div>
      </Card>

      <Card>
        <div className="space-y-4">
          <h3 className="text-md font-semibold">Edit Configuration Files</h3>
          <div className="flex items-center gap-2">
            <select className="bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white" value={selectedPath ?? ''} onChange={(e) => handleReadFile(e.target.value)}>
              <option value="">Select a file...</option>
              {listMutation.data?.files.map((f) => (
                <option value={f} key={f}>{f}</option>
              ))}
            </select>
            <Button variant="secondary" onClick={() => listMutation.refetch()}>Refresh</Button>
          </div>
          <textarea value={fileContent ?? ''} onChange={(e) => setFileContent(e.target.value)} rows={12} className="w-full bg-gray-900 border border-gray-700 rounded-lg p-3 text-white" />
          <div className="flex gap-2">
            <Button onClick={handleSaveFile} isLoading={writeMutation.isPending || backupMutation.isPending}>Save</Button>
            <Button variant="secondary" onClick={() => { setSelectedPath(null); setFileContent(null) }}>Close</Button>
          </div>
        </div>
      </Card>
    </div>
  )
}
