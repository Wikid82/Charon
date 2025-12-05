import { useState } from 'react'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { getSecurityStatus } from '../api/security'
import { exportCrowdsecConfig, importCrowdsecConfig, listCrowdsecFiles, readCrowdsecFile, writeCrowdsecFile, listCrowdsecDecisions, banIP, unbanIP, CrowdSecDecision } from '../api/crowdsec'
import { createBackup } from '../api/backups'
import { updateSetting } from '../api/settings'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from '../utils/toast'
import { ConfigReloadOverlay } from '../components/LoadingStates'
import { Shield, ShieldOff, Trash2 } from 'lucide-react'

export default function CrowdSecConfig() {
  const { data: status, isLoading, error } = useQuery({ queryKey: ['security-status'], queryFn: getSecurityStatus })
  const [file, setFile] = useState<File | null>(null)
  const [selectedPath, setSelectedPath] = useState<string | null>(null)
  const [fileContent, setFileContent] = useState<string | null>(null)
  const [showBanModal, setShowBanModal] = useState(false)
  const [banForm, setBanForm] = useState({ ip: '', duration: '24h', reason: '' })
  const [confirmUnban, setConfirmUnban] = useState<CrowdSecDecision | null>(null)
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

  // Banned IPs queries and mutations
  const decisionsQuery = useQuery({
    queryKey: ['crowdsec-decisions'],
    queryFn: listCrowdsecDecisions,
    enabled: status?.crowdsec?.mode !== 'disabled',
  })

  const banMutation = useMutation({
    mutationFn: () => banIP(banForm.ip, banForm.duration, banForm.reason),
    onSuccess: () => {
      toast.success(`IP ${banForm.ip} has been banned`)
      queryClient.invalidateQueries({ queryKey: ['crowdsec-decisions'] })
      setShowBanModal(false)
      setBanForm({ ip: '', duration: '24h', reason: '' })
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : 'Failed to ban IP')
    },
  })

  const unbanMutation = useMutation({
    mutationFn: (ip: string) => unbanIP(ip),
    onSuccess: (_, ip) => {
      toast.success(`IP ${ip} has been unbanned`)
      queryClient.invalidateQueries({ queryKey: ['crowdsec-decisions'] })
      setConfirmUnban(null)
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : 'Failed to unban IP')
    },
  })

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

  // Determine if any operation is in progress
  const isApplyingConfig =
    importMutation.isPending ||
    writeMutation.isPending ||
    updateModeMutation.isPending ||
    backupMutation.isPending ||
    banMutation.isPending ||
    unbanMutation.isPending

  // Determine contextual message
  const getMessage = () => {
    if (importMutation.isPending) {
      return { message: 'Summoning the guardian...', submessage: 'Importing CrowdSec configuration' }
    }
    if (writeMutation.isPending) {
      return { message: 'Guardian inscribes...', submessage: 'Saving configuration file' }
    }
    if (updateModeMutation.isPending) {
      return { message: 'Three heads turn...', submessage: 'CrowdSec mode updating' }
    }
    if (banMutation.isPending) {
      return { message: 'Guardian raises shield...', submessage: 'Banning IP address' }
    }
    if (unbanMutation.isPending) {
      return { message: 'Guardian lowers shield...', submessage: 'Unbanning IP address' }
    }
    return { message: 'Strengthening the guard...', submessage: 'Configuration in progress' }
  }

  const { message, submessage } = getMessage()

  if (isLoading) return <div className="p-8 text-center text-white">Loading CrowdSec configuration...</div>
  if (error) return <div className="p-8 text-center text-red-500">Failed to load security status: {(error as Error).message}</div>
  if (!status) return <div className="p-8 text-center text-gray-400">No security status available</div>

  return (
    <>
      {isApplyingConfig && (
        <ConfigReloadOverlay
          message={message}
          submessage={submessage}
          type="cerberus"
        />
      )}
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

      {/* Banned IPs Section */}
      <Card>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Shield className="h-5 w-5 text-red-400" />
              <h3 className="text-md font-semibold">Banned IPs</h3>
            </div>
            <Button
              onClick={() => setShowBanModal(true)}
              disabled={status.crowdsec.mode === 'disabled'}
              size="sm"
            >
              <ShieldOff className="h-4 w-4 mr-1" />
              Ban IP
            </Button>
          </div>

          {status.crowdsec.mode === 'disabled' ? (
            <p className="text-sm text-gray-500">Enable CrowdSec to manage banned IPs</p>
          ) : decisionsQuery.isLoading ? (
            <p className="text-sm text-gray-400">Loading banned IPs...</p>
          ) : decisionsQuery.error ? (
            <p className="text-sm text-red-400">Failed to load banned IPs</p>
          ) : !decisionsQuery.data?.decisions?.length ? (
            <p className="text-sm text-gray-500">No banned IPs</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-700">
                    <th className="text-left py-2 px-3 text-gray-400 font-medium">IP</th>
                    <th className="text-left py-2 px-3 text-gray-400 font-medium">Reason</th>
                    <th className="text-left py-2 px-3 text-gray-400 font-medium">Duration</th>
                    <th className="text-left py-2 px-3 text-gray-400 font-medium">Banned At</th>
                    <th className="text-left py-2 px-3 text-gray-400 font-medium">Source</th>
                    <th className="text-right py-2 px-3 text-gray-400 font-medium">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {decisionsQuery.data.decisions.map((decision) => (
                    <tr key={decision.id} className="border-b border-gray-800 hover:bg-gray-800/50">
                      <td className="py-2 px-3 font-mono text-white">{decision.ip}</td>
                      <td className="py-2 px-3 text-gray-300">{decision.reason || '-'}</td>
                      <td className="py-2 px-3 text-gray-300">{decision.duration}</td>
                      <td className="py-2 px-3 text-gray-300">
                        {decision.created_at ? new Date(decision.created_at).toLocaleString() : '-'}
                      </td>
                      <td className="py-2 px-3 text-gray-300">{decision.source || 'manual'}</td>
                      <td className="py-2 px-3 text-right">
                        <Button
                          variant="danger"
                          size="sm"
                          onClick={() => setConfirmUnban(decision)}
                        >
                          <Trash2 className="h-3 w-3 mr-1" />
                          Unban
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </Card>
      </div>

      {/* Ban IP Modal */}
      {showBanModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/60" onClick={() => setShowBanModal(false)} />
          <div className="relative bg-dark-card rounded-lg p-6 w-[480px] max-w-full">
            <h3 className="text-xl font-semibold text-white mb-4 flex items-center gap-2">
              <ShieldOff className="h-5 w-5 text-red-400" />
              Ban IP Address
            </h3>
            <div className="space-y-4">
              <Input
                label="IP Address"
                placeholder="192.168.1.100"
                value={banForm.ip}
                onChange={(e) => setBanForm({ ...banForm, ip: e.target.value })}
              />
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-1.5">Duration</label>
                <select
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white"
                  value={banForm.duration}
                  onChange={(e) => setBanForm({ ...banForm, duration: e.target.value })}
                >
                  <option value="1h">1 hour</option>
                  <option value="4h">4 hours</option>
                  <option value="24h">24 hours</option>
                  <option value="7d">7 days</option>
                  <option value="30d">30 days</option>
                  <option value="permanent">Permanent</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-1.5">Reason</label>
                <textarea
                  placeholder="Reason for banning this IP..."
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  rows={3}
                  value={banForm.reason}
                  onChange={(e) => setBanForm({ ...banForm, reason: e.target.value })}
                />
              </div>
            </div>
            <div className="flex gap-3 justify-end mt-6">
              <Button variant="secondary" onClick={() => setShowBanModal(false)}>
                Cancel
              </Button>
              <Button
                variant="danger"
                onClick={() => banMutation.mutate()}
                isLoading={banMutation.isPending}
                disabled={!banForm.ip.trim()}
              >
                Ban IP
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Unban Confirmation Modal */}
      {confirmUnban && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/60" onClick={() => setConfirmUnban(null)} />
          <div className="relative bg-dark-card rounded-lg p-6 w-[400px] max-w-full">
            <h3 className="text-xl font-semibold text-white mb-4">Confirm Unban</h3>
            <p className="text-gray-300 mb-6">
              Are you sure you want to unban <span className="font-mono text-white">{confirmUnban.ip}</span>?
            </p>
            <div className="flex gap-3 justify-end">
              <Button variant="secondary" onClick={() => setConfirmUnban(null)}>
                Cancel
              </Button>
              <Button
                variant="danger"
                onClick={() => unbanMutation.mutate(confirmUnban.ip)}
                isLoading={unbanMutation.isPending}
              >
                Unban
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
