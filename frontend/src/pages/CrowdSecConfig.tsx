import { useEffect, useMemo, useState } from 'react'
import { isAxiosError } from 'axios'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { Switch } from '../components/ui/Switch'
import { getSecurityStatus } from '../api/security'
import { exportCrowdsecConfig, importCrowdsecConfig, listCrowdsecFiles, readCrowdsecFile, writeCrowdsecFile, listCrowdsecDecisions, banIP, unbanIP, CrowdSecDecision } from '../api/crowdsec'
import { listCrowdsecPresets, pullCrowdsecPreset, applyCrowdsecPreset, getCrowdsecPresetCache } from '../api/presets'
import { createBackup } from '../api/backups'
import { updateSetting } from '../api/settings'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from '../utils/toast'
import { ConfigReloadOverlay } from '../components/LoadingStates'
import { Shield, ShieldOff, Trash2 } from 'lucide-react'
import { buildCrowdsecExportFilename, downloadCrowdsecExport, promptCrowdsecFilename } from '../utils/crowdsecExport'
import { CROWDSEC_PRESETS, CrowdsecPreset } from '../data/crowdsecPresets'

export default function CrowdSecConfig() {
  const { data: status, isLoading, error } = useQuery({ queryKey: ['security-status'], queryFn: getSecurityStatus })
  const [file, setFile] = useState<File | null>(null)
  const [selectedPath, setSelectedPath] = useState<string | null>(null)
  const [fileContent, setFileContent] = useState<string | null>(null)
  const [selectedPresetSlug, setSelectedPresetSlug] = useState<string>('')
  const [showBanModal, setShowBanModal] = useState(false)
  const [banForm, setBanForm] = useState({ ip: '', duration: '24h', reason: '' })
  const [confirmUnban, setConfirmUnban] = useState<CrowdSecDecision | null>(null)
  const [isApplyingPreset, setIsApplyingPreset] = useState(false)
  const [presetPreview, setPresetPreview] = useState<string>('')
  const [presetMeta, setPresetMeta] = useState<{ cacheKey?: string; etag?: string; retrievedAt?: string; source?: string } | null>(null)
  const [presetStatusMessage, setPresetStatusMessage] = useState<string | null>(null)
  const [hubUnavailable, setHubUnavailable] = useState(false)
  const [validationError, setValidationError] = useState<string | null>(null)
  const [applyInfo, setApplyInfo] = useState<{ status?: string; backup?: string; reloadHint?: boolean; usedCscli?: boolean; cacheKey?: string } | null>(null)
  const queryClient = useQueryClient()
  const isLocalMode = !!status && status.crowdsec?.mode !== 'disabled'

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
  const updateModeMutation = useMutation({
    mutationFn: async (mode: string) => updateSetting('security.crowdsec.mode', mode, 'security', 'string'),
    onSuccess: (_data, mode) => {
      queryClient.invalidateQueries({ queryKey: ['security-status'] })
      toast.success(mode === 'disabled' ? 'CrowdSec disabled' : 'CrowdSec set to Local mode')
    },
    onError: (err: unknown) => {
      const msg = err instanceof Error ? err.message : 'Failed to update mode'
      toast.error(msg)
    },
  })

  const presetsQuery = useQuery({
    queryKey: ['crowdsec-presets'],
    queryFn: listCrowdsecPresets,
    enabled: !!status?.crowdsec,
    retry: false,
  })

  type PresetCatalogEntry = CrowdsecPreset & {
    requiresHub: boolean
    available?: boolean
    cached?: boolean
    cacheKey?: string
    etag?: string
    retrievedAt?: string
    source?: string
  }

  const presetCatalog: PresetCatalogEntry[] = useMemo(() => {
    const remotePresets = presetsQuery.data?.presets
    const localMap = new Map(CROWDSEC_PRESETS.map((preset) => [preset.slug, preset]))
    if (remotePresets?.length) {
      return remotePresets.map((preset) => {
        const local = localMap.get(preset.slug)
        return {
          slug: preset.slug,
          title: preset.title || local?.title || preset.slug,
          description: local?.description || preset.summary,
          content: local?.content || '',
          tags: local?.tags || preset.tags,
          warning: local?.warning,
          requiresHub: Boolean(preset.requires_hub),
          available: preset.available,
          cached: preset.cached,
          cacheKey: preset.cache_key,
          etag: preset.etag,
          retrievedAt: preset.retrieved_at,
          source: preset.source,
        }
      })
    }
    return CROWDSEC_PRESETS.map((preset) => ({ ...preset, requiresHub: false, available: true, cached: false, source: 'charon-curated' }))
  }, [presetsQuery.data])

  useEffect(() => {
    if (!presetCatalog.length) return
    if (!selectedPresetSlug || !presetCatalog.some((preset) => preset.slug === selectedPresetSlug)) {
      setSelectedPresetSlug(presetCatalog[0].slug)
    }
  }, [presetCatalog, selectedPresetSlug])

  const selectedPreset = presetCatalog.find((preset) => preset.slug === selectedPresetSlug)
  const selectedPresetRequiresHub = selectedPreset?.requiresHub ?? false

  const pullPresetMutation = useMutation({
    mutationFn: (slug: string) => pullCrowdsecPreset(slug),
    onSuccess: (data) => {
      setPresetPreview(data.preview)
      setPresetMeta({ cacheKey: data.cache_key, etag: data.etag, retrievedAt: data.retrieved_at, source: data.source })
      setPresetStatusMessage('Preview fetched from hub')
      setHubUnavailable(false)
      setValidationError(null)
    },
    onError: (err: unknown) => {
      setPresetStatusMessage(null)
      if (isAxiosError(err)) {
        if (err.response?.status === 400) {
          setValidationError(err.response.data?.error || 'Preset slug is invalid')
          return
        }
        if (err.response?.status === 503) {
          setHubUnavailable(true)
          setPresetStatusMessage('CrowdSec hub unavailable. Retry or load cached copy.')
          return
        }
        setPresetStatusMessage(err.response?.data?.error || 'Failed to pull preset preview')
      } else {
        setPresetStatusMessage('Failed to pull preset preview')
      }
    },
  })

  useEffect(() => {
    if (!selectedPreset) return
    setValidationError(null)
    setPresetStatusMessage(null)
    setApplyInfo(null)
    setPresetMeta({
      cacheKey: selectedPreset.cacheKey,
      etag: selectedPreset.etag,
      retrievedAt: selectedPreset.retrievedAt,
      source: selectedPreset.source,
    })
    setPresetPreview(selectedPreset.content || '')
    pullPresetMutation.mutate(selectedPreset.slug)
  }, [selectedPreset?.slug])

  const loadCachedPreview = async () => {
    if (!selectedPreset) return
    try {
      const cached = await getCrowdsecPresetCache(selectedPreset.slug)
      setPresetPreview(cached.preview)
      setPresetMeta({ cacheKey: cached.cache_key, etag: cached.etag, retrievedAt: selectedPreset.retrievedAt, source: selectedPreset.source })
      setPresetStatusMessage('Loaded cached preview')
      setHubUnavailable(false)
    } catch (err) {
      const msg = isAxiosError(err) ? err.response?.data?.error || err.message : 'Failed to load cached preview'
      toast.error(msg)
    }
  }

  // Banned IPs queries and mutations
  const decisionsQuery = useQuery({
    queryKey: ['crowdsec-decisions'],
    queryFn: listCrowdsecDecisions,
    enabled: isLocalMode,
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
    const defaultName = buildCrowdsecExportFilename()
    const filename = promptCrowdsecFilename(defaultName)
    if (!filename) return

    try {
      const blob = await exportCrowdsecConfig()
      downloadCrowdsecExport(blob, filename)
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

  const handleModeToggle = (nextEnabled: boolean) => {
    const mode = nextEnabled ? 'local' : 'disabled'
    updateModeMutation.mutate(mode)
  }

  const applyPresetLocally = async (reason?: string) => {
    if (!selectedPreset) {
      toast.error('Select a preset to apply')
      return
    }

    const targetPath = selectedPath ?? listMutation.data?.files?.[0]
    if (!targetPath) {
      toast.error('Select a configuration file to apply the preset')
      return
    }

    const content = presetPreview || selectedPreset.content
    if (!content) {
      toast.error('Preset preview is unavailable; retry pulling before applying')
      return
    }

    try {
      await backupMutation.mutateAsync()
      await writeCrowdsecFile(targetPath, content)
      queryClient.invalidateQueries({ queryKey: ['crowdsec-files'] })
      setSelectedPath(targetPath)
      setFileContent(content)
      setApplyInfo({ status: 'applied-locally', cacheKey: presetMeta?.cacheKey })
      toast.success(reason ? `${reason}: preset applied locally` : 'Preset applied locally (backup created)')
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to apply preset locally'
      toast.error(msg)
    }
  }

  const handleApplyPreset = async () => {
    if (!selectedPreset) {
      toast.error('Select a preset to apply')
      return
    }

    setIsApplyingPreset(true)
    setApplyInfo(null)
    setValidationError(null)
    try {
      const res = await applyCrowdsecPreset({ slug: selectedPreset.slug, cache_key: presetMeta?.cacheKey })
      setApplyInfo({
        status: res.status,
        backup: res.backup,
        reloadHint: res.reload_hint,
        usedCscli: res.used_cscli,
        cacheKey: res.cache_key,
      })

      const reloadNote = res.reload_hint ? ' (reload required)' : ''
      toast.success(`Preset applied via backend${reloadNote}`)
      if (res.backup) {
        setPresetStatusMessage(`Backup stored at ${res.backup}`)
      }
    } catch (err) {
      if (isAxiosError(err)) {
        if (err.response?.status === 501) {
          toast.info('Preset apply is not available on the server; applying locally instead')
          await applyPresetLocally('Backend apply unavailable')
          return
        }

        if (err.response?.status === 400) {
          setValidationError(err.response?.data?.error || 'Preset validation failed')
          toast.error('Preset validation failed')
          return
        }

        if (err.response?.status === 503) {
          setHubUnavailable(true)
          setPresetStatusMessage('CrowdSec hub unavailable. Retry or load cached copy.')
          toast.error('Hub unavailable; retry pull/apply or use cached copy')
          return
        }

        const errorMsg = err.response?.data?.error || err.message
        const backupPath = (err.response?.data as { backup?: string })?.backup

        // Check if error is due to missing cache
        if (errorMsg.includes('not cached') || errorMsg.includes('Pull the preset first')) {
          toast.error(errorMsg)
          setValidationError('Preset must be pulled before applying. Click "Pull Preview" first.')
          return
        }

        if (backupPath) {
          setApplyInfo({ status: 'failed', backup: backupPath, cacheKey: presetMeta?.cacheKey })
          toast.error(`Apply failed: ${errorMsg}. Backup created at ${backupPath}`)
          return
        }
        toast.error(`Apply failed: ${errorMsg}`)
      } else {
        toast.error('Failed to apply preset')
      }
    } finally {
      setIsApplyingPreset(false)
    }
  }

  const presetActionDisabled =
    !selectedPreset ||
    isApplyingPreset ||
    backupMutation.isPending ||
    pullPresetMutation.isPending ||
    (selectedPresetRequiresHub && hubUnavailable)

  // Determine if any operation is in progress
  const isApplyingConfig =
    importMutation.isPending ||
    writeMutation.isPending ||
    updateModeMutation.isPending ||
    backupMutation.isPending ||
    pullPresetMutation.isPending ||
    isApplyingPreset ||
    banMutation.isPending ||
    unbanMutation.isPending

  // Determine contextual message
  const getMessage = () => {
    if (pullPresetMutation.isPending) {
      return { message: 'Fetching preset...', submessage: 'Pulling preview from CrowdSec Hub' }
    }
    if (isApplyingPreset) {
      return { message: 'Loading preset...', submessage: 'Applying curated preset with backup' }
    }
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
  if (!status.crowdsec) return <div className="p-8 text-center text-red-500">CrowdSec configuration not found in security status</div>

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
        <div className="flex items-center justify-between gap-4 flex-wrap">
          <div className="space-y-1">
            <h2 className="text-lg font-semibold">CrowdSec Mode</h2>
            <p className="text-sm text-gray-400">
              {isLocalMode ? 'CrowdSec runs locally; disable to pause decisions.' : 'CrowdSec decisions are paused; enable to resume local protection.'}
            </p>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-sm text-gray-400">Disabled</span>
            <Switch
              checked={isLocalMode}
              onChange={(e) => handleModeToggle(e.target.checked)}
              disabled={updateModeMutation.isPending}
              data-testid="crowdsec-mode-toggle"
            />
            <span className="text-sm text-gray-200">Local</span>
          </div>
        </div>
      </Card>

      <Card>
        <div className="space-y-4">
          <div className="flex items-center justify-between gap-3 flex-wrap">
            <h3 className="text-md font-semibold">Configuration Packages</h3>
            <div className="flex gap-2">
              <Button
                variant="secondary"
                onClick={handleExport}
                disabled={importMutation.isPending || backupMutation.isPending}
              >
                Export
              </Button>
              <Button onClick={handleImport} disabled={!file || importMutation.isPending || backupMutation.isPending} data-testid="import-btn">
                {importMutation.isPending ? 'Importing...' : 'Import'}
              </Button>
            </div>
          </div>
          <p className="text-sm text-gray-400">Import or export CrowdSec configuration packages. A backup is created before imports.</p>
          <input type="file" onChange={(e) => setFile(e.target.files?.[0] ?? null)} data-testid="import-file" accept=".tar.gz,.zip" />
        </div>
      </Card>

      <Card>
        <div className="space-y-4">
          <div className="flex items-start justify-between gap-3 flex-wrap">
            <div className="space-y-1">
              <h3 className="text-md font-semibold">CrowdSec Presets</h3>
              <p className="text-sm text-gray-400">Select a curated preset, preview it, then apply with an automatic backup.</p>
            </div>
            <div className="flex items-center gap-2 flex-wrap">
              <select
                className="bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white"
                value={selectedPresetSlug}
                onChange={(e) => setSelectedPresetSlug(e.target.value)}
                data-testid="preset-select"
              >
                {presetCatalog.map((preset) => (
                  <option key={preset.slug} value={preset.slug}>{preset.title}</option>
                ))}
              </select>
              <Button
                variant="secondary"
                onClick={() => selectedPreset && pullPresetMutation.mutate(selectedPreset.slug)}
                disabled={!selectedPreset || pullPresetMutation.isPending}
                isLoading={pullPresetMutation.isPending}
              >
                Pull Preview
              </Button>
              <Button
                onClick={handleApplyPreset}
                disabled={presetActionDisabled}
                isLoading={isApplyingPreset}
                data-testid="apply-preset-btn"
              >
                Apply Preset
              </Button>
            </div>
          </div>

          {validationError && (
            <p className="text-sm text-red-400" data-testid="preset-validation-error">{validationError}</p>
          )}

          {presetStatusMessage && (
            <p className="text-sm text-yellow-300" data-testid="preset-status">{presetStatusMessage}</p>
          )}

          {hubUnavailable && (
            <div className="flex flex-wrap gap-2 items-center text-sm text-red-300" data-testid="preset-hub-unavailable">
              <span>Hub unreachable. Retry pull or load cached copy if available.</span>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => selectedPreset && pullPresetMutation.mutate(selectedPreset.slug)}
                disabled={pullPresetMutation.isPending}
              >
                Retry
              </Button>
              {selectedPreset?.cached && (
                <Button size="sm" variant="secondary" onClick={loadCachedPreview}>Use cached preview</Button>
              )}
            </div>
          )}

          {selectedPreset && (
            <div className="space-y-3">
              <div className="space-y-1">
                <p className="text-sm font-semibold text-white">{selectedPreset.title}</p>
                <p className="text-sm text-gray-400">{selectedPreset.description}</p>
                {selectedPreset.warning && (
                  <p className="text-xs text-yellow-300" data-testid="preset-warning">{selectedPreset.warning}</p>
                )}
                <p className="text-xs text-gray-500">Target file: {selectedPath ?? 'Select a file below (used for local fallback)'} </p>
              </div>
              {presetMeta && (
                <div className="text-xs text-gray-400 flex flex-wrap gap-3" data-testid="preset-meta">
                  <span>Cache key: {presetMeta.cacheKey || '—'}</span>
                  <span>Etag: {presetMeta.etag || '—'}</span>
                  <span>Source: {presetMeta.source || selectedPreset.source || '—'}</span>
                  <span>Fetched: {presetMeta.retrievedAt ? new Date(presetMeta.retrievedAt).toLocaleString() : '—'}</span>
                </div>
              )}
              <div>
                <p className="text-xs text-gray-400 mb-2">Preset preview (YAML)</p>
                <pre
                  className="bg-gray-900 border border-gray-800 rounded-lg p-3 text-sm text-gray-200 whitespace-pre-wrap"
                  data-testid="preset-preview"
                >
                  {presetPreview || selectedPreset.content || 'Preview unavailable. Pull from hub or use cached copy.'}
                </pre>
              </div>

              {applyInfo && (
                <div className="rounded-lg border border-gray-800 bg-gray-900/70 p-3 text-xs text-gray-200" data-testid="preset-apply-info">
                  <p>Status: {applyInfo.status || 'applied'}</p>
                  {applyInfo.backup && <p>Backup: {applyInfo.backup}</p>}
                  {applyInfo.reloadHint && <p>Reload: {applyInfo.reloadHint}</p>}
                  {applyInfo.usedCscli !== undefined && <p>Method: {applyInfo.usedCscli ? 'cscli' : 'filesystem'}</p>}
                </div>
              )}

              <div className="flex flex-wrap gap-2 items-center text-xs text-gray-400">
                {selectedPreset.cached && (
                  <Button size="sm" variant="secondary" onClick={loadCachedPreview}>
                    Load cached preview
                  </Button>
                )}
                {selectedPresetRequiresHub && hubUnavailable && (
                  <span className="text-red-300">Apply disabled while hub is offline.</span>
                )}
              </div>
            </div>
          )}

          {presetCatalog.length === 0 && (
            <p className="text-sm text-gray-500">No presets available. Ensure Cerberus is enabled.</p>
          )}
        </div>
      </Card>

      <Card>
        <div className="space-y-4">
          <h3 className="text-md font-semibold">Edit Configuration Files</h3>
          <div className="flex items-center gap-2">
            <select
              className="bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white"
              value={selectedPath ?? ''}
              onChange={(e) => handleReadFile(e.target.value)}
              data-testid="crowdsec-file-select"
            >
              <option value="">Select a file...</option>
              {listMutation.data?.files?.map((f) => (
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
