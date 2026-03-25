import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import { Shield, ShieldOff, Trash2, Search, AlertTriangle, ExternalLink } from 'lucide-react'
import { lazy, Suspense, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, Link } from 'react-router-dom'

import { createBackup } from '../api/backups'
import { exportCrowdsecConfig, importCrowdsecConfig, listCrowdsecFiles, readCrowdsecFile, writeCrowdsecFile, listCrowdsecDecisions, banIP, unbanIP, type CrowdSecDecision, statusCrowdsec, type CrowdSecStatus, startCrowdsec } from '../api/crowdsec'
import { getFeatureFlags } from '../api/featureFlags'
import { listCrowdsecPresets, pullCrowdsecPreset, applyCrowdsecPreset, getCrowdsecPresetCache } from '../api/presets'
import { getSecurityStatus } from '../api/security'
import { CrowdSecBouncerKeyDisplay } from '../components/CrowdSecBouncerKeyDisplay'
import { ConfigReloadOverlay } from '../components/LoadingStates'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { Skeleton } from '../components/ui/Skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/Tabs'
import { CROWDSEC_PRESETS, type CrowdsecPreset } from '../data/crowdsecPresets'
import { useConsoleStatus, useEnrollConsole, useClearConsoleEnrollment } from '../hooks/useConsoleEnrollment'
import { buildCrowdsecExportFilename, downloadCrowdsecExport, promptCrowdsecFilename } from '../utils/crowdsecExport'
import { toast } from '../utils/toast'

const CrowdSecDashboard = lazy(() => import('../components/crowdsec/CrowdSecDashboard'))

export default function CrowdSecConfig() {
  const { t } = useTranslation()
  const { data: status, isLoading, error } = useQuery({ queryKey: ['security-status'], queryFn: getSecurityStatus })
  const [searchQuery, setSearchQuery] = useState('')
  const [sortBy, setSortBy] = useState<'alpha' | 'type' | 'source'>('alpha')
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
  // Read the "CrowdSec is starting" signal broadcast by Security.tsx via the
  // QueryClient cache. No HTTP call is made; this is pure in-memory coordination.
  const { data: crowdsecStartingCache } = useQuery<{ isStarting: boolean; startedAt?: number }>({
    queryKey: ['crowdsec-starting'],
    queryFn: () => ({ isStarting: false, startedAt: 0 }),
    staleTime: Infinity,
    gcTime: Infinity,
  })

  // isStartingUp is true only while the mutation is genuinely running.
  // The 90-second cap guards against stale cache if Security.tsx onSuccess/onError
  // never fired (e.g., browser tab was closed mid-mutation).
  const isStartingUp =
    (crowdsecStartingCache?.isStarting === true) &&
    Date.now() - (crowdsecStartingCache.startedAt ?? 0) < 90_000

  const isLocalMode = !!status && status.crowdsec?.mode !== 'disabled'
  // Note: CrowdSec mode is now controlled via Security Dashboard toggle
  const { data: featureFlags } = useQuery({ queryKey: ['feature-flags'], queryFn: getFeatureFlags })
  const consoleEnrollmentEnabled = Boolean(featureFlags?.['feature.crowdsec.console_enrollment'])
  const [enrollmentToken, setEnrollmentToken] = useState('')
  const [consoleTenant, setConsoleTenant] = useState('')
  const [consoleAgentName, setConsoleAgentName] = useState((typeof window !== 'undefined' && window.location?.hostname) || 'charon-agent')
  const [consoleAck, setConsoleAck] = useState(false)
  const [consoleErrors, setConsoleErrors] = useState<{ token?: string; agent?: string; tenant?: string; ack?: string; submit?: string }>({})
  const consoleStatusQuery = useConsoleStatus(consoleEnrollmentEnabled)
  const enrollConsoleMutation = useEnrollConsole()
  const clearEnrollmentMutation = useClearConsoleEnrollment()
  const [showReenrollForm, setShowReenrollForm] = useState(false)
  const navigate = useNavigate()
  const [initialCheckComplete, setInitialCheckComplete] = useState(false)

  // Add initial delay to avoid false negative when LAPI is starting
  useEffect(() => {
    if (consoleEnrollmentEnabled && !initialCheckComplete) {
      const timer = setTimeout(() => {
        setInitialCheckComplete(true)
      }, 3000) // Wait 3 seconds before first check
      return () => clearTimeout(timer)
    }
  }, [consoleEnrollmentEnabled, initialCheckComplete])

  // Add LAPI status check with polling
  const lapiStatusQuery = useQuery<CrowdSecStatus>({
    queryKey: ['crowdsec-lapi-status'],
    queryFn: statusCrowdsec,
    enabled: consoleEnrollmentEnabled && initialCheckComplete,
    refetchInterval: 5000, // Poll every 5 seconds
    retry: false,
  })

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

  const filteredPresets = useMemo(() => {
    let result = [...presetCatalog]

    if (searchQuery) {
      const query = searchQuery.toLowerCase()
      result = result.filter(
        (p) =>
          p.title.toLowerCase().includes(query) ||
          p.description?.toLowerCase().includes(query) ||
          p.slug.toLowerCase().includes(query)
      )
    }

    result.sort((a, b) => {
      if (sortBy === 'alpha') {
        return a.title.localeCompare(b.title)
      }
      if (sortBy === 'source') {
        const sourceA = a.source || 'z'
        const sourceB = b.source || 'z'
        if (sourceA !== sourceB) return sourceA.localeCompare(sourceB)
        return a.title.localeCompare(b.title)
      }
      return a.title.localeCompare(b.title)
    })

    return result
  }, [presetCatalog, searchQuery, sortBy])

  useEffect(() => {
    if (!presetCatalog.length) return
    if (!selectedPresetSlug || !presetCatalog.some((preset) => preset.slug === selectedPresetSlug)) {
      setSelectedPresetSlug(presetCatalog[0].slug)
    }
  }, [presetCatalog, selectedPresetSlug])

  useEffect(() => {
    if (consoleStatusQuery.data?.agent_name) {
      setConsoleAgentName((prev) => prev || consoleStatusQuery.data?.agent_name || prev)
    }
    if (consoleStatusQuery.data?.tenant) {
      setConsoleTenant((prev) => prev || consoleStatusQuery.data?.tenant || prev)
    }
  }, [consoleStatusQuery.data?.agent_name, consoleStatusQuery.data?.tenant])

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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- Only re-run when slug changes, not on mutation/preset object identity changes
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

  const normalizedConsoleStatus = consoleStatusQuery.data?.status === 'failed' ? 'degraded' : consoleStatusQuery.data?.status || 'not_enrolled'
  const isConsoleDegraded = normalizedConsoleStatus === 'degraded'
  const isConsolePending = enrollConsoleMutation.isPending || normalizedConsoleStatus === 'enrolling'
  const isConsolePendingAcceptance = normalizedConsoleStatus === 'pending_acceptance'
  const consoleStatusLabel = normalizedConsoleStatus.replace('_', ' ')
  const consoleTokenState = consoleStatusQuery.data ? (consoleStatusQuery.data.key_present ? 'Stored (masked)' : 'Not stored') : '—'
  const canRotateKey = normalizedConsoleStatus === 'enrolled' || normalizedConsoleStatus === 'degraded' || isConsolePendingAcceptance
  const consoleDocsHref = 'https://wikid82.github.io/charon/security/'

  const sanitizeSecret = (msg: string) => msg.replace(/\b[A-Za-z0-9]{10,64}\b/g, '***')

  const sanitizeErrorMessage = (err: unknown) => {
    if (isAxiosError(err)) {
      return sanitizeSecret(err.response?.data?.error || err.message)
    }
    if (err instanceof Error) return sanitizeSecret(err.message)
    return 'Console enrollment failed'
  }

  const validateConsoleEnrollment = (options?: { allowMissingTenant?: boolean; requireAck?: boolean }) => {
    const nextErrors: { token?: string; agent?: string; tenant?: string; ack?: string } = {}
    if (!enrollmentToken.trim()) {
      nextErrors.token = 'Enrollment token is required'
    }
    if (!consoleAgentName.trim()) {
      nextErrors.agent = 'Agent name is required'
    }
    if (!consoleTenant.trim() && !options?.allowMissingTenant) {
      nextErrors.tenant = 'Tenant / organization is required'
    }
    if (options?.requireAck && !consoleAck) {
      nextErrors.ack = 'You must acknowledge the console data-sharing notice'
    }
    setConsoleErrors(nextErrors)
    return Object.keys(nextErrors).length === 0
  }

  const submitConsoleEnrollment = async (force = false) => {
    const allowMissingTenant = force && !consoleTenant.trim()
    const requireAck = normalizedConsoleStatus === 'not_enrolled'
    if (!validateConsoleEnrollment({ allowMissingTenant, requireAck })) return
    const tenantValue = consoleTenant.trim() || consoleStatusQuery.data?.tenant || consoleAgentName || 'charon-agent'
    try {
      await enrollConsoleMutation.mutateAsync({
        enrollment_key: enrollmentToken.trim(),
        tenant: tenantValue,
        agent_name: consoleAgentName.trim(),
        force,
      })
      setConsoleErrors({})
      setEnrollmentToken('')
      setShowReenrollForm(false)
      if (!consoleTenant.trim()) {
        setConsoleTenant(tenantValue)
      }
      toast.success(
        force
          ? 'Enrollment submitted! Accept the request on app.crowdsec.net to complete.'
          : 'Enrollment request sent! Accept the enrollment on app.crowdsec.net to complete registration.'
      )
    } catch (err) {
      const message = sanitizeErrorMessage(err)
      setConsoleErrors((prev) => ({ ...prev, submit: message }))
      toast.error(message)
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
    if (banMutation.isPending) {
      return { message: 'Guardian raises shield...', submessage: 'Banning IP address' }
    }
    if (unbanMutation.isPending) {
      return { message: 'Guardian lowers shield...', submessage: 'Unbanning IP address' }
    }
    return { message: 'Strengthening the guard...', submessage: 'Configuration in progress' }
  }

  const { message, submessage } = getMessage()

  if (isLoading) return <div className="p-8 text-center text-white">{t('crowdsecConfig.loading')}</div>
  if (error) return <div className="p-8 text-center text-red-500">{t('crowdsecConfig.loadError')}: {(error as Error).message}</div>
  if (!status) return <div className="p-8 text-center text-gray-400">{t('crowdsecConfig.noStatus')}</div>
  if (!status.crowdsec) return <div className="p-8 text-center text-red-500">{t('crowdsecConfig.noConfig')}</div>

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
      <h1 className="text-2xl font-bold">{t('crowdsecConfig.title')}</h1>

      <Tabs defaultValue="config">
        <TabsList>
          <TabsTrigger value="config">{t('security.crowdsec.tabs.config', 'Configuration')}</TabsTrigger>
          <TabsTrigger value="dashboard">{t('security.crowdsec.tabs.dashboard', 'Dashboard')}</TabsTrigger>
        </TabsList>

        <TabsContent value="dashboard" className="mt-4">
          <Suspense fallback={<div className="space-y-4"><Skeleton className="h-24 w-full" /><Skeleton className="h-64 w-full" /></div>}>
            <CrowdSecDashboard />
          </Suspense>
        </TabsContent>

        <TabsContent value="config" className="mt-4">

      {/* CrowdSec Bouncer API Key - moved from Security Dashboard */}
      {status.cerberus?.enabled && status.crowdsec.enabled && (
        <CrowdSecBouncerKeyDisplay />
      )}

      <div className="bg-blue-900/20 border border-blue-700 rounded-lg p-4 mb-4">
        <p className="text-sm text-blue-200">
          <strong>{t('crowdsecConfig.note')}:</strong> {t('crowdsecConfig.noteText')}{' '}
          <Link to="/security" className="text-blue-400 hover:text-blue-300 underline">{t('navigation.security')}</Link>.
        </p>
      </div>

      {consoleEnrollmentEnabled && (
        <Card data-testid="console-enrollment-card">
          <div className="space-y-4">
            <div className="flex items-start justify-between gap-3 flex-wrap">
              <div className="space-y-1">
                <h3 className="text-md font-semibold">{t('crowdsecConfig.consoleEnrollment.title')}</h3>
                <p className="text-sm text-gray-400">{t('crowdsecConfig.consoleEnrollment.description')}</p>
                <p className="text-xs text-gray-500">
                  {t('crowdsecConfig.consoleEnrollment.privacyNote')}
                  <a className="text-blue-400 hover:underline ml-1" href={consoleDocsHref} target="_blank" rel="noreferrer">{t('common.docs')}</a>
                </p>
              </div>
              <div className="text-right text-sm text-gray-300 space-y-1">
                <p className="font-semibold text-white capitalize" data-testid="console-status-label">{t('common.status')}: {consoleStatusLabel}</p>
                <p className="text-xs text-gray-500">{t('crowdsecConfig.consoleEnrollment.lastHeartbeat')}: {consoleStatusQuery.data?.last_heartbeat_at ? new Date(consoleStatusQuery.data.last_heartbeat_at).toLocaleString() : '—'}</p>
              </div>
            </div>

            {consoleStatusQuery.data?.last_error && (
              <p className="text-xs text-yellow-300" data-testid="console-status-error">{t('crowdsecConfig.consoleEnrollment.lastError')}: {sanitizeSecret(consoleStatusQuery.data.last_error)}</p>
            )}
            {consoleErrors.submit && (
              <p className="text-sm text-red-400" data-testid="console-enroll-error">{consoleErrors.submit}</p>
            )}

            {/* Yellow warning: Process running but LAPI initializing */}
            {lapiStatusQuery.data && lapiStatusQuery.data.running && !lapiStatusQuery.data.lapi_ready && initialCheckComplete && !isStartingUp && (
              <div className="flex items-start gap-3 p-4 bg-yellow-900/20 border border-yellow-700/50 rounded-lg" data-testid="lapi-warning">
                <AlertTriangle className="w-5 h-5 text-yellow-400 shrink-0 mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm text-yellow-200 font-medium mb-2">
                    {t('crowdsecConfig.lapiInitializing')}
                  </p>
                  <p className="text-xs text-yellow-300 mb-3">
                    {t('crowdsecConfig.lapiInitializingDesc')}
                    {lapiStatusQuery.isRefetching && ` ${t('crowdsecConfig.checkingAgain')}`}
                  </p>
                  <div className="flex gap-2">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => lapiStatusQuery.refetch()}
                      disabled={lapiStatusQuery.isRefetching}
                    >
                      Check Now
                    </Button>
                  </div>
                </div>
              </div>
            )}

            {/* Red warning: Process not running at all */}
            {lapiStatusQuery.data && !lapiStatusQuery.data.running && initialCheckComplete && !isStartingUp && (
              <div className="flex items-start gap-3 p-4 bg-red-900/20 border border-red-700/50 rounded-lg" data-testid="lapi-not-running-warning">
                <AlertTriangle className="w-5 h-5 text-red-400 shrink-0 mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm text-red-200 font-medium mb-2">
                    {t('crowdsecConfig.notRunning')}
                  </p>
                  <p className="text-xs text-red-300 mb-3">
                    {t('crowdsecConfig.notRunningDesc')}
                  </p>
                  <div className="flex gap-2">
                    <Button
                      variant="primary"
                      size="sm"
                      onClick={async () => {
                        try {
                          await startCrowdsec();
                          toast.info(t('crowdsecConfig.starting'));
                          // Refetch status after a delay to allow startup
                          setTimeout(() => {
                            lapiStatusQuery.refetch();
                          }, 3000);
                        } catch {
                          toast.error(t('crowdsecConfig.startFailed'));
                        }
                      }}
                    >
                      {t('crowdsecConfig.startCrowdsec')}
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => lapiStatusQuery.refetch()}
                      disabled={lapiStatusQuery.isRefetching}
                    >
                      Check Now
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => navigate('/security')}
                    >
                      {t('crowdsecConfig.goToSecurity')}
                    </Button>
                  </div>
                </div>
              </div>
            )}

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <Input
                id="console-enrollment-token"
                label={t('crowdsecConfig.consoleEnrollment.enrollToken')}
                type="password"
                value={enrollmentToken}
                onChange={(e) => setEnrollmentToken(e.target.value)}
                placeholder={t('crowdsecConfig.consoleEnrollment.tokenPlaceholder')}
                helperText={t('crowdsecConfig.consoleEnrollment.tokenHelper')}
                error={consoleErrors.token}
                errorTestId="console-enroll-error"
                data-testid="console-enrollment-token"
              />
              <Input
                id="console-agent-name"
                label={t('crowdsecConfig.consoleEnrollment.agentName')}
                value={consoleAgentName}
                onChange={(e) => setConsoleAgentName(e.target.value)}
                error={consoleErrors.agent}
                errorTestId="console-enroll-error"
                data-testid="console-agent-name"
              />
              <Input
                id="console-tenant"
                label={t('crowdsecConfig.consoleEnrollment.tenant')}
                value={consoleTenant}
                onChange={(e) => setConsoleTenant(e.target.value)}
                helperText={t('crowdsecConfig.consoleEnrollment.tenantHelper')}
                error={consoleErrors.tenant}
                errorTestId="console-enroll-error"
                data-testid="console-tenant"
              />
            </div>

            <div className="flex items-center gap-2">
              <input
                id="console-ack"
                type="checkbox"
                className="h-4 w-4 accent-blue-500"
                checked={consoleAck}
                onChange={(e) => setConsoleAck(e.target.checked)}
                disabled={isConsolePending}
                data-testid="console-ack-checkbox"
              />
              <label htmlFor="console-ack" className="text-sm text-gray-400">{t('crowdsecConfig.consoleEnrollment.ackText')}</label>
            </div>
            {consoleErrors.ack && <p className="text-sm text-red-400" data-testid="console-enroll-error">{consoleErrors.ack}</p>}

            <div className="flex flex-wrap gap-2">
              <Button
                onClick={() => submitConsoleEnrollment(false)}
                disabled={isConsolePending || (lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready) || !enrollmentToken.trim()}
                isLoading={enrollConsoleMutation.isPending}
                data-testid="console-enroll-btn"
                title={
                  lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready
                    ? t('crowdsecConfig.consoleEnrollment.lapiMustBeReady')
                    : !enrollmentToken.trim()
                    ? t('crowdsecConfig.consoleEnrollment.tokenRequired')
                    : undefined
                }
              >
                {t('crowdsecConfig.consoleEnrollment.enroll')}
              </Button>
              <Button
                variant="secondary"
                onClick={() => submitConsoleEnrollment(true)}
                disabled={isConsolePending || !canRotateKey || (lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready)}
                isLoading={enrollConsoleMutation.isPending}
                data-testid="console-rotate-btn"
                title={
                  lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready
                    ? t('crowdsecConfig.consoleEnrollment.lapiMustBeReadyRotate')
                    : undefined
                }
              >
                {t('crowdsecConfig.consoleEnrollment.rotateKey')}
              </Button>
              {isConsoleDegraded && (
                <Button
                  variant="secondary"
                  onClick={() => submitConsoleEnrollment(true)}
                  disabled={isConsolePending || (lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready)}
                  isLoading={enrollConsoleMutation.isPending}
                  data-testid="console-retry-btn"
                  title={
                    lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready
                      ? t('crowdsecConfig.consoleEnrollment.lapiMustBeReadyRetry')
                      : undefined
                  }
                >
                  {t('crowdsecConfig.consoleEnrollment.retryEnrollment')}
                </Button>
              )}
            </div>

            {/* Info box for pending acceptance status */}
            {isConsolePendingAcceptance && (
              <div className="bg-blue-900/30 border border-blue-500 rounded-lg p-4" data-testid="pending-acceptance-info">
                <p className="text-sm text-blue-200">
                  <strong>{t('crowdsecConfig.consoleEnrollment.actionRequired')}:</strong> {t('crowdsecConfig.consoleEnrollment.pendingAcceptanceText')}{' '}
                  <a
                    href="https://app.crowdsec.net"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="underline hover:text-blue-100"
                  >
                    app.crowdsec.net
                  </a>.
                  {t('crowdsecConfig.consoleEnrollment.pendingAcceptanceNote')}
                </p>
              </div>
            )}

            {/* Re-enrollment Section - shown when enrolled or pending */}
            {(normalizedConsoleStatus === 'enrolled' || normalizedConsoleStatus === 'pending_acceptance') && (
              <div className="border border-blue-500/30 bg-blue-500/5 rounded-lg p-4" data-testid="reenroll-section">
                <div className="space-y-4">
                  <div className="flex items-start justify-between">
                    <div>
                      <h3 className="text-lg font-semibold text-gray-100">{t('crowdsecConfig.reenroll.title')}</h3>
                      <p className="text-sm text-gray-400 mt-1">
                        {t('crowdsecConfig.reenroll.description')}
                      </p>
                    </div>
                  </div>

                  {!showReenrollForm ? (
                    <div className="flex flex-wrap gap-3">
                      <a
                        href="https://app.crowdsec.net/security-engines"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-2 text-blue-400 hover:text-blue-300 text-sm"
                      >
                        <ExternalLink className="w-4 h-4" />
                        {t('crowdsecConfig.reenroll.getNewKey')}
                      </a>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => setShowReenrollForm(true)}
                        data-testid="show-reenroll-form-btn"
                      >
                        {t('crowdsecConfig.reenroll.withNewKey')}
                      </Button>
                    </div>
                  ) : (
                    <div className="space-y-4 pt-2 border-t border-gray-700">
                      {/* Re-enrollment form */}
                      <div className="space-y-3">
                        <div>
                          <label className="block text-sm font-medium text-gray-300 mb-1" htmlFor="reenroll-token">
                            {t('crowdsecConfig.reenroll.newEnrollmentKey')}
                          </label>
                          <Input
                            id="reenroll-token"
                            type="text"
                            value={enrollmentToken}
                            onChange={(e) => setEnrollmentToken(e.target.value)}
                            placeholder={t('crowdsecConfig.reenroll.keyPlaceholder')}
                            data-testid="reenroll-token-input"
                          />
                        </div>
                        <div>
                          <label className="block text-sm font-medium text-gray-300 mb-1" htmlFor="reenroll-agent-name">
                            {t('crowdsecConfig.consoleEnrollment.agentName')}
                          </label>
                          <Input
                            id="reenroll-agent-name"
                            type="text"
                            value={consoleAgentName}
                            onChange={(e) => setConsoleAgentName(e.target.value)}
                            placeholder={t('crowdsecConfig.reenroll.agentPlaceholder')}
                          />
                        </div>
                        <div>
                          <label className="block text-sm font-medium text-gray-300 mb-1" htmlFor="reenroll-tenant">
                            {t('crowdsecConfig.reenroll.tenantOptional')}
                          </label>
                          <Input
                            id="reenroll-tenant"
                            type="text"
                            value={consoleTenant}
                            onChange={(e) => setConsoleTenant(e.target.value)}
                            placeholder={t('crowdsecConfig.reenroll.orgPlaceholder')}
                          />
                        </div>
                      </div>

                      <div className="flex gap-3">
                        <Button
                          variant="primary"
                          onClick={() => submitConsoleEnrollment(true)}
                          disabled={!enrollmentToken.trim() || enrollConsoleMutation.isPending}
                          isLoading={enrollConsoleMutation.isPending}
                          data-testid="reenroll-submit-btn"
                        >
                          {enrollConsoleMutation.isPending ? t('crowdsecConfig.reenroll.reenrolling') : t('crowdsecConfig.reenroll.reenroll')}
                        </Button>
                        <Button
                          variant="ghost"
                          onClick={() => setShowReenrollForm(false)}
                        >
                          {t('common.cancel')}
                        </Button>
                      </div>
                    </div>
                  )}

                  {/* Clear enrollment option */}
                  <div className="pt-3 border-t border-gray-700">
                    <button
                      onClick={() => {
                        if (window.confirm(t('crowdsecConfig.reenroll.clearConfirm'))) {
                          clearEnrollmentMutation.mutate()
                        }
                      }}
                      className="text-sm text-gray-500 hover:text-gray-400"
                      disabled={clearEnrollmentMutation.isPending}
                      data-testid="clear-enrollment-btn"
                    >
                      {clearEnrollmentMutation.isPending ? t('crowdsecConfig.reenroll.clearing') : t('crowdsecConfig.reenroll.clearState')}
                    </button>
                  </div>
                </div>
              </div>
            )}

            <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-sm text-gray-400">
              <div>
                <p className="text-xs text-gray-500">{t('crowdsecConfig.consoleEnrollment.agent')}</p>
                <p className="text-white">{consoleStatusQuery.data?.agent_name || consoleAgentName || '—'}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500">{t('crowdsecConfig.consoleEnrollment.tenantLabel')}</p>
                <p className="text-white">{consoleStatusQuery.data?.tenant || consoleTenant || '—'}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500">{t('crowdsecConfig.consoleEnrollment.enrollmentToken')}</p>
                <p className="text-white" data-testid="console-token-state">{consoleTokenState}</p>
              </div>
              <div className="md:col-span-3 flex flex-wrap gap-4 text-xs text-gray-500">
                <span>{t('crowdsecConfig.consoleEnrollment.lastAttempt')}: {consoleStatusQuery.data?.last_attempt_at ? new Date(consoleStatusQuery.data.last_attempt_at).toLocaleString() : '—'}</span>
                <span>{t('crowdsecConfig.consoleEnrollment.enrolledAt')}: {consoleStatusQuery.data?.enrolled_at ? new Date(consoleStatusQuery.data.enrolled_at).toLocaleString() : '—'}</span>
                {consoleStatusQuery.data?.correlation_id && <span>{t('crowdsecConfig.consoleEnrollment.correlationId')}: {consoleStatusQuery.data.correlation_id}</span>}
              </div>
            </div>
          </div>
        </Card>
      )}

      <Card>
        <div className="space-y-4">
          <div className="flex items-center justify-between gap-3 flex-wrap">
            <h3 className="text-md font-semibold">{t('crowdsecConfig.packages.title')}</h3>
            <div className="flex gap-2">
              <Button
                variant="secondary"
                onClick={handleExport}
                disabled={importMutation.isPending || backupMutation.isPending}
              >
                {t('crowdsecConfig.packages.export')}
              </Button>
              <Button onClick={handleImport} disabled={!file || importMutation.isPending || backupMutation.isPending} data-testid="import-btn">
                {importMutation.isPending ? t('crowdsecConfig.packages.importing') : t('crowdsecConfig.packages.import')}
              </Button>
            </div>
          </div>
          <p className="text-sm text-gray-400">{t('crowdsecConfig.packages.description')}</p>
          <input
            type="file"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            data-testid="import-file"
            accept=".tar.gz,.zip"
            aria-label={t('crowdsecConfig.packages.selectFile') || "Select CrowdSec package"}
          />
        </div>
      </Card>

      <Card>
        <div className="space-y-4">
          <div className="space-y-1">
            <h3 className="text-md font-semibold">{t('crowdsecConfig.presets.title')}</h3>
            <p className="text-sm text-gray-400">{t('crowdsecConfig.presets.description')}</p>
          </div>

          <div className="flex gap-2">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
              <input
                type="text"
                placeholder={t('crowdsecConfig.presets.searchPlaceholder')}
                aria-label={t('crowdsecConfig.presets.searchPlaceholder')}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg pl-9 pr-3 py-2 text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <select
              value={sortBy}
              aria-label={t('crowdsecConfig.presets.sortBy') || "Sort presets"}
              onChange={(e) => setSortBy(e.target.value as 'alpha' | 'type' | 'source')}
              className="bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white"
            >
              <option value="alpha">{t('crowdsecConfig.presets.sortAlpha')}</option>
              <option value="source">{t('crowdsecConfig.presets.sortSource')}</option>
            </select>
          </div>

          <div className="border border-gray-700 rounded-lg max-h-60 overflow-y-auto bg-gray-900">
            {filteredPresets.length > 0 ? (
              filteredPresets.map((preset) => (
                <div
                  key={preset.slug}
                  onClick={() => setSelectedPresetSlug(preset.slug)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      setSelectedPresetSlug(preset.slug)
                    }
                  }}
                  aria-pressed={selectedPresetSlug === preset.slug}
                  className={`p-3 cursor-pointer hover:bg-gray-800 border-b border-gray-800 last:border-0 outline-none focus-visible:bg-gray-800 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 ${
                    selectedPresetSlug === preset.slug ? 'bg-blue-900/20 border-l-2 border-l-blue-500' : 'border-l-2 border-l-transparent'
                  }`}
                >
                  <div className="flex justify-between items-start mb-1">
                    <span className="font-medium text-white">{preset.title}</span>
                    {preset.source && (
                      <span className={`text-xs px-2 py-0.5 rounded-full ${
                        preset.source === 'charon-curated' ? 'bg-purple-900/50 text-purple-300' : 'bg-blue-900/50 text-blue-300'
                      }`}>
                        {preset.source === 'charon-curated' ? t('crowdsecConfig.presets.curated') : t('crowdsecConfig.presets.hub')}
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-gray-400 line-clamp-1">{preset.description}</p>
                </div>
              ))
            ) : (
              <div className="p-4 text-center text-gray-500 text-sm">{t('crowdsecConfig.presets.noPresets', { query: searchQuery })}</div>
            )}
          </div>

          <div className="flex items-center gap-2 flex-wrap justify-end">
            <Button
              variant="secondary"
              onClick={() => selectedPreset && pullPresetMutation.mutate(selectedPreset.slug)}
              disabled={!selectedPreset || pullPresetMutation.isPending}
              isLoading={pullPresetMutation.isPending}
            >
              {t('crowdsecConfig.presets.pullPreview')}
            </Button>
            <Button
              onClick={handleApplyPreset}
              disabled={presetActionDisabled}
              isLoading={isApplyingPreset}
              data-testid="apply-preset-btn"
            >
              {t('crowdsecConfig.presets.applyPreset')}
            </Button>
          </div>

          {validationError && (
            <p className="text-sm text-red-400" data-testid="preset-validation-error">{validationError}</p>
          )}

          {presetStatusMessage && (
            <p className="text-sm text-yellow-300" data-testid="preset-status">{presetStatusMessage}</p>
          )}

          {hubUnavailable && (
            <div className="flex flex-wrap gap-2 items-center text-sm text-red-300" data-testid="preset-hub-unavailable">
              <span>{t('crowdsecConfig.presets.hubUnreachable')}</span>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => selectedPreset && pullPresetMutation.mutate(selectedPreset.slug)}
                disabled={pullPresetMutation.isPending}
              >
                {t('crowdsecConfig.presets.retry')}
              </Button>
              {selectedPreset?.cached && (
                <Button size="sm" variant="secondary" onClick={loadCachedPreview}>{t('crowdsecConfig.presets.useCached')}</Button>
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
                <p className="text-xs text-gray-500">{t('crowdsecConfig.presets.targetFile')}: {selectedPath ?? t('crowdsecConfig.presets.selectFileBelow')} </p>
              </div>
              {presetMeta && (
                <div className="text-xs text-gray-400 flex flex-wrap gap-3" data-testid="preset-meta">
                  <span>{t('crowdsecConfig.presets.cacheKey')}: {presetMeta.cacheKey || '—'}</span>
                  <span>{t('crowdsecConfig.presets.etag')}: {presetMeta.etag || '—'}</span>
                  <span>{t('crowdsecConfig.presets.source')}: {presetMeta.source || selectedPreset.source || '—'}</span>
                  <span>{t('crowdsecConfig.presets.fetched')}: {presetMeta.retrievedAt ? new Date(presetMeta.retrievedAt).toLocaleString() : '—'}</span>
                </div>
              )}
              <div>
                <p className="text-xs text-gray-400 mb-2">{t('crowdsecConfig.presets.previewYaml')}</p>
                <pre
                  className="bg-gray-900 border border-gray-800 rounded-lg p-3 text-sm text-gray-200 whitespace-pre-wrap"
                  data-testid="preset-preview"
                >
                  {presetPreview || selectedPreset.content || t('crowdsecConfig.presets.previewUnavailable')}
                </pre>
              </div>

              {applyInfo && (
                <div className="rounded-lg border border-gray-800 bg-gray-900/70 p-3 text-xs text-gray-200" data-testid="preset-apply-info">
                  <p>{t('common.status')}: {applyInfo.status || t('crowdsecConfig.presets.applied')}</p>
                  {applyInfo.backup && <p>{t('crowdsecConfig.presets.backup')}: {applyInfo.backup}</p>}
                  {applyInfo.reloadHint && <p>{t('crowdsecConfig.presets.reload')}: {t('crowdsecConfig.presets.required')}</p>}
                  {applyInfo.usedCscli !== undefined && <p>{t('crowdsecConfig.presets.method')}: {applyInfo.usedCscli ? 'cscli' : t('crowdsecConfig.presets.filesystem')}</p>}
                </div>
              )}

              <div className="flex flex-wrap gap-2 items-center text-xs text-gray-400">
                {selectedPreset.cached && (
                  <Button size="sm" variant="secondary" onClick={loadCachedPreview}>
                    {t('crowdsecConfig.presets.loadCached')}
                  </Button>
                )}
                {selectedPresetRequiresHub && hubUnavailable && (
                  <span className="text-red-300">{t('crowdsecConfig.presets.applyDisabled')}</span>
                )}
              </div>
            </div>
          )}

          {presetCatalog.length === 0 && (
            <p className="text-sm text-gray-500">{t('crowdsecConfig.presets.noPresets')}</p>
          )}
        </div>
      </Card>

      <Card>
        <div className="space-y-4">
          <h3 className="text-md font-semibold">{t('crowdsecConfig.files.title')}</h3>
          <div className="flex items-center gap-2">
            <select
              className="bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white"
              value={selectedPath ?? ''}
              onChange={(e) => handleReadFile(e.target.value)}
              data-testid="crowdsec-file-select"
              aria-label={t('crowdsecConfig.files.selectFile')}
            >
              <option value="">{t('crowdsecConfig.files.selectFile')}</option>
              {listMutation.data?.files?.map((f) => (
                <option value={f} key={f}>{f}</option>
              ))}
            </select>
            <Button variant="secondary" onClick={() => listMutation.refetch()}>{t('crowdsecConfig.files.refresh')}</Button>
          </div>
          <textarea
            value={fileContent ?? ''}
            onChange={(e) => setFileContent(e.target.value)}
            rows={12}
            className="w-full bg-gray-900 border border-gray-700 rounded-lg p-3 text-white"
            aria-label={t('crowdsecConfig.files.content') || "File content"}
          />
          <div className="flex gap-2">
            <Button onClick={handleSaveFile} isLoading={writeMutation.isPending || backupMutation.isPending}>{t('common.save')}</Button>
            <Button variant="secondary" onClick={() => { setSelectedPath(null); setFileContent(null) }}>{t('common.close')}</Button>
          </div>
        </div>
      </Card>

      {/* Banned IPs Section */}
      <Card>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Shield className="h-5 w-5 text-red-400" />
              <h3 className="text-md font-semibold">{t('crowdsecConfig.bannedIps.title')}</h3>
            </div>
            <Button
              data-testid="ban-ip-trigger"
              onClick={() => setShowBanModal(true)}
              disabled={status.crowdsec.mode === 'disabled'}
              size="sm"
            >
              <ShieldOff className="h-4 w-4 mr-1" />
              {t('crowdsecConfig.bannedIps.banIp')}
            </Button>
          </div>

          {status.crowdsec.mode === 'disabled' ? (
            <p className="text-sm text-gray-500">{t('crowdsecConfig.bannedIps.enableFirst')}</p>
          ) : decisionsQuery.isLoading ? (
            <p className="text-sm text-gray-400">{t('crowdsecConfig.bannedIps.loading')}</p>
          ) : decisionsQuery.error ? (
            <p className="text-sm text-red-400">{t('crowdsecConfig.bannedIps.loadFailed')}</p>
          ) : !decisionsQuery.data?.decisions?.length ? (
            <p className="text-sm text-gray-500">{t('crowdsecConfig.bannedIps.none')}</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-700">
                    <th className="text-left py-2 px-3 text-gray-400 font-medium">{t('crowdsecConfig.bannedIps.ip')}</th>
                    <th className="text-left py-2 px-3 text-gray-400 font-medium">{t('crowdsecConfig.bannedIps.reason')}</th>
                    <th className="text-left py-2 px-3 text-gray-400 font-medium">{t('crowdsecConfig.bannedIps.duration')}</th>
                    <th className="text-left py-2 px-3 text-gray-400 font-medium">{t('crowdsecConfig.bannedIps.bannedAt')}</th>
                    <th className="text-left py-2 px-3 text-gray-400 font-medium">{t('crowdsecConfig.bannedIps.source')}</th>
                    <th className="text-right py-2 px-3 text-gray-400 font-medium">{t('crowdsecConfig.bannedIps.actions')}</th>
                  </tr>
                </thead>
                <tbody>
                  {decisionsQuery.data.decisions.map((decision) => (
                    <tr
                      key={decision.id}
                      className="border-b border-gray-800 hover:bg-gray-800/50 focus:outline-none focus-visible:bg-gray-800 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500"
                      tabIndex={0}
                    >
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
                          {t('crowdsecConfig.bannedIps.unban')}
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

      </TabsContent>
      </Tabs>
      </div>

      {/* Ban IP Modal */}
      {showBanModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center"
          role="dialog"
          aria-modal="true"
          aria-labelledby="ban-modal-title"
        >
          {/* Layer 1: Background overlay (z-40) */}
          <div
            className="absolute inset-0 bg-black/60"
            onClick={() => setShowBanModal(false)}
            role="button"
            tabIndex={-1}
            aria-label={t('common.close')}
          />

          {/* Layer 3: Form content (pointer-events-auto) */}
          <div
            className="relative bg-dark-card rounded-lg p-6 w-[480px] max-w-full shadow-xl"
            onKeyDown={(e) => {
              if (e.key === 'Escape') setShowBanModal(false)
              if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) banMutation.mutate()
            }}
          >
            <h3 id="ban-modal-title" className="text-xl font-semibold text-white mb-4 flex items-center gap-2">
              <ShieldOff className="h-5 w-5 text-red-400" />
              {t('crowdsecConfig.banModal.title')}
            </h3>
            <div className="space-y-4">
              <Input
                id="ban-ip"
                label={t('crowdsecConfig.banModal.ipLabel')}
                placeholder="192.168.1.100"
                value={banForm.ip}
                onChange={(e) => setBanForm({ ...banForm, ip: e.target.value })}
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    if (banForm.ip.trim()) banMutation.mutate()
                  }
                }}
              />
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-1.5" htmlFor="ban-duration">{t('crowdsecConfig.banModal.durationLabel')}</label>
                <select
                  id="ban-duration"
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                  value={banForm.duration}
                  onChange={(e) => setBanForm({ ...banForm, duration: e.target.value })}
                  aria-label={t('crowdsecConfig.banModal.durationLabel')}
                >
                  <option value="1h">{t('crowdsecConfig.banModal.duration1h')}</option>
                  <option value="4h">{t('crowdsecConfig.banModal.duration4h')}</option>
                  <option value="24h">{t('crowdsecConfig.banModal.duration24h')}</option>
                  <option value="7d">{t('crowdsecConfig.banModal.duration7d')}</option>
                  <option value="30d">{t('crowdsecConfig.banModal.duration30d')}</option>
                  <option value="permanent">{t('crowdsecConfig.banModal.durationPermanent')}</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-1.5" htmlFor="ban-reason">{t('crowdsecConfig.banModal.reasonLabel')}</label>
                <textarea
                  id="ban-reason"
                  placeholder={t('crowdsecConfig.banModal.reasonPlaceholder')}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  rows={3}
                  value={banForm.reason}
                  onChange={(e) => setBanForm({ ...banForm, reason: e.target.value })}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault()
                      if (banForm.ip.trim()) banMutation.mutate()
                    }
                  }}
                  aria-label={t('crowdsecConfig.banModal.reasonLabel')}
                />
              </div>
            </div>
            <div className="flex gap-3 justify-end mt-6">
              <Button variant="secondary" onClick={() => setShowBanModal(false)}>
                {t('common.cancel')}
              </Button>
              <Button
                variant="danger"
                onClick={() => banMutation.mutate()}
                isLoading={banMutation.isPending}
                disabled={!banForm.ip.trim()}
              >
                {t('crowdsecConfig.banModal.submit')}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Unban Confirmation Modal */}
      {confirmUnban && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center"
          role="dialog"
          aria-modal="true"
          aria-labelledby="unban-modal-title"
        >
          <div
            className="absolute inset-0 bg-black/60"
            onClick={() => setConfirmUnban(null)}
            role="button"
            tabIndex={-1}
            aria-label={t('common.close')}
          />
          <div
            className="relative bg-dark-card rounded-lg p-6 w-[400px] max-w-full shadow-xl"
            onKeyDown={(e) => {
              if (e.key === 'Escape') setConfirmUnban(null)
              if (e.key === 'Enter') unbanMutation.mutate(confirmUnban.ip)
            }}
          >
            <h3 id="unban-modal-title" className="text-xl font-semibold text-white mb-4">{t('crowdsecConfig.unbanModal.title')}</h3>
            <p className="text-gray-300 mb-6">
              {t('crowdsecConfig.unbanModal.confirm')} <span className="font-mono text-white">{confirmUnban.ip}</span>?
            </p>
            <div className="flex gap-3 justify-end">
              <Button
                variant="secondary"
                onClick={() => setConfirmUnban(null)}
                autoFocus
              >
                {t('common.cancel')}
              </Button>
              <Button
                variant="danger"
                onClick={() => unbanMutation.mutate(confirmUnban.ip)}
                isLoading={unbanMutation.isPending}
              >
                {t('crowdsecConfig.unbanModal.submit')}
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
