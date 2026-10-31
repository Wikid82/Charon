import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import { Shield, ShieldOff, Trash2, Search, AlertTriangle, ExternalLink } from 'lucide-react'
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, Link } from 'react-router'

import { createBackup } from '../api/backups'
import { exportCrowdsecConfig, importCrowdsecConfig, listCrowdsecFiles, readCrowdsecFile, writeCrowdsecFile, listCrowdsecDecisions, banIP, unbanIP, type CrowdSecDecision, type CrowdSecWhitelistEntry, statusCrowdsec, type CrowdSecStatus, startCrowdsec, type ImportHubItems, getCrowdsecImportError, getCrowdsecBanError, getCrowdsecExportError, type CrowdsecBanErrorCode } from '../api/crowdsec'
import { crowdsecBusyMessage, crowdsecErrorMessage } from '../api/errorBody'
import { getFeatureFlags } from '../api/featureFlags'
import {
  listCrowdsecPresets,
  pullCrowdsecPreset,
  applyCrowdsecPreset,
  getCrowdsecPresetCache,
  getCrowdsecPresetAmbiguousTypes,
  getCrowdsecPresetErrorCode,
  getCrowdsecPresetTaintedItems,
  PRESET_ERROR_CODES,
  type CrowdsecPresetErrorCode,
} from '../api/presets'
import { getMyIP } from '../api/system'
import { CrowdSecBouncerKeyDisplay } from '../components/CrowdSecBouncerKeyDisplay'
import { ListErrorNote } from '../components/crowdsec/ListErrorNote'
import HubItemsRestorePanel from '../components/crowdsec/HubItemsRestorePanel'
import { PresetInstallProgress } from '../components/crowdsec/PresetInstallProgress'
import { ConfigReloadOverlay } from '../components/LoadingStates'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { Skeleton } from '../components/ui/Skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/Tabs'
import { CROWDSEC_PRESETS, type CrowdsecPreset } from '../data/crowdsecPresets'
import { useConsoleStatus, useEnrollConsole, useClearConsoleEnrollment } from '../hooks/useConsoleEnrollment'
import { useCrowdsecApplyJob } from '../hooks/useCrowdsecApplyJob'
import { useWhitelistEntries, useAddWhitelist, useDeleteWhitelist, isWhitelistFieldError, whitelistAddErrorMessage } from '../hooks/useCrowdSecWhitelist'
import { SECURITY_STATUS_QUERY_KEY, useSecurityStatus } from '../hooks/useSecurity'
import { buildCrowdsecExportFilename, downloadCrowdsecExport, promptCrowdsecFilename } from '../utils/crowdsecExport'
import { sanitizeKeptFileName } from '../utils/keptFileNames'
import { sanitizeSecret } from '../utils/sanitizeSecret'
import { toast } from '../utils/toast'

/** Identity of a hub item in the preset list: its section and name. */
function presetKey(preset: { type?: string; slug: string }): string {
  return preset.type ? `${preset.type}/${preset.slug}` : preset.slug
}

/**
 * A hub-backed row that is not cached and cannot be fetched because the hub is offline.
 * It stays listed (muted) but is never selected, so no pull is sent for it.
 */
function isPresetUnavailableOffline(preset: { requiresHub: boolean; available?: boolean; cached?: boolean }): boolean {
  return preset.requiresHub && preset.available === false && !preset.cached
}

function isPresetSelectable(preset: { requiresHub: boolean; available?: boolean; cached?: boolean }): boolean {
  return !isPresetUnavailableOffline(preset)
}

/** Prefer the server's `error` message for Axios failures, falling back to the given text. */
function getServerErrorMessage(err: unknown, fallback: string): string {
  if (isAxiosError(err)) {
    const serverError = (err.response?.data as { error?: string } | undefined)?.error
    return serverError || err.message || fallback
  }
  return fallback
}

const MAX_LISTED_NAMES = 10
const OMITTED_NAMES_SHOWN = 5
const WORD_SEPARATOR = ' '

const CrowdSecDashboard = lazy(() => import('../components/crowdsec/CrowdSecDashboard'))

export default function CrowdSecConfig() {
  const { t } = useTranslation()
  const { data: status, isLoading, error } = useSecurityStatus()
  const [searchQuery, setSearchQuery] = useState('')
  const [sortBy, setSortBy] = useState<'alpha' | 'type' | 'source'>('alpha')
  const [file, setFile] = useState<File | null>(null)
  const [omittedExport, setOmittedExport] = useState<{ names: string[]; more: number } | null>(null)
  // Hub items the last import reported as missing; the key restarts the panel per import.
  const [importHubItems, setImportHubItems] = useState<{ key: number; items: ImportHubItems } | null>(null)
  const [keptFiles, setKeptFiles] = useState<string[]>([])
  // Passive import notes: Charon's own allow-list file was rewritten, and other allow-list files were found.
  const [whitelistRegenerated, setWhitelistRegenerated] = useState(false)
  const [otherWhitelistFiles, setOtherWhitelistFiles] = useState<string[]>([])
  const [needsValues, setNeedsValues] = useState<string[]>([])
  const [selectedPath, setSelectedPath] = useState<string | null>(null)
  const [fileContent, setFileContent] = useState<string | null>(null)
  // A selection is a hub item: the same name can exist in several sections, so the section is part of the key.
  const [selectedPresetKey, setSelectedPresetKey] = useState<string>('')
  // Sections offered when the server cannot tell which item a name means; chosenType is the user's pick.
  const [ambiguousTypes, setAmbiguousTypes] = useState<string[]>([])
  const [chosenType, setChosenType] = useState('')
  const [showBanModal, setShowBanModal] = useState(false)
  const [banForm, setBanForm] = useState({ ip: '', duration: '24h', reason: '' })
  const [banFieldErrors, setBanFieldErrors] = useState<{ ip?: string; duration?: string; reason?: string }>({})
  const [confirmUnban, setConfirmUnban] = useState<CrowdSecDecision | null>(null)
  const [whitelistForm, setWhitelistForm] = useState<{ ip_or_cidr: string; reason: string }>({ ip_or_cidr: '', reason: '' })
  const [confirmDeleteWhitelist, setConfirmDeleteWhitelist] = useState<CrowdSecWhitelistEntry | null>(null)
  const [isApplyingPreset, setIsApplyingPreset] = useState(false)
  const [presetPreview, setPresetPreview] = useState<string>('')
  const [presetMeta, setPresetMeta] = useState<{ slug?: string; type?: string; cacheKey?: string; itemSha256?: string; etag?: string; retrievedAt?: string; source?: string } | null>(null)
  const [presetStatusMessage, setPresetStatusMessage] = useState<string | null>(null)
  const [hubUnavailable, setHubUnavailable] = useState(false)
  const [validationError, setValidationError] = useState<string | null>(null)
  const [applyInfo, setApplyInfo] = useState<{ status?: string; backup?: string; reloadHint?: boolean; reloadPending?: boolean; usedCscli?: boolean; cacheKey?: string; warnings?: string[] } | null>(null)
  const queryClient = useQueryClient()
  const applyJob = useCrowdsecApplyJob({ enabled: !!status?.crowdsec })
  const backupCreatedMessage = (name: string) =>
    t('crowdsecConfig.presets.backupCreated', { name, location: t('crowdsecConfig.presets.backupLocation') })
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
  // Tracks manual edits so late-arriving enrollment status never overwrites what the user typed
  const consoleAgentNameEdited = useRef(false)
  const consoleTenantEdited = useRef(false)
  const [consoleAck, setConsoleAck] = useState(false)
  const [consoleErrors, setConsoleErrors] = useState<{ token?: string; agent?: string; ack?: string; submit?: string }>({})
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
    onSuccess: (result) => {
      toast.success(t('crowdsecConfig.importSuccess'))
      const items = result.hub_items
      setImportHubItems(items && items.missing.length > 0 ? { key: Date.now(), items } : null)
      setKeptFiles(result.kept_files ?? [])
      setNeedsValues(result.needs_values ?? [])
      setWhitelistRegenerated(result.whitelist_regenerated === true)
      setOtherWhitelistFiles(result.other_whitelist_files ?? [])
      queryClient.invalidateQueries({ queryKey: SECURITY_STATUS_QUERY_KEY })
    },
    onError: (err: unknown) => {
      toast.error(importFailureMessage(err))
    }
  })

  const listMutation = useQuery({ queryKey: ['crowdsec-files'], queryFn: listCrowdsecFiles })
  const readMutation = useMutation({ mutationFn: (path: string) => readCrowdsecFile(path), onSuccess: (data) => setFileContent(data.content) })
  const writeMutation = useMutation({
    mutationFn: async ({ path, content }: { path: string; content: string }) => writeCrowdsecFile(path, content),
    onSuccess: () => {
      toast.success(t('crowdsecConfig.toasts.fileSaved'))
      queryClient.invalidateQueries({ queryKey: ['crowdsec-files'] })
    },
    onError: (err: unknown) => {
      toast.error(crowdsecErrorMessage(err, t, getServerErrorMessage(err, t('crowdsecConfig.toasts.saveFailed'))))
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
    type?: string
  }

  const presetCatalog: PresetCatalogEntry[] = useMemo(() => {
    const remotePresets = presetsQuery.data?.presets
    const localMap = new Map(CROWDSEC_PRESETS.map((preset) => [preset.slug, preset]))
    if (remotePresets?.length) {
      return remotePresets.map((preset) => {
        const local = localMap.get(preset.slug)
        return {
          slug: preset.slug,
          type: preset.type,
          title: preset.title || local?.title || preset.slug,
          description: local?.description || preset.summary,
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
    // Only show the static fallback once the server list failed; otherwise a placeholder
    // entry would be auto-selected and pulled while the real list is still loading.
    if (!presetsQuery.isError) return []
    return CROWDSEC_PRESETS.map((preset) => ({ ...preset, requiresHub: false, available: true, cached: false, source: 'charon-curated' }))
  }, [presetsQuery.data, presetsQuery.isError])

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

  // Keep the current selection while it is still listed and usable; otherwise select the first usable row
  // in catalog order, or nothing when no row can be used (the empty key never matches a row).
  useEffect(() => {
    if (!presetCatalog.length) return
    const keepsSelection = presetCatalog.some((preset) => presetKey(preset) === selectedPresetKey && isPresetSelectable(preset))
    if (keepsSelection) return
    const firstSelectable = presetCatalog.find(isPresetSelectable)
    setSelectedPresetKey(firstSelectable ? presetKey(firstSelectable) : '')
  }, [presetCatalog, selectedPresetKey])

  const selectPreset = (preset: PresetCatalogEntry) => {
    if (isPresetSelectable(preset)) setSelectedPresetKey(presetKey(preset))
  }

  // Prefer the enrolled identity over the hostname default, unless the user already edited the field
  useEffect(() => {
    const enrolledAgent = consoleStatusQuery.data?.agent_name
    if (enrolledAgent && !consoleAgentNameEdited.current) {
      setConsoleAgentName(enrolledAgent)
    }
    const enrolledTenant = consoleStatusQuery.data?.tenant
    if (enrolledTenant && !consoleTenantEdited.current) {
      setConsoleTenant(enrolledTenant)
    }
  }, [consoleStatusQuery.data?.agent_name, consoleStatusQuery.data?.tenant])

  const handleConsoleAgentNameChange = (value: string) => {
    consoleAgentNameEdited.current = true
    setConsoleAgentName(value)
  }

  const handleConsoleTenantChange = (value: string) => {
    consoleTenantEdited.current = true
    setConsoleTenant(value)
  }

  const selectedPreset = presetCatalog.find((preset) => presetKey(preset) === selectedPresetKey)
  const selectedType = selectedPreset?.type || chosenType || undefined
  const selectedPresetRequiresHub = selectedPreset?.requiresHub ?? false

  const pullPresetMutation = useMutation({
    mutationFn: ({ slug, type }: { slug: string; type?: string }) => pullCrowdsecPreset(slug, type),
    onSuccess: (data) => {
      setPresetPreview(data.preview)
      setPresetMeta({ slug: data.slug, type: data.type, cacheKey: data.cache_key, itemSha256: data.item_sha256, etag: data.etag, retrievedAt: data.retrieved_at, source: data.source })
      setPresetStatusMessage(t('crowdsecConfig.presets.status.previewFetched'))
      setHubUnavailable(false)
      setValidationError(null)
      setAmbiguousTypes([])
    },
    onError: (err: unknown) => {
      setPresetStatusMessage(null)
      if (isAxiosError(err)) {
        if (getCrowdsecPresetErrorCode(err) === 'preset_type_ambiguous') {
          setAmbiguousTypes(getCrowdsecPresetAmbiguousTypes(err))
          setPresetStatusMessage(t('crowdsecConfig.presets.status.typeAmbiguous'))
          return
        }
        if (getCrowdsecPresetErrorCode(err) === 'preset_hub_updating') {
          setPresetStatusMessage(t('crowdsecConfig.presets.status.applyHubUpdating'))
          return
        }
        if (err.response?.status === 400) {
          setValidationError(err.response.data?.error || t('crowdsecConfig.presets.status.slugInvalid'))
          return
        }
        if (err.response?.status === 503) {
          setHubUnavailable(true)
          setPresetStatusMessage(t('crowdsecConfig.presets.status.hubUnavailable'))
          return
        }
        setPresetStatusMessage(err.response?.data?.error || t('crowdsecConfig.presets.status.pullFailed'))
      } else {
        setPresetStatusMessage(t('crowdsecConfig.presets.status.pullFailed'))
      }
    },
  })

  useEffect(() => {
    if (!selectedPreset) return
    setValidationError(null)
    setPresetStatusMessage(null)
    setApplyInfo(null)
    setAmbiguousTypes([])
    setChosenType('')
    setPresetMeta({
      cacheKey: selectedPreset.cacheKey,
      etag: selectedPreset.etag,
      retrievedAt: selectedPreset.retrievedAt,
      source: selectedPreset.source,
    })
    setPresetPreview('')
    pullPresetMutation.mutate({ slug: selectedPreset.slug, type: selectedPreset.type })
    // eslint-disable-next-line react-hooks/exhaustive-deps -- Only re-run when slug changes, not on mutation/preset object identity changes
  }, [selectedPresetKey])

  const loadCachedPreview = async () => {
    if (!selectedPreset) return
    try {
      const cached = await getCrowdsecPresetCache(selectedPreset.slug, selectedType)
      setPresetPreview(cached.preview)
      setPresetMeta({ cacheKey: cached.cache_key, etag: cached.etag, retrievedAt: selectedPreset.retrievedAt, source: selectedPreset.source })
      setPresetStatusMessage(t('crowdsecConfig.presets.status.cachedLoaded'))
      setHubUnavailable(false)
    } catch (err) {
      const msg = isAxiosError(err) ? err.response?.data?.error || err.message : t('crowdsecConfig.presets.status.cacheLoadFailed')
      toast.error(msg)
    }
  }

  const normalizedConsoleStatus = consoleStatusQuery.data?.status === 'failed' ? 'degraded' : consoleStatusQuery.data?.status || 'not_enrolled'
  const isConsoleDegraded = normalizedConsoleStatus === 'degraded'
  const isConsolePending = enrollConsoleMutation.isPending || normalizedConsoleStatus === 'enrolling'
  const isConsolePendingAcceptance = normalizedConsoleStatus === 'pending_acceptance'
  const consoleStatusLabel = normalizedConsoleStatus.replace('_', ' ')
  const consoleTokenState = consoleStatusQuery.data ? (consoleStatusQuery.data.key_present ? t('crowdsecConfig.consoleEnrollment.tokenStored') : t('crowdsecConfig.consoleEnrollment.tokenNotStored')) : '—'
  const canRotateKey = normalizedConsoleStatus === 'enrolled' || normalizedConsoleStatus === 'degraded' || isConsolePendingAcceptance
  const consoleDocsHref = 'https://wikid82.github.io/charon/security/'

  const sanitizeErrorMessage = (err: unknown) => {
    if (isAxiosError(err)) {
      return sanitizeSecret(err.response?.data?.error || err.message)
    }
    if (err instanceof Error) return sanitizeSecret(err.message)
    return t('crowdsecConfig.consoleEnrollment.errors.enrollFailed')
  }

  const validateConsoleEnrollment = (options?: { requireAck?: boolean }) => {
    const nextErrors: { token?: string; agent?: string; ack?: string } = {}
    if (!enrollmentToken.trim()) {
      nextErrors.token = t('crowdsecConfig.consoleEnrollment.tokenRequired')
    }
    if (!consoleAgentName.trim()) {
      nextErrors.agent = t('crowdsecConfig.consoleEnrollment.errors.agentRequired')
    }
    if (options?.requireAck && !consoleAck) {
      nextErrors.ack = t('crowdsecConfig.consoleEnrollment.errors.ackRequired')
    }
    setConsoleErrors(nextErrors)
    return Object.keys(nextErrors).length === 0
  }

  const submitConsoleEnrollment = async (force = false) => {
    const requireAck = normalizedConsoleStatus === 'not_enrolled'
    if (!validateConsoleEnrollment({ requireAck })) return
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
          ? t('crowdsecConfig.consoleEnrollment.enrollSubmitted')
          : t('crowdsecConfig.consoleEnrollment.enrollSent')
      )
    } catch (err) {
      const message = crowdsecErrorMessage(err, t, sanitizeErrorMessage(err))
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

  /** Translated text for a refused ban or unban; unknown or missing codes keep the server's text. */
  const banFailureMessage = (err: unknown, fallback: string): string => {
    const messages: Record<CrowdsecBanErrorCode, string> = {
      invalid_ip: t('crowdsecConfig.banErrors.invalidIp'),
      invalid_duration: t('crowdsecConfig.banErrors.invalidDuration'),
      invalid_reason: t('crowdsecConfig.banErrors.invalidReason'),
      decision_not_found: t('crowdsecConfig.banErrors.decisionNotFound'),
      cscli_timeout: t('crowdsecConfig.banErrors.timeout'),
      request_too_large: t('crowdsecConfig.banErrors.requestTooLarge'),
    }
    const code = getCrowdsecBanError(err)
    if (code) return messages[code]
    return err instanceof Error ? err.message : fallback
  }

  const banMutation = useMutation({
    mutationFn: () => {
      setBanFieldErrors({})
      return banIP(banForm.ip, banForm.duration, banForm.reason)
    },
    onSuccess: () => {
      toast.success(t('crowdsecConfig.toasts.ipBanned', { ip: banForm.ip }))
      queryClient.invalidateQueries({ queryKey: ['crowdsec-decisions'] })
      setShowBanModal(false)
      setBanForm({ ip: '', duration: '24h', reason: '' })
    },
    onError: (err: unknown) => {
      const message = banFailureMessage(err, t('crowdsecConfig.toasts.banFailed'))
      // Field problems are shown beside the field so the dialog can be corrected in place.
      switch (getCrowdsecBanError(err)) {
        case 'invalid_ip':
          setBanFieldErrors({ ip: message })
          break
        case 'invalid_duration':
          setBanFieldErrors({ duration: message })
          break
        case 'invalid_reason':
          setBanFieldErrors({ reason: message })
          break
        default:
          toast.error(message)
      }
    },
  })

  const unbanMutation = useMutation({
    mutationFn: (ip: string) => unbanIP(ip),
    onSuccess: (_, ip) => {
      toast.success(t('crowdsecConfig.toasts.ipUnbanned', { ip }))
      queryClient.invalidateQueries({ queryKey: ['crowdsec-decisions'] })
      setConfirmUnban(null)
    },
    onError: (err: unknown) => {
      const message = banFailureMessage(err, t('crowdsecConfig.toasts.unbanFailed'))
      toast.error(message)
    },
  })

  const whitelistQuery = useWhitelistEntries()
  const addWhitelistMutation = useAddWhitelist()
  const deleteWhitelistMutation = useDeleteWhitelist()

  // Only field problems (400/409 duplicate) show inline. Busy, server and network failures toast only.
  const whitelistInlineError = isWhitelistFieldError(addWhitelistMutation.error)
    ? whitelistAddErrorMessage(addWhitelistMutation.error, t)
    : null

  const handleAddMyIP = async () => {
    try {
      const result = await getMyIP()
      setWhitelistForm((prev) => ({ ...prev, ip_or_cidr: result.ip }))
    } catch {
      toast.error(t('crowdsecConfig.toasts.ipDetectFailed'))
    }
  }

  const omittedTotal = omittedExport ? omittedExport.names.length + omittedExport.more : 0

  const handleExport = async () => {
    const defaultName = buildCrowdsecExportFilename()
    const filename = promptCrowdsecFilename(t, defaultName)
    if (!filename) return

    setOmittedExport(null)
    try {
      const { blob, omitted, omittedMore } = await exportCrowdsecConfig()
      downloadCrowdsecExport(blob, filename)
      setOmittedExport({ names: omitted, more: omittedMore })
      toast.success(t('crowdsecConfig.exportSuccess'))
    } catch (err) {
      toast.error(getCrowdsecExportError(err) ? t('crowdsecConfig.exportConfigUnsafe') : t('crowdsecConfig.exportFailed'))
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
      // Failures are surfaced by the mutations' onError handlers; success toast only fires on onSuccess.
    }
  }

  /** Comma-separated, sanitised names: the first MAX_LISTED_NAMES, then "and N more". */
  const formatNameList = (names: string[]): string => {
    const listed = names.slice(0, MAX_LISTED_NAMES).map(sanitizeKeptFileName)
    const hidden = names.length - listed.length
    return (hidden > 0 ? [...listed, t('crowdsecConfig.importNeedsValuesMore', { count: hidden })] : listed).join(', ')
  }

  /** Translated text for a refused import; unknown or missing codes keep the server's text. */
  const importFailureMessage = (err: unknown): string => {
    const busy = crowdsecBusyMessage(err, t)
    if (busy) return busy
    const fallback = err instanceof Error ? err.message : t('crowdsecConfig.importFailed')
    const refusal = getCrowdsecImportError(err)
    const names = refusal ? formatNameList(refusal.names) : ''
    switch (refusal?.code) {
      case 'import_flat_archive':
        return t('crowdsecConfig.importErrorFlatArchive')
      case 'import_profile_names_undefined':
        return names ? t('crowdsecConfig.importErrorProfileNamesUndefined', { names }) : fallback
      case 'import_kept_profile_undefined':
        return names ? t('crowdsecConfig.importErrorKeptProfileUndefined', { names }) : fallback
      case 'import_kept_profile_duplicate':
        return t('crowdsecConfig.importErrorKeptProfileDuplicate')
      case 'import_keep_current_failed':
        return t('crowdsecConfig.importErrorKeepCurrentFailed')
      case 'import_config_not_found':
        return t('crowdsecConfig.importErrorConfigNotFound')
      default:
        return fallback
    }
  }

  /** Translated text for an apply outcome reported by the server. */
  const applyOutcomeMessage = (code: CrowdsecPresetErrorCode, editedItems: string[]): string => {
    switch (code) {
      case 'preset_cache_key_required':
      case 'preset_cache_stale':
        return t('crowdsecConfig.presets.status.applyStale')
      case 'preset_hub_updating':
        return t('crowdsecConfig.presets.status.applyHubUpdating')
      case 'preset_local_modified':
        return editedItems.length > 0
          ? [
              t('crowdsecConfig.presets.status.applyLocalModified'),
              t('crowdsecConfig.presets.status.editedItems', { items: formatNameList(editedItems) }),
            ].join(WORD_SEPARATOR)
          : t('crowdsecConfig.presets.status.applyLocalModified')
      case 'preset_apply_timeout':
        return t('crowdsecConfig.presets.status.applyTimeout')
      case 'preset_type_ambiguous':
        return t('crowdsecConfig.presets.status.typeAmbiguous')
      case 'hub_unreachable':
        return t('crowdsecConfig.presets.status.applyHubUnreachable')
      case 'cscli_unavailable':
        return t('crowdsecConfig.presets.status.cscliUnavailable')
    }
  }

  /** Translated reason for a failed install job; null when the server named none we know. */
  const jobFailureText = (code: string | undefined): string | null => {
    const known = PRESET_ERROR_CODES.find((candidate) => candidate === code)
    return known ? applyOutcomeMessage(known, []) : null
  }

  const handleApplyPreset = async () => {
    if (!selectedPreset) {
      toast.error(t('crowdsecConfig.toasts.selectPreset'))
      return
    }

    setIsApplyingPreset(true)
    setApplyInfo(null)
    setValidationError(null)
    setPresetStatusMessage(null)
    applyJob.dismiss()
    try {
      // Apply the exact entry that was previewed: the canonical slug from pull and its cache key.
      const request = { slug: presetMeta?.slug ?? selectedPreset.slug, type: presetMeta?.type ?? selectedType, cache_key: presetMeta?.cacheKey, item_sha256: presetMeta?.itemSha256 || undefined }
      if (selectedPresetRequiresHub) {
        // Hub installs can take minutes: the server runs them as a background job shown in the progress row.
        await applyJob.start(request)
        return
      }
      const res = await applyCrowdsecPreset(request)
      setApplyInfo({
        status: res.status,
        backup: res.backup,
        reloadHint: res.reload_hint,
        reloadPending: res.reload_pending,
        usedCscli: res.used_cscli,
        cacheKey: res.cache_key,
        warnings: res.warnings,
      })

      if (res.status !== 'applied') {
        toast.error(t('crowdsecConfig.toasts.presetNotCompleted', { status: res.status || t('crowdsecConfig.toasts.statusUnknown') }))
        return
      }

      // A queued reload is not a pending user action, so it does not use the "reload required" toast.
      toast.success(t(res.reload_hint && !res.reload_pending ? 'crowdsecConfig.toasts.presetAppliedReload' : 'crowdsecConfig.toasts.presetApplied'))
      if (res.backup) {
        setPresetStatusMessage(backupCreatedMessage(res.backup))
      }
    } catch (err) {
      if (!isAxiosError(err)) {
        toast.error(t('crowdsecConfig.toasts.presetApplyFailed'))
        return
      }
      const busy = crowdsecBusyMessage(err, t)
      if (busy) {
        setPresetStatusMessage(busy)
        return
      }
      // The server is authoritative for every preset source: surface its message for any
      // non-2xx response and never write preset content client-side.
      const outcomeCode = getCrowdsecPresetErrorCode(err)
      if (outcomeCode) {
        // The stored copy is gone or out of date: drop the preview so the user pulls it again.
        if (outcomeCode === 'preset_cache_key_required' || outcomeCode === 'preset_cache_stale' || outcomeCode === 'preset_hub_updating') {
          setPresetMeta(null)
          setPresetPreview('')
        }
        setPresetStatusMessage(applyOutcomeMessage(outcomeCode, getCrowdsecPresetTaintedItems(err)))
        setApplyInfo({ status: 'failed' })
        return
      }
      const status = err.response?.status
      const backupName = (err.response?.data as { backup?: string } | undefined)?.backup
      const message = getServerErrorMessage(err, t('crowdsecConfig.toasts.presetApplyFailed'))
      if (status === 400) {
        setValidationError(message)
      }
      if (status === 503) {
        setHubUnavailable(true)
      }
      setApplyInfo({ status: 'failed', backup: backupName || undefined, cacheKey: presetMeta?.cacheKey })
      toast.error(
        backupName
          ? t('crowdsecConfig.toasts.applyFailedWithBackup', { message, backup: backupCreatedMessage(backupName) })
          : t('crowdsecConfig.toasts.applyFailed', { message })
      )
    } finally {
      setIsApplyingPreset(false)
    }
  }

  const installJob = applyJob.job

  const presetActionDisabled =
    !selectedPreset ||
    isApplyingPreset ||
    applyJob.isActive ||
    backupMutation.isPending ||
    pullPresetMutation.isPending ||
    (selectedPresetRequiresHub && (hubUnavailable || !presetMeta?.cacheKey))

  // Determine if any operation is in progress
  const isApplyingConfig =
    importMutation.isPending ||
    writeMutation.isPending ||
    backupMutation.isPending ||
    pullPresetMutation.isPending ||
    isApplyingPreset ||
    banMutation.isPending ||
    unbanMutation.isPending ||
    addWhitelistMutation.isPending ||
    deleteWhitelistMutation.isPending

  // Determine contextual message
  const getMessage = () => {
    if (pullPresetMutation.isPending) {
      return { message: t('crowdsecConfig.overlay.fetchingPreset'), submessage: t('crowdsecConfig.overlay.fetchingPresetDetail') }
    }
    if (isApplyingPreset) {
      return { message: t('crowdsecConfig.overlay.loadingPreset'), submessage: t('crowdsecConfig.overlay.loadingPresetDetail') }
    }
    if (importMutation.isPending) {
      return { message: t('crowdsecConfig.overlay.importing'), submessage: t('crowdsecConfig.overlay.importingDetail') }
    }
    if (writeMutation.isPending) {
      return { message: t('crowdsecConfig.overlay.saving'), submessage: t('crowdsecConfig.overlay.savingDetail') }
    }
    if (banMutation.isPending) {
      return { message: t('crowdsecConfig.overlay.banning'), submessage: t('crowdsecConfig.overlay.banningDetail') }
    }
    if (unbanMutation.isPending) {
      return { message: t('crowdsecConfig.overlay.unbanning'), submessage: t('crowdsecConfig.overlay.unbanningDetail') }
    }
    if (addWhitelistMutation.isPending) {
      return { message: t('crowdsecConfig.overlay.updatingList'), submessage: t('crowdsecConfig.overlay.whitelistAddingDetail') }
    }
    if (deleteWhitelistMutation.isPending) {
      return { message: t('crowdsecConfig.overlay.updatingList'), submessage: t('crowdsecConfig.overlay.whitelistRemovingDetail') }
    }
    return { message: t('crowdsecConfig.overlay.default'), submessage: t('crowdsecConfig.overlay.defaultDetail') }
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
          {isLocalMode && <TabsTrigger value="whitelist">{t('crowdsecConfig.whitelist.tabLabel', 'Whitelist')}</TabsTrigger>}
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
          <Link to="/security" className="text-blue-400 hover:text-blue-300 underline">{t('navigation.cerberus')}</Link>.
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
                      {t('crowdsecConfig.checkNow')}
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
                        } catch (err) {
                          toast.error(crowdsecErrorMessage(err, t, t('crowdsecConfig.startFailed')));
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
                      {t('crowdsecConfig.checkNow')}
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
                onChange={(e) => handleConsoleAgentNameChange(e.target.value)}
                error={consoleErrors.agent}
                errorTestId="console-enroll-error"
                data-testid="console-agent-name"
              />
              <Input
                id="console-tenant"
                label={t('crowdsecConfig.consoleEnrollment.tenant')}
                value={consoleTenant}
                onChange={(e) => handleConsoleTenantChange(e.target.value)}
                helperText={t('crowdsecConfig.consoleEnrollment.tenantHelper')}
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
                            onChange={(e) => handleConsoleAgentNameChange(e.target.value)}
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
                            onChange={(e) => handleConsoleTenantChange(e.target.value)}
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
          {omittedTotal > 0 && omittedExport && (
            <p className="text-sm text-gray-400" data-testid="export-omitted-note">
              {t('crowdsecConfig.packages.omittedNote', {
                count: omittedTotal,
                names: omittedExport.names.slice(0, OMITTED_NAMES_SHOWN).join(', '),
              })}
              {omittedTotal > OMITTED_NAMES_SHOWN &&
                t('crowdsecConfig.packages.omittedMore', { count: omittedTotal - OMITTED_NAMES_SHOWN })}
            </p>
          )}
          <input
            type="file"
            onChange={(e) => {
              setFile(e.target.files?.[0] ?? null)
              setImportHubItems(null)
              setKeptFiles([])
              setNeedsValues([])
              setWhitelistRegenerated(false)
              setOtherWhitelistFiles([])
            }}
            data-testid="import-file"
            accept=".tar.gz"
            aria-label={t('crowdsecConfig.packages.selectFile')}
          />
          {keptFiles.length > 0 && (
            <p className="text-sm text-gray-400" data-testid="import-kept-files">
              {t('crowdsecConfig.importKeptFiles', { files: keptFiles.map(sanitizeKeptFileName).join(', ') })}
            </p>
          )}
          {needsValues.length > 0 && (
            <p className="text-sm text-gray-400" data-testid="import-needs-values">
              {t('crowdsecConfig.importNeedsValues', { files: formatNameList(needsValues) })}
            </p>
          )}
          {whitelistRegenerated && (
            <p className="text-sm text-gray-400" data-testid="import-whitelist-regenerated">
              {t('crowdsecConfig.import.whitelistRegenerated')}
            </p>
          )}
          {otherWhitelistFiles.length > 0 && (
            <p className="text-sm text-gray-400" data-testid="import-other-whitelist-files">
              {t('crowdsecConfig.import.otherWhitelistFiles', {
                count: otherWhitelistFiles.length,
                files: otherWhitelistFiles.map(sanitizeKeptFileName).join(', '),
              })}
            </p>
          )}
          {importHubItems && <HubItemsRestorePanel key={importHubItems.key} hubItems={importHubItems.items} />}
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
              aria-label={t('crowdsecConfig.presets.sortBy')}
              onChange={(e) => setSortBy(e.target.value as 'alpha' | 'type' | 'source')}
              className="bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white"
            >
              <option value="alpha">{t('crowdsecConfig.presets.sortAlpha')}</option>
              <option value="source">{t('crowdsecConfig.presets.sortSource')}</option>
            </select>
          </div>

          <div className="border border-gray-700 rounded-lg max-h-60 overflow-y-auto bg-gray-900">
            {filteredPresets.length > 0 ? (
              filteredPresets.map((preset) => {
                const selected = selectedPresetKey === presetKey(preset)
                const selectable = isPresetSelectable(preset)
                return (
                  <div
                    key={presetKey(preset)}
                    onClick={() => selectPreset(preset)}
                    role="button"
                    tabIndex={0}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        selectPreset(preset)
                      }
                    }}
                    aria-pressed={selected}
                    aria-disabled={selectable ? undefined : true}
                    className={`p-3 border-b border-gray-800 last:border-0 outline-none focus-visible:bg-gray-800 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 ${
                      selectable ? 'cursor-pointer hover:bg-gray-800' : 'cursor-not-allowed opacity-60'
                    } ${
                      selected ? 'bg-blue-900/20 border-l-2 border-l-blue-500' : 'border-l-2 border-l-transparent'
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
                    {preset.type && <p className="text-xs text-gray-500" data-testid="preset-type">{preset.type}</p>}
                    <p className="text-xs text-gray-400 line-clamp-1">{preset.description}</p>
                    {!selectable && (
                      <p className="text-xs text-gray-500" data-testid="preset-unavailable-offline">{t('crowdsecConfig.presets.unavailableOffline')}</p>
                    )}
                  </div>
                )
              })
            ) : (
              <div className="p-4 text-center text-gray-500 text-sm">{t('crowdsecConfig.presets.noPresets', { query: searchQuery })}</div>
            )}
          </div>

          {presetsQuery.data?.hub_offline && (
            <p className="text-xs text-gray-500" data-testid="preset-hub-offline">{t('crowdsecConfig.presets.hubOffline')}</p>
          )}

          <div className="flex items-center gap-2 flex-wrap justify-end">
            <Button
              variant="secondary"
              onClick={() => selectedPreset && pullPresetMutation.mutate({ slug: selectedPreset.slug, type: selectedType })}
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

          {installJob && (
            <PresetInstallProgress
              job={installJob}
              isWaitingForServer={applyJob.isWaitingForServer}
              isCancelling={applyJob.isCancelling}
              cancelFailed={applyJob.cancelFailed}
              onCancel={() => applyJob.cancel(installJob.job_id)}
              failureText={jobFailureText(installJob.error_code)}
            />
          )}

          {ambiguousTypes.length > 0 && selectedPreset && (
            <div className="flex flex-wrap gap-2 items-center text-sm" data-testid="preset-type-picker">
              <label htmlFor="preset-type-select" className="text-gray-300">{t('crowdsecConfig.presets.typeLabel')}</label>
              <select
                id="preset-type-select"
                value={chosenType}
                onChange={(e) => {
                  setChosenType(e.target.value)
                  if (e.target.value) pullPresetMutation.mutate({ slug: selectedPreset.slug, type: e.target.value })
                }}
                className="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-white"
              >
                <option value="">{t('crowdsecConfig.presets.typePlaceholder')}</option>
                {ambiguousTypes.map((type) => (
                  <option key={type} value={type}>{type}</option>
                ))}
              </select>
            </div>
          )}

          {hubUnavailable && (
            <div className="flex flex-wrap gap-2 items-center text-sm text-red-300" data-testid="preset-hub-unavailable">
              <span>{t('crowdsecConfig.presets.hubUnreachable')}</span>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => selectedPreset && pullPresetMutation.mutate({ slug: selectedPreset.slug, type: selectedType })}
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
                  {presetPreview || t('crowdsecConfig.presets.previewUnavailable')}
                </pre>
              </div>

              {applyInfo && (
                <div className="rounded-lg border border-gray-800 bg-gray-900/70 p-3 text-xs text-gray-200" data-testid="preset-apply-info">
                  <p>{t('common.status')}: {applyInfo.status || t('crowdsecConfig.presets.applied')}</p>
                  {applyInfo.backup && <p>{t('crowdsecConfig.presets.backup')}: {applyInfo.backup} ({t('crowdsecConfig.presets.backupLocation')})</p>}
                  {applyInfo.reloadPending && <p data-testid="preset-reload-pending">{t('crowdsecConfig.reloadPending')}</p>}
                  {applyInfo.reloadHint && !applyInfo.reloadPending && <p>{t('crowdsecConfig.presets.reload')}: {t('crowdsecConfig.presets.required')}</p>}
                  {applyInfo.warnings && applyInfo.warnings.length > 0 && (
                    <div className="mt-1 text-yellow-300" data-testid="preset-apply-warnings">
                      <p>{t('crowdsecConfig.presets.status.applyDependencyNote')}</p>
                      <ul className="list-disc pl-4">
                        {applyInfo.warnings.map((warning) => (
                          <li key={warning}>{warning}</li>
                        ))}
                      </ul>
                    </div>
                  )}
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
            aria-label={t('crowdsecConfig.files.content')}
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
              onClick={() => {
                setBanFieldErrors({})
                setShowBanModal(true)
              }}
              disabled={status.crowdsec.mode === 'disabled'}
              size="sm"
            >
              <ShieldOff className="h-4 w-4 mr-1" />
              {t('crowdsecConfig.bannedIps.banIp')}
            </Button>
          </div>

          {status.crowdsec.mode !== 'disabled' && (
            <ListErrorNote error={decisionsQuery.data?.error} error_code={decisionsQuery.data?.error_code} />
          )}

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

      {isLocalMode && (
        <TabsContent value="whitelist" className="mt-4">
          <Card>
            <div className="space-y-4">
              <div className="flex items-center gap-2">
                <Shield className="h-5 w-5 text-green-400" />
                <h3 className="text-md font-semibold">{t('crowdsecConfig.whitelist.title', 'IP Whitelist')}</h3>
              </div>
              <p className="text-sm text-gray-400">
                {t('crowdsecConfig.whitelist.description', 'Whitelisted IPs and CIDRs are never blocked by CrowdSec, even if they trigger alerts.')}
              </p>

              {/* Add entry form */}
              <div className="flex flex-wrap gap-3 items-end">
                <div className="flex-1 min-w-[180px]">
                  <Input
                    id="whitelist-ip"
                    label={t('crowdsecConfig.whitelist.ipLabel', 'IP or CIDR')}
                    placeholder="192.168.1.1 or 10.0.0.0/8"
                    value={whitelistForm.ip_or_cidr}
                    onChange={(e) => {
                      setWhitelistForm((prev) => ({ ...prev, ip_or_cidr: e.target.value }))
                      if (addWhitelistMutation.error) addWhitelistMutation.reset()
                    }}
                    error={whitelistInlineError ?? undefined}
                    errorTestId="whitelist-ip-error"
                    aria-required={true}
                    data-testid="whitelist-ip-input"
                  />
                </div>
                <div className="flex-1 min-w-[180px]">
                  <Input
                    id="whitelist-reason"
                    label={t('crowdsecConfig.whitelist.reasonLabel', 'Reason')}
                    placeholder={t('crowdsecConfig.whitelist.reasonPlaceholder', 'Optional reason')}
                    value={whitelistForm.reason}
                    onChange={(e) => setWhitelistForm((prev) => ({ ...prev, reason: e.target.value }))}
                    data-testid="whitelist-reason-input"
                  />
                </div>
                <div className="flex gap-2 pb-0.5">
                  <Button
                    variant="secondary"
                    size="sm"
                    type="button"
                    onClick={handleAddMyIP}
                    data-testid="whitelist-add-my-ip-btn"
                  >
                    {t('crowdsecConfig.whitelist.addMyIp', 'Add My IP')}
                  </Button>
                  <Button
                    size="sm"
                    type="button"
                    onClick={() => {
                      addWhitelistMutation.mutate(whitelistForm, {
                        onSuccess: () => setWhitelistForm({ ip_or_cidr: '', reason: '' }),
                      })
                    }}
                    disabled={!whitelistForm.ip_or_cidr.trim() || addWhitelistMutation.isPending}
                    isLoading={addWhitelistMutation.isPending}
                    data-testid="whitelist-add-btn"
                  >
                    {t('common.add', 'Add')}
                  </Button>
                </div>
              </div>

              {/* Entries table */}
              {whitelistQuery.isLoading ? (
                <div className="space-y-2">
                  <Skeleton className="h-8 w-full" />
                  <Skeleton className="h-8 w-full" />
                  <Skeleton className="h-8 w-full" />
                </div>
              ) : !whitelistQuery.data?.length ? (
                <p className="text-sm text-gray-500" data-testid="whitelist-empty">
                  {t('crowdsecConfig.whitelist.none', 'No whitelist entries')}
                </p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-gray-700">
                        <th className="text-left py-2 px-3 text-gray-400 font-medium" scope="col">
                          {t('crowdsecConfig.whitelist.columnIp', 'IP / CIDR')}
                        </th>
                        <th className="text-left py-2 px-3 text-gray-400 font-medium" scope="col">
                          {t('crowdsecConfig.whitelist.columnReason', 'Reason')}
                        </th>
                        <th className="text-left py-2 px-3 text-gray-400 font-medium" scope="col">
                          {t('crowdsecConfig.whitelist.columnAdded', 'Added')}
                        </th>
                        <th className="text-right py-2 px-3 text-gray-400 font-medium" scope="col">
                          {t('crowdsecConfig.bannedIps.actions')}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {whitelistQuery.data.map((entry) => (
                        <tr key={entry.uuid} className="border-b border-gray-800 hover:bg-gray-800/50">
                          <td className="py-2 px-3 font-mono text-white">{entry.ip_or_cidr}</td>
                          <td className="py-2 px-3 text-gray-300">{entry.reason || '-'}</td>
                          <td className="py-2 px-3 text-gray-300">
                            {entry.created_at ? new Date(entry.created_at).toLocaleString() : '-'}
                          </td>
                          <td className="py-2 px-3 text-right">
                            <Button
                              variant="danger"
                              size="sm"
                              onClick={() => setConfirmDeleteWhitelist(entry)}
                              aria-label={`${t('crowdsecConfig.whitelist.deleteAriaLabel', 'Remove whitelist entry for')} ${entry.ip_or_cidr}`}
                              data-testid="whitelist-delete-btn"
                            >
                              <Trash2 className="h-3 w-3 mr-1" />
                              {t('crowdsecConfig.whitelist.delete', 'Delete')}
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
      )}

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
                onChange={(e) => {
                  setBanForm({ ...banForm, ip: e.target.value })
                  setBanFieldErrors((prev) => ({ ...prev, ip: undefined }))
                }}
                error={banFieldErrors.ip}
                errorTestId="ban-ip-error"
                helperText={t('crowdsecConfig.banModal.ipHint')}
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
                  onChange={(e) => {
                    setBanForm({ ...banForm, duration: e.target.value })
                    setBanFieldErrors((prev) => ({ ...prev, duration: undefined }))
                  }}
                  aria-label={t('crowdsecConfig.banModal.durationLabel')}
                  aria-describedby={banFieldErrors.duration ? 'ban-duration-error ban-duration-help' : 'ban-duration-help'}
                >
                  <option value="1h">{t('crowdsecConfig.banModal.duration1h')}</option>
                  <option value="4h">{t('crowdsecConfig.banModal.duration4h')}</option>
                  <option value="24h">{t('crowdsecConfig.banModal.duration24h')}</option>
                  <option value="7d">{t('crowdsecConfig.banModal.duration7d')}</option>
                  <option value="30d">{t('crowdsecConfig.banModal.duration30d')}</option>
                  <option value="365d">{t('crowdsecConfig.banModal.duration365d')}</option>
                </select>
                <p id="ban-duration-help" className="mt-1.5 text-xs text-gray-400">{t('crowdsecConfig.banModal.durationHelp')}</p>
                {banFieldErrors.duration && (
                  <p id="ban-duration-error" className="mt-1.5 text-sm text-error" role="alert">{banFieldErrors.duration}</p>
                )}
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-1.5" htmlFor="ban-reason">{t('crowdsecConfig.banModal.reasonLabel')}</label>
                <textarea
                  id="ban-reason"
                  placeholder={t('crowdsecConfig.banModal.reasonPlaceholder')}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  rows={3}
                  value={banForm.reason}
                  onChange={(e) => {
                    setBanForm({ ...banForm, reason: e.target.value })
                    setBanFieldErrors((prev) => ({ ...prev, reason: undefined }))
                  }}
                  aria-describedby={banFieldErrors.reason ? 'ban-reason-error' : undefined}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault()
                      if (banForm.ip.trim()) banMutation.mutate()
                    }
                  }}
                  aria-label={t('crowdsecConfig.banModal.reasonLabel')}
                />
                {banFieldErrors.reason && (
                  <p id="ban-reason-error" className="mt-1.5 text-sm text-error" role="alert">{banFieldErrors.reason}</p>
                )}
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

      {/* Delete Whitelist Entry Modal */}
      {confirmDeleteWhitelist && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center"
          role="dialog"
          aria-modal="true"
          aria-labelledby="whitelist-delete-modal-title"
        >
          <button
            type="button"
            className="absolute inset-0 bg-black/60 focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-white"
            onClick={() => setConfirmDeleteWhitelist(null)}
            aria-label={t('common.close')}
          />
          <div
            className="relative z-10 bg-gray-900 rounded-lg border border-gray-700 p-6 max-w-md w-full mx-4 shadow-xl"
            onKeyDown={(e) => {
              if (e.key === 'Escape') setConfirmDeleteWhitelist(null)
            }}
          >
            <h2 id="whitelist-delete-modal-title" className="text-lg font-semibold text-white mb-2">
              {t('crowdsecConfig.whitelist.deleteModal.title', 'Remove Whitelist Entry')}
            </h2>
            <p className="text-sm text-gray-300 mb-4">
              {t('crowdsecConfig.whitelist.deleteModal.body', { ip: confirmDeleteWhitelist.ip_or_cidr })}
            </p>
            <div className="flex justify-end gap-3">
              <Button
                variant="secondary"
                onClick={() => setConfirmDeleteWhitelist(null)}
                autoFocus
              >
                {t('common.cancel', 'Cancel')}
              </Button>
              <Button
                variant="danger"
                onClick={() => deleteWhitelistMutation.mutate(confirmDeleteWhitelist.uuid, {
                  onSuccess: () => setConfirmDeleteWhitelist(null),
                })}
                isLoading={deleteWhitelistMutation.isPending}
              >
                {t('crowdsecConfig.whitelist.deleteModal.submit', 'Remove')}
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
