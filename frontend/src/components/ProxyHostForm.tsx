import { useState, useEffect, useRef, useCallback } from 'react'
import { CircleHelp, AlertCircle, Check, X, Loader2, Copy, Info, AlertTriangle } from 'lucide-react'
import { toast } from 'react-hot-toast'
import type { ProxyHost, ApplicationPreset } from '../api/proxyHosts'
import { testProxyHostConnection } from '../api/proxyHosts'
import { syncMonitors } from '../api/uptime'
import { useRemoteServers } from '../hooks/useRemoteServers'
import { useDomains } from '../hooks/useDomains'
import { useCertificates } from '../hooks/useCertificates'
import { useDocker } from '../hooks/useDocker'
import AccessListSelector from './AccessListSelector'
import { useSecurityHeaderProfiles } from '../hooks/useSecurityHeaders'
import { SecurityScoreDisplay } from './SecurityScoreDisplay'
import { parse } from 'tldts'
import { Alert } from './ui/Alert'
import { isLikelyDockerContainerIP, isPrivateOrDockerIP } from '../utils/validation'
import DNSProviderSelector from './DNSProviderSelector'
import { useDetectDNSProvider } from '../hooks/useDNSDetection'
import { DNSDetectionResult } from './DNSDetectionResult'
import type { DNSProvider } from '../api/dnsProviders'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from './ui/Select'

// Application preset configurations
const APPLICATION_PRESETS: { value: ApplicationPreset; label: string; description: string }[] = [
  { value: 'none', label: 'None', description: 'Standard reverse proxy' },
  { value: 'plex', label: 'Plex', description: 'Media server with remote access' },
  { value: 'jellyfin', label: 'Jellyfin', description: 'Open source media server' },
  { value: 'emby', label: 'Emby', description: 'Media server' },
  { value: 'homeassistant', label: 'Home Assistant', description: 'Home automation' },
  { value: 'nextcloud', label: 'Nextcloud', description: 'File sync and share' },
  { value: 'vaultwarden', label: 'Vaultwarden', description: 'Password manager' },
]

// Docker image to preset mapping for auto-detection
const IMAGE_TO_PRESET: Record<string, ApplicationPreset> = {
  'plexinc/pms-docker': 'plex',
  'linuxserver/plex': 'plex',
  'jellyfin/jellyfin': 'jellyfin',
  'linuxserver/jellyfin': 'jellyfin',
  'emby/embyserver': 'emby',
  'linuxserver/emby': 'emby',
  'homeassistant/home-assistant': 'homeassistant',
  'ghcr.io/home-assistant/home-assistant': 'homeassistant',
  'nextcloud': 'nextcloud',
  'linuxserver/nextcloud': 'nextcloud',
  'vaultwarden/server': 'vaultwarden',
}

// Advanced Caddy config snippets for presets (auto-populate for review)
const PRESET_ADVANCED_CONFIG: Record<ApplicationPreset, string> = {
  none: '',
  plex: JSON.stringify([
    {
      handler: 'headers',
      request: {
        set: {
          'X-Plex-Client-Identifier': '{http.request.header.X-Plex-Client-Identifier}',
          'X-Real-IP': '{http.request.remote.host}',
          'X-Forwarded-Host': '{http.request.host}',
        },
      },
    },
  ], null, 2),
  jellyfin: JSON.stringify([
    {
      handler: 'headers',
      request: {
        set: {
          'X-Real-IP': '{http.request.remote.host}',
          'X-Forwarded-Host': '{http.request.host}',
        },
      },
    },
  ], null, 2),
  emby: JSON.stringify([
    {
      handler: 'headers',
      request: { set: { 'X-Real-IP': '{http.request.remote.host}' } },
    },
  ], null, 2),
  homeassistant: JSON.stringify([
    { handler: 'headers', request: { set: { 'X-Real-IP': '{http.request.remote.host}' } } },
  ], null, 2),
  nextcloud: JSON.stringify([
    { handler: 'headers', request: { set: { 'X-Real-IP': '{http.request.remote.host}', 'X-Forwarded-Host': '{http.request.host}' } } },
  ], null, 2),
  vaultwarden: JSON.stringify([
    { handler: 'headers', request: { set: { 'X-Real-IP': '{http.request.remote.host}' } } },
  ], null, 2),
}

interface ProxyHostFormProps {
  host?: ProxyHost
  onSubmit: (data: Partial<ProxyHost>) => Promise<void>
  onCancel: () => void
}

function buildInitialFormData(host?: ProxyHost): Partial<ProxyHost> & {
  addUptime?: boolean
  uptimeInterval?: number
  uptimeMaxRetries?: number
} {
  return {
    name: host?.name || '',
    domain_names: host?.domain_names || '',
    forward_scheme: host?.forward_scheme || 'http',
    forward_host: host?.forward_host || '',
    forward_port: host?.forward_port || 80,
    ssl_forced: host?.ssl_forced ?? true,
    http2_support: host?.http2_support ?? true,
    hsts_enabled: host?.hsts_enabled ?? true,
    hsts_subdomains: host?.hsts_subdomains ?? true,
    block_exploits: host?.block_exploits ?? true,
    websocket_support: host?.websocket_support ?? true,
    enable_standard_headers: host?.enable_standard_headers ?? true,
    application: (host?.application || 'none') as ApplicationPreset,
    advanced_config: host?.advanced_config || '',
    enabled: host?.enabled ?? true,
    certificate_id: host?.certificate_id,
    access_list_id: host?.access_list_id,
    security_header_profile_id: host?.security_header_profile_id,
    dns_provider_id: host?.dns_provider_id || null,
  }
}

function normalizeNullableID(value: unknown): number | null | undefined {
  if (value === undefined) {
    return undefined
  }

  if (value === null) {
    return null
  }

  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : null
  }

  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (trimmed === '') {
      return null
    }

    if (!/^\d+$/.test(trimmed)) {
      return undefined
    }

    const parsed = Number.parseInt(trimmed, 10)
    return Number.isNaN(parsed) ? undefined : parsed
  }

  return undefined
}

function normalizeAccessListReference(value: unknown): number | string | null | undefined {
  const numericValue = normalizeNullableID(value)
  if (numericValue !== undefined) {
    return numericValue
  }

  if (typeof value !== 'string') {
    return undefined
  }

  const trimmed = value.trim()
  return trimmed === '' ? null : trimmed
}

function resolveSelectToken(value: number | string | null | undefined): string {
  if (value === null || value === undefined) {
    return 'none'
  }

  if (typeof value === 'number') {
    return `id:${value}`
  }

  const trimmed = value.trim()
  if (trimmed === '') {
    return 'none'
  }

  if (trimmed.startsWith('id:') || trimmed.startsWith('uuid:')) {
    return trimmed
  }

  if (/^\d+$/.test(trimmed)) {
    const parsed = Number.parseInt(trimmed, 10)
    return `id:${parsed}`
  }

  return `uuid:${trimmed}`
}

function resolveTokenToFormValue(value: string): number | string | null {
  if (value === 'none') {
    return null
  }

  if (value.startsWith('id:')) {
    const parsed = Number.parseInt(value.slice(3), 10)
    return Number.isNaN(parsed) ? null : parsed
  }

  if (value.startsWith('uuid:')) {
    return value.slice(5)
  }

  if (/^\d+$/.test(value)) {
    const parsed = Number.parseInt(value, 10)
    return Number.isNaN(parsed) ? value : parsed
  }

  return value
}

function getEntityToken(entity: { id?: number; uuid?: string }): string | null {
  if (typeof entity.id === 'number' && Number.isFinite(entity.id)) {
    return `id:${entity.id}`
  }

  if (entity.uuid) {
    return `uuid:${entity.uuid}`
  }

  return null
}

export default function ProxyHostForm({ host, onSubmit, onCancel }: ProxyHostFormProps) {
  type ProxyHostFormState = Omit<Partial<ProxyHost>, 'access_list_id' | 'security_header_profile_id'> & {
    access_list_id?: number | string | null
    security_header_profile_id?: number | string | null
    addUptime?: boolean
    uptimeInterval?: number
    uptimeMaxRetries?: number
  }
  const [formData, setFormData] = useState<ProxyHostFormState>(buildInitialFormData(host))

  useEffect(() => {
    setFormData(buildInitialFormData(host))
  }, [host?.uuid])

  // Charon internal IP for config helpers (previously CPMP internal IP)
  const [charonInternalIP, setCharonInternalIP] = useState<string>('')
  const [copiedField, setCopiedField] = useState<string | null>(null)

  // DNS auto-detection state
  const { mutateAsync: detectProvider, isPending: isDetecting, data: detectionResult, reset: resetDetection } = useDetectDNSProvider()
  const [manualProviderSelection, setManualProviderSelection] = useState(false)
  const detectionTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const portInputRef = useRef<HTMLInputElement | null>(null)

  // Fetch Charon internal IP on mount (legacy: CPMP internal IP)
  useEffect(() => {
    fetch('/api/v1/health')
      .then(res => res.json())
      .then(data => {
        if (data.internal_ip) {
          setCharonInternalIP(data.internal_ip)
        }
      })
      .catch(() => {})
  }, [])

  // Auto-detect DNS provider when wildcard domain is entered (debounced 500ms)
  useEffect(() => {
    // Clear any pending detection
    if (detectionTimeoutRef.current) {
      clearTimeout(detectionTimeoutRef.current)
      detectionTimeoutRef.current = null
    }

    // Reset detection if domain is cleared or manual selection is active
    if (!formData.domain_names || manualProviderSelection) {
      resetDetection()
      return
    }

    // Check if domain contains wildcard
    const domains = formData.domain_names.split(',').map(d => d.trim())
    const wildcardDomain = domains.find(d => d.startsWith('*'))

    if (!wildcardDomain) {
      resetDetection()
      return
    }

    // Extract base domain from wildcard (*.example.com -> example.com)
    const baseDomain = wildcardDomain.replace(/^\*\./, '')

    // Don't detect if provider already set (unless detection succeeded before)
    if (formData.dns_provider_id && !detectionResult?.suggested_provider) {
      return
    }

    // Debounce detection call by 500ms
    detectionTimeoutRef.current = setTimeout(() => {
      detectProvider(baseDomain).catch(err => {
        console.error('DNS detection failed:', err)
      })
    }, 500)

    return () => {
      if (detectionTimeoutRef.current) {
        clearTimeout(detectionTimeoutRef.current)
      }
    }
  }, [formData.domain_names, formData.dns_provider_id, detectProvider, resetDetection, detectionResult, manualProviderSelection])

  // Auto-select suggested provider if confidence is high
  useEffect(() => {
    if (detectionResult?.suggested_provider && detectionResult.confidence === 'high' && !manualProviderSelection && !formData.dns_provider_id) {
      setFormData(prev => ({ ...prev, dns_provider_id: detectionResult.suggested_provider!.id }))
      toast.success(`Auto-selected: ${detectionResult.suggested_provider.name}`)
    }
  }, [detectionResult, manualProviderSelection, formData.dns_provider_id])

  // Handle using suggested provider
  const handleUseSuggested = useCallback((provider: DNSProvider) => {
    setFormData(prev => ({ ...prev, dns_provider_id: provider.id }))
    setManualProviderSelection(false)
    toast.success(`Selected: ${provider.name}`)
  }, [])

  // Handle manual provider selection
  const handleManualSelection = useCallback(() => {
    setManualProviderSelection(true)
  }, [])


  // Auto-detect application preset from Docker image
  const detectApplicationPreset = (imageName: string): ApplicationPreset => {
    const lowerImage = imageName.toLowerCase()
    for (const [pattern, preset] of Object.entries(IMAGE_TO_PRESET)) {
      if (lowerImage.includes(pattern.toLowerCase())) {
        return preset
      }
    }
    return 'none'
  }

  // Copy to clipboard helper
  const copyToClipboard = async (text: string, field: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedField(field)
      setTimeout(() => setCopiedField(null), 2000)
    } catch {
      // Silently fail if clipboard access is denied
    }
  }

  // Get the external URL for this proxy host
  const getExternalUrl = () => {
    const domain = (formData.domain_names ?? '').split(',')[0]?.trim()
    if (!domain) return ''
    return `https://${domain}:443`
  }

  const { servers: remoteServers } = useRemoteServers()
  const safeRemoteServers = Array.isArray(remoteServers) ? remoteServers : []
  const { domains, createDomain } = useDomains()
  const safeDomains = Array.isArray(domains) ? domains : []
  const { certificates } = useCertificates()
  const { data: securityProfiles } = useSecurityHeaderProfiles()

  const [connectionSource, setConnectionSource] = useState<'local' | 'custom' | string>('custom')

  const { containers: dockerContainers, isLoading: dockerLoading, error: dockerError } = useDocker(
    connectionSource === 'local' ? 'local' : undefined,
    connectionSource !== 'local' && connectionSource !== 'custom' ? connectionSource : undefined
  )

  const [selectedDomain, setSelectedDomain] = useState('')
  const [selectedContainerId, setSelectedContainerId] = useState<string>('')

  // New Domain Popup State
  const [showDomainPrompt, setShowDomainPrompt] = useState(false)
  const [pendingDomain, setPendingDomain] = useState('')
  const [dontAskAgain, setDontAskAgain] = useState(false)

  useEffect(() => {
    const stored = localStorage.getItem('charon_dont_ask_domain') ?? localStorage.getItem('cpmp_dont_ask_domain')
    if (stored === 'true') {
      setDontAskAgain(true)
    }
  }, [])

  const [testStatus, setTestStatus] = useState<'idle' | 'testing' | 'success' | 'error'>('idle')
  const [showPresetConfirmModal, setShowPresetConfirmModal] = useState(false)
  const [pendingPreset, setPendingPreset] = useState<ApplicationPreset | null>(null)

  const handleConfirmPresetOverwrite = () => {
    if (!pendingPreset) return
    const presetConfig = PRESET_ADVANCED_CONFIG[pendingPreset]
    const needsWebsockets = ['plex', 'jellyfin', 'emby', 'homeassistant', 'vaultwarden'].includes(pendingPreset)
    setFormData(prev => ({
      ...prev,
      application: pendingPreset,
      websocket_support: needsWebsockets || prev.websocket_support,
      advanced_config: presetConfig,
    }))
    setShowPresetConfirmModal(false)
    setPendingPreset(null)
  }

  const handleCancelPresetOverwrite = () => {
    setShowPresetConfirmModal(false)
    setPendingPreset(null)
  }

  const handleRestoreBackup = () => {
    if (host?.advanced_config_backup) {
      setFormData(prev => ({ ...prev, advanced_config: host.advanced_config_backup || '' }))
    }
  }

  const checkNewDomains = (input: string) => {
    if (dontAskAgain) return

    const domainList = input.split(',').map(d => d.trim()).filter(d => d)
    for (const domain of domainList) {
      const parsed = parse(domain)
      if (parsed.domain && parsed.domain !== domain) {
        // It's a subdomain, check if the base domain exists
        const baseDomain = parsed.domain
        const exists = safeDomains.some(d => d.name === baseDomain)
        if (!exists) {
          setPendingDomain(baseDomain)
          setShowDomainPrompt(true)
          return // Only prompt for one at a time
        }
      } else if (parsed.domain && parsed.domain === domain) {
         // It is a base domain, check if it exists
         const exists = safeDomains.some(d => d.name === domain)
         if (!exists) {
            setPendingDomain(domain)
            setShowDomainPrompt(true)
            return
         }
      }
    }
  }


  const handleSaveDomain = async () => {
    try {
      await createDomain(pendingDomain)
      setShowDomainPrompt(false)
    } catch {
      // Failed to save domain - user can retry
    }
  }

  const handleDontAskToggle = (checked: boolean) => {
    setDontAskAgain(checked)
    localStorage.setItem('charon_dont_ask_domain', String(checked))
    // Keep legacy value for stored user preference compatibility
    localStorage.setItem('cpmp_dont_ask_domain', String(checked))
  }

  const handleTestConnection = async () => {
    if (!formData.forward_host || !formData.forward_port) return

    setTestStatus('testing')
    try {
      await testProxyHostConnection(formData.forward_host, formData.forward_port)
      setTestStatus('success')
      // Reset status after 3 seconds
      setTimeout(() => setTestStatus('idle'), 3000)
    } catch {
      setTestStatus('error')
      // Reset status after 3 seconds
      setTimeout(() => setTestStatus('idle'), 3000)
    }
  }

  // Validate forward_host for IP address warnings
  useEffect(() => {
    const host = formData.forward_host?.trim() || ''
    if (isLikelyDockerContainerIP(host)) {
      setForwardHostWarning(
        'This looks like a Docker container IP address. Docker IPs can change when containers restart. ' +
        'Consider using the container name (e.g., "my-app" or "my-app:8080") for more reliable connections.'
      )
    } else if (isPrivateOrDockerIP(host)) {
      setForwardHostWarning(
        'Using a private IP address. If this is a Docker container, the IP may change on restart. ' +
        'Container names are more reliable for Docker services.'
      )
    } else {
      setForwardHostWarning(null)
    }
  }, [formData.forward_host])

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [nameError, setNameError] = useState<string | null>(null)
  const [forwardHostWarning, setForwardHostWarning] = useState<string | null>(null)
  const [addUptime, setAddUptime] = useState(false)
  const [uptimeInterval, setUptimeInterval] = useState(60)
  const [uptimeMaxRetries, setUptimeMaxRetries] = useState(3)

  // Wildcard domain detection for DNS-01 challenge requirement
  const hasWildcardDomain = formData.domain_names
    ?.split(',')
    .some(d => d.trim().startsWith('*'))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (!formData.forward_port) {
      portInputRef.current?.setCustomValidity('Port is required')
      portInputRef.current?.reportValidity()
      portInputRef.current?.focus()
      return
    }

    if (formData.forward_port < 1 || formData.forward_port > 65535) {
      portInputRef.current?.setCustomValidity('Port must be between 1 and 65535')
      portInputRef.current?.reportValidity()
      portInputRef.current?.focus()
      return
    }

    // Validate DNS provider for wildcard domains
    if (hasWildcardDomain && !formData.dns_provider_id) {
      toast.error('DNS provider is required for wildcard domains')
      return
    }

    setLoading(true)
    try {
      const payload = { ...formData }
      // strip temporary uptime-only flags from payload by destructuring
      const { addUptime: _addUptime, uptimeInterval: _uptimeInterval, uptimeMaxRetries: _uptimeMaxRetries, ...payloadWithoutUptime } = payload as ProxyHostFormState
      void _addUptime; void _uptimeInterval; void _uptimeMaxRetries;

      const submitPayload: Partial<ProxyHost> = {
        ...payloadWithoutUptime,
        access_list_id: normalizeAccessListReference(payloadWithoutUptime.access_list_id),
        security_header_profile_id: normalizeNullableID(payloadWithoutUptime.security_header_profile_id),
      }

      const res = await onSubmit(submitPayload)

      // if user asked to add uptime, request server to sync monitors
      if (addUptime) {
        try {
          await syncMonitors({ interval: uptimeInterval, max_retries: uptimeMaxRetries })
          toast.success('Requested uptime monitor creation')
        } catch (err: unknown) {
          const msg = err instanceof Error ? err.message : String(err)
          toast.error(msg || 'Failed to request uptime creation')
        }
      }

      onCancel()
      return res
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to save host'
      setError(message)
      toast.error(message)
      throw err
    } finally {
      setLoading(false)
    }
  }

  const tryApplyPreset = (preset: ApplicationPreset) => {
    const presetConfig = PRESET_ADVANCED_CONFIG[preset] || ''
    // Auto-apply if no advanced config is present
    if (!formData.advanced_config || formData.advanced_config.trim() === '') {
      const needsWebsockets = ['plex', 'jellyfin', 'emby', 'homeassistant', 'vaultwarden'].includes(preset)
      setFormData(prev => ({ ...prev, application: preset, websocket_support: needsWebsockets || prev.websocket_support, advanced_config: presetConfig }))
      return
    }
    // Otherwise, prompt if different
    if (formData.advanced_config.trim() !== presetConfig.trim()) {
      setPendingPreset(preset)
      setShowPresetConfirmModal(true)
      return
    }
  }

  const handleContainerSelect = (containerId: string) => {
    setSelectedContainerId(containerId)
    const container = dockerContainers.find(c => c.id === containerId)
    if (container) {
      // Prefer container name over IP for stability across restarts
      // Container names work when both Charon and the target are on the same Docker network
      let host = container.names[0] || container.ip
      let port = container.ports && container.ports.length > 0 ? container.ports[0].private_port : 80

      // If using local Docker and we have a container name, show info toast
      if (connectionSource === 'local' && container.names[0]) {
        toast.success(`Using container name "${container.names[0]}" for stable addressing`, { duration: 3000 })
      }

      // If using a Remote Server, try to use the Host IP and Mapped Public Port
      if (connectionSource !== 'local' && connectionSource !== 'custom') {
        const server = safeRemoteServers.find(s => s.uuid === connectionSource)
        if (server) {
          // Use the Remote Server's Host IP (e.g. public/tailscale IP)
          host = server.host

          // Find a mapped public port
          // We prefer the first mapped port we find
          const mappedPort = container.ports?.find(p => p.public_port)
          if (mappedPort) {
            port = mappedPort.public_port
          } else {
            // If no public port is mapped, we can't reach it from outside
            // But we'll leave the internal port as a fallback, though it likely won't work
          }
        }
      }

      let newDomainNames = formData.domain_names
      if (selectedDomain) {
        const subdomain = container.names[0].replace(/^\//, '')
        newDomainNames = `${subdomain}.${selectedDomain}`
      }

      // Auto-detect application preset from image name
      const detectedPreset = detectApplicationPreset(container.image)
      // Auto-enable websockets for apps that need it
      const needsWebsockets = ['plex', 'jellyfin', 'emby', 'homeassistant', 'vaultwarden'].includes(detectedPreset)

      // Try to apply the preset logic (auto-populate or prompt)
      tryApplyPreset(detectedPreset)

      setFormData(prev => ({
        ...prev,
        forward_host: host,
        forward_port: port,
        forward_scheme: 'http',
        domain_names: newDomainNames,
        application: detectedPreset,
        websocket_support: needsWebsockets || prev.websocket_support,
      }))
    }
  }

  const handleBaseDomainChange = (domain: string) => {
    setSelectedDomain(domain)
    if (selectedContainerId && domain) {
      const container = dockerContainers.find(c => c.id === selectedContainerId)
      if (container) {
        const subdomain = container.names[0].replace(/^\//, '')
        setFormData(prev => ({
          ...prev,
          domain_names: `${subdomain}.${domain}`
        }))
      }
    }
  }

  return (
    <>
      {/* Layer 1: Background overlay (z-40) */}
      <div className="fixed inset-0 bg-black/50 z-40" onClick={onCancel} />

      {/* Layer 2: Form container (z-50, pointer-events-none) */}
      <div className="fixed inset-0 flex items-center justify-center p-4 pointer-events-none z-50">

        {/* Layer 3: Form content (pointer-events-auto) */}
        <div
          className="bg-dark-card rounded-lg border border-gray-800 max-w-2xl w-full max-h-[90vh] overflow-y-auto pointer-events-auto"
          role="dialog"
          aria-modal="true"
          aria-labelledby="proxy-host-form-title"
        >
        <div className="p-6 border-b border-gray-800">
          <h2 id="proxy-host-form-title" className="text-2xl font-bold text-white">
            {host ? 'Edit Proxy Host' : 'Add Proxy Host'}
          </h2>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          {error && (
            <div className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded">
              {error}
            </div>
          )}

          {/* Name Field */}
          <div>
            <label htmlFor="proxy-name" className="block text-sm font-medium text-gray-300 mb-2">
              Name <span className="text-red-400">*</span>
            </label>
            <input
              id="proxy-name"
              type="text"
              required
              value={formData.name}
              onChange={e => {
                setFormData(prev => ({ ...prev, name: e.target.value }))
                if (nameError && e.target.value.trim()) {
                  setNameError(null)
                }
              }}
              placeholder="My Service"
              className={`w-full bg-gray-900 border rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                nameError ? 'border-red-500' : 'border-gray-700'
              }`}
            />
            {nameError ? (
              <p className="text-xs text-red-400 mt-1">{nameError}</p>
            ) : (
              <p className="text-xs text-gray-500 mt-1">
                A friendly name to identify this proxy host
              </p>
            )}
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* Docker Container Quick Select */}
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">
                Source
              </label>
              <Select value={connectionSource} onValueChange={setConnectionSource}>
                <SelectTrigger id="connection-source" className="w-full bg-gray-900 border-gray-700 text-white" aria-label="Source">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="custom">Custom / Manual</SelectItem>
                  <SelectItem value="local">Local (Docker Socket)</SelectItem>
                  {safeRemoteServers
                    .filter(s => s.provider === 'docker' && s.enabled)
                    .map(server => (
                      <SelectItem key={server.uuid} value={server.uuid}>
                        {server.name} ({server.host})
                      </SelectItem>
                    ))
                  }
                </SelectContent>
              </Select>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">
                Containers
              </label>

              <Select
                value=""
                onValueChange={e => e && handleContainerSelect(e)}
              >
                <SelectTrigger id="quick-select-docker" className="w-full bg-gray-900 border-gray-700 text-white disabled:opacity-50" disabled={dockerLoading || connectionSource === 'custom'} aria-label="Containers">
                  <SelectValue placeholder={connectionSource === 'custom'
                    ? 'Select a source to view containers'
                    : (dockerLoading ? 'Loading containers...' : 'Select a container')}
                  />
                </SelectTrigger>
                <SelectContent>
                  {dockerContainers.map(container => (
                    <SelectItem key={container.id} value={container.id}>
                      {container.names[0]} ({container.image})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {dockerError && connectionSource !== 'custom' && (
                <div className="mt-2 p-3 bg-red-500/10 border border-red-500/30 rounded-lg">
                  <div className="flex items-start gap-2">
                    <AlertTriangle className="w-4 h-4 text-red-400 shrink-0 mt-0.5" />
                    <div className="text-xs text-red-300">
                      <p className="font-semibold mb-1">Docker Connection Failed</p>
                      <p className="text-red-400/90 mb-2">
                        {(dockerError as Error).message}
                      </p>
                      <p className="text-gray-400">
                        <strong>Troubleshooting:</strong> Ensure Docker is running and the socket is accessible.
                        If running in a container, mount <code className="text-xs bg-gray-800 px-1 py-0.5 rounded">/var/run/docker.sock</code> and
                        ensure the container has access to the Docker socket group
                        (e.g., <code className="text-xs bg-gray-800 px-1 py-0.5 rounded">group_add</code> in
                        Compose or <code className="text-xs bg-gray-800 px-1 py-0.5 rounded">--group-add</code> with
                        Docker&nbsp;CLI).
                      </p>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Domain Names */}
          <div className="space-y-4">
            {safeDomains.length > 0 && (
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Base Domain (Auto-fill)
                </label>
                <Select value={selectedDomain} onValueChange={handleBaseDomainChange}>
                  <SelectTrigger className="w-full bg-gray-900 border-gray-700 text-white" aria-label="Base Domain (Auto-fill)">
                    <SelectValue placeholder="Select a base domain" />
                  </SelectTrigger>
                  <SelectContent>
                    {safeDomains.map(domain => (
                      <SelectItem key={domain.uuid} value={domain.name}>
                        {domain.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
            <div>
              <label htmlFor="domain-names" className="block text-sm font-medium text-gray-300 mb-2">
                Domain Names (comma-separated)
              </label>
              <input
                id="domain-names"
                type="text"
                required
                value={formData.domain_names}
                onChange={e => setFormData(prev => ({ ...prev, domain_names: e.target.value }))}
                onBlur={e => checkNewDomains(e.target.value)}
                placeholder="example.com, www.example.com"
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>

          {/* Forward Details */}
          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">Scheme</label>
              <Select value={formData.forward_scheme} onValueChange={scheme => setFormData(prev => ({ ...prev, forward_scheme: scheme }))}>
                <SelectTrigger className="w-full bg-gray-900 border-gray-700 text-white" aria-label="Scheme">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="http">HTTP</SelectItem>
                  <SelectItem value="https">HTTPS</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <label htmlFor="forward-host" className="block text-sm font-medium text-gray-300 mb-2">
                Host
                <div
                  title="Enter a hostname, container name, or IP address. For Docker containers, using the container name (e.g., 'my-nginx') is recommended as it remains stable across container restarts."
                  className="inline-block ml-1 text-gray-500 hover:text-gray-300 cursor-help"
                >
                  <CircleHelp size={14} />
                </div>
              </label>
              <input
                id="forward-host"
                type="text"
                required
                value={formData.forward_host}
                onChange={e => setFormData(prev => ({ ...prev, forward_host: e.target.value }))}
                placeholder="my-container or 192.168.1.100"
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              {forwardHostWarning && (
                <Alert variant="warning" className="mt-2" title="IP Address Detected">
                  {forwardHostWarning}
                </Alert>
              )}
            </div>
            <div>
              <label htmlFor="forward-port" className="block text-sm font-medium text-gray-300 mb-2">Port</label>
              <input
                id="forward-port"
                type="number"
                required
                min="1"
                max="65535"
                ref={portInputRef}
                value={formData.forward_port}
                onChange={e => {
                  const v = parseInt(e.target.value)
                  portInputRef.current?.setCustomValidity('')
                  setFormData(prev => ({ ...prev, forward_port: Number.isNaN(v) ? 0 : v }))
                }}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>

          {/* SSL Certificate Selection */}
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              SSL Certificate
            </label>
            <Select value={String(formData.certificate_id || 0)} onValueChange={e => setFormData(prev => ({ ...prev, certificate_id: parseInt(e) || null }))}>
              <SelectTrigger className="w-full bg-gray-900 border-gray-700 text-white" aria-label="SSL Certificate">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="0">Auto-manage with Let's Encrypt (recommended)</SelectItem>
                {certificates.map(cert => (
                  <SelectItem key={cert.id || cert.domain} value={String(cert.id ?? 0)}>
                    {(cert.name || cert.domain)}
                    {cert.provider ? ` (${cert.provider})` : ''}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-gray-500 mt-1">
              Choose an existing certificate if already issued for these domains, or let Charon request/renew via Let's Encrypt automatically.
            </p>
          </div>

          {/* DNS Provider Selector for Wildcard Domains */}
          {hasWildcardDomain && (
            <div className="space-y-3" data-testid="dns-provider-section">
              <Alert variant="info">
                <Info className="h-4 w-4" />
                <div>
                  <p className="font-medium">Wildcard Certificate Required</p>
                  <p className="text-sm mt-1">
                    Wildcard certificates (*.example.com) require DNS-01 challenge.
                    Select a DNS provider to automatically manage DNS records for certificate validation.
                  </p>
                </div>
              </Alert>

              {/* DNS Detection Result */}
              {(isDetecting || detectionResult) && !manualProviderSelection && (
                <DNSDetectionResult
                  result={detectionResult!}
                  isLoading={isDetecting}
                  onUseSuggested={handleUseSuggested}
                  onSelectManually={handleManualSelection}
                />
              )}

              <DNSProviderSelector
                value={formData.dns_provider_id ?? undefined}
                onChange={(id) => {
                  setFormData(prev => ({ ...prev, dns_provider_id: id ?? null }))
                  if (id) {
                    setManualProviderSelection(true)
                  }
                }}
                required={true}
              />
            </div>
          )}

          {/* Access Control List */}
          <AccessListSelector
            value={formData.access_list_id ?? null}
            onChange={id => setFormData(prev => ({ ...prev, access_list_id: id }))}
          />

          {/* Security Headers Profile */}
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Security Headers
              <span className="text-gray-500 font-normal ml-2">(Optional)</span>
            </label>

            <Select
              value={resolveSelectToken(formData.security_header_profile_id as number | string | null | undefined)}
              onValueChange={(value) => {
                setFormData(prev => ({
                  ...prev,
                  security_header_profile_id: resolveTokenToFormValue(value),
                }))
              }}
            >
              <SelectTrigger className="w-full bg-gray-900 border-gray-700 text-white" aria-label="Security Headers">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">None (No Security Headers)</SelectItem>
                {securityProfiles
                  ?.filter(p => p.is_preset)
                  .sort((a, b) => a.security_score - b.security_score)
                  .map(profile => {
                    const optionToken = getEntityToken(profile)
                    if (!optionToken) {
                      return null
                    }

                    return (
                      <SelectItem key={optionToken} value={optionToken}>
                        {profile.name} (Score: {profile.security_score}/100)
                      </SelectItem>
                    )
                  })}
                {(securityProfiles?.filter(p => !p.is_preset) || []).length > 0 && (
                  <>
                    {(securityProfiles || [])
                      .filter(p => !p.is_preset)
                      .map(profile => {
                        const optionToken = getEntityToken(profile)
                        if (!optionToken) {
                          return null
                        }

                        return (
                          <SelectItem key={optionToken} value={optionToken}>
                            {profile.name} (Score: {profile.security_score}/100)
                          </SelectItem>
                        )
                      })}
                  </>
                )}
              </SelectContent>
            </Select>

            {formData.security_header_profile_id && (() => {
              const selectedToken = resolveSelectToken(formData.security_header_profile_id)
              const selected = securityProfiles?.find(p => getEntityToken(p) === selectedToken)
              if (!selected) return null

              return (
                <div className="mt-2 flex items-center gap-2">
                  <SecurityScoreDisplay
                    score={selected.security_score}
                    size="sm"
                    showDetails={false}
                  />
                  <span className="text-xs text-gray-400">
                    {selected.description}
                  </span>
                </div>
              )
            })()}

            {/* Mobile App Compatibility Warning for Strict/Paranoid profiles */}
            {formData.security_header_profile_id && (() => {
              const selectedToken = resolveSelectToken(formData.security_header_profile_id)
              const selected = securityProfiles?.find(p => getEntityToken(p) === selectedToken)
              if (!selected) return null

              const isRestrictive = selected.preset_type === 'strict' || selected.preset_type === 'paranoid'

              if (!isRestrictive) return null

              return (
                <div className="mt-2 p-3 bg-yellow-900/30 border border-yellow-600 rounded-lg">
                  <div className="flex items-start gap-2">
                    <AlertTriangle className="w-5 h-5 text-yellow-500 shrink-0 mt-0.5" />
                    <div className="text-sm">
                      <p className="font-medium text-yellow-400">Mobile App Compatibility Warning</p>
                      <p className="text-yellow-300/80 mt-1">
                        This security profile may break mobile apps like Radarr, Plex, Jellyfin, or Home Assistant companion apps.
                        Consider using "API-Friendly" or "Basic" for services accessed by mobile clients.
                      </p>
                    </div>
                  </div>
                </div>
              )
            })()}

            <p className="text-xs text-gray-500 mt-1">
              Apply HTTP security headers to protect against common web vulnerabilities.{' '}
              <a
                href="/security-headers"
                target="_blank"
                className="text-blue-400 hover:text-blue-300"
              >
                Manage Profiles →
              </a>
            </p>
          </div>

          {/* Application Preset */}
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Application Preset
              <span className="text-gray-500 font-normal ml-2">(Optional)</span>
            </label>
            <Select
              value={formData.application}
              onValueChange={preset => {
                const presetVal = preset as ApplicationPreset
                // Apply with advanced_config logic
                const needsWebsockets = ['plex', 'jellyfin', 'emby', 'homeassistant', 'vaultwarden'].includes(presetVal)
                // Delegate to shared logic which will auto-apply or prompt
                tryApplyPreset(presetVal)
                // Ensure we still enable websockets when preset implies it
                setFormData(prev => ({ ...prev, websocket_support: needsWebsockets || prev.websocket_support }))
              }}
            >
              <SelectTrigger
                id="application-preset"
                data-testid="application-preset"
                className="w-full bg-gray-900 border-gray-700 text-white"
                aria-label="Application Preset"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {APPLICATION_PRESETS.map(preset => (
                  <SelectItem key={preset.value} value={preset.value}>
                    {preset.label} - {preset.description}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-gray-500 mt-1">
              Presets automatically configure headers for remote access behind tunnels/CGNAT.
            </p>
          </div>

          {/* Application Config Helper */}
          {formData.application !== 'none' && formData.domain_names && (
            <div className="bg-blue-900/20 border border-blue-700 rounded-lg p-4">
              <div className="flex items-start gap-3">
                <Info className="w-5 h-5 text-blue-400 shrink-0 mt-0.5" />
                <div className="flex-1 space-y-3">
                  <h4 className="text-sm font-semibold text-blue-300">
                    {formData.application === 'plex' && 'Plex Remote Access Setup'}
                    {formData.application === 'jellyfin' && 'Jellyfin Proxy Setup'}
                    {formData.application === 'emby' && 'Emby Proxy Setup'}
                    {formData.application === 'homeassistant' && 'Home Assistant Proxy Setup'}
                    {formData.application === 'nextcloud' && 'Nextcloud Proxy Setup'}
                    {formData.application === 'vaultwarden' && 'Vaultwarden Setup'}
                  </h4>

                  {/* Plex Helper */}
                  {formData.application === 'plex' && (
                    <>
                      <p className="text-xs text-gray-300">
                        Copy this URL and paste it into <strong>Plex Settings → Network → Custom server access URLs</strong>
                      </p>
                      <div className="flex items-center gap-2">
                        <code className="flex-1 bg-gray-900 px-3 py-2 rounded text-sm text-green-400 font-mono">
                          {getExternalUrl()}
                        </code>
                        <button
                          type="button"
                          onClick={() => copyToClipboard(getExternalUrl(), 'plex-url')}
                          className="px-3 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded text-sm flex items-center gap-1"
                        >
                          {copiedField === 'plex-url' ? <Check size={14} /> : <Copy size={14} />}
                          {copiedField === 'plex-url' ? 'Copied!' : 'Copy'}
                        </button>
                      </div>
                      {charonInternalIP && (
                        <div className="mt-3">
                          <p className="text-xs text-gray-300">
                            Add this IP to <strong>Plex Settings → Network → List of IP addresses that are considered local</strong> or as a trusted proxy to ensure Plex displays remote clients correctly.
                          </p>
                          <div className="flex items-center gap-2 mt-2">
                            <code className="flex-1 bg-gray-900 px-3 py-2 rounded text-sm text-green-400 font-mono">
                              {charonInternalIP}
                            </code>
                            <button
                              type="button"
                              onClick={() => copyToClipboard(charonInternalIP, 'plex-proxy-ip')}
                              className="px-3 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded text-sm flex items-center gap-1"
                            >
                              {copiedField === 'plex-proxy-ip' ? <Check size={14} /> : <Copy size={14} />}
                              {copiedField === 'plex-proxy-ip' ? 'Copied!' : 'Copy'}
                            </button>
                          </div>
                        </div>
                      )}
                    </>
                  )}

                  {/* Jellyfin/Emby Helper */}
                  {(formData.application === 'jellyfin' || formData.application === 'emby') && charonInternalIP && (
                    <>
                      <p className="text-xs text-gray-300">
                        Add this IP to <strong>{formData.application === 'jellyfin' ? 'Jellyfin' : 'Emby'} → Dashboard → Networking → Known Proxies</strong>
                      </p>
                      <div className="flex items-center gap-2">
                        <code className="flex-1 bg-gray-900 px-3 py-2 rounded text-sm text-green-400 font-mono">
                          {charonInternalIP}
                        </code>
                        <button
                          type="button"
                          onClick={() => copyToClipboard(charonInternalIP, 'proxy-ip')}
                          className="px-3 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded text-sm flex items-center gap-1"
                        >
                          {copiedField === 'proxy-ip' ? <Check size={14} /> : <Copy size={14} />}
                          {copiedField === 'proxy-ip' ? 'Copied!' : 'Copy'}
                        </button>
                      </div>
                    </>
                  )}

                  {/* Home Assistant Helper */}
                  {formData.application === 'homeassistant' && charonInternalIP && (
                    <>
                      <p className="text-xs text-gray-300">
                        Add this to your <strong>configuration.yaml</strong> under <code>http:</code>
                      </p>
                      <div className="relative">
                        <pre className="bg-gray-900 px-3 py-2 rounded text-sm text-green-400 font-mono overflow-x-auto">
{`http:
  use_x_forwarded_for: true
  trusted_proxies:
    - ${charonInternalIP}`}
                        </pre>
                        <button
                          type="button"
                          onClick={() => copyToClipboard(`http:\n  use_x_forwarded_for: true\n  trusted_proxies:\n    - ${charonInternalIP}`, 'ha-yaml')}
                          className="absolute top-2 right-2 px-2 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded text-xs flex items-center gap-1"
                        >
                          {copiedField === 'ha-yaml' ? <Check size={12} /> : <Copy size={12} />}
                          {copiedField === 'ha-yaml' ? 'Copied!' : 'Copy'}
                        </button>
                      </div>
                    </>
                  )}

                  {/* Nextcloud Helper */}
                  {formData.application === 'nextcloud' && charonInternalIP && (
                    <>
                      <p className="text-xs text-gray-300">
                        Add this to your <strong>config/config.php</strong>
                      </p>
                      <div className="relative">
                        <pre className="bg-gray-900 px-3 py-2 rounded text-sm text-green-400 font-mono overflow-x-auto">
{`'trusted_proxies' => ['${charonInternalIP}'],
'overwriteprotocol' => 'https',`}
                        </pre>
                        <button
                          type="button"
                          onClick={() => copyToClipboard(`'trusted_proxies' => ['${charonInternalIP}'],\n'overwriteprotocol' => 'https',`, 'nc-php')}
                          className="absolute top-2 right-2 px-2 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded text-xs flex items-center gap-1"
                        >
                          {copiedField === 'nc-php' ? <Check size={12} /> : <Copy size={12} />}
                          {copiedField === 'nc-php' ? 'Copied!' : 'Copy'}
                        </button>
                      </div>
                    </>
                  )}

                  {/* Vaultwarden Helper */}
                  {formData.application === 'vaultwarden' && (
                    <p className="text-xs text-gray-300">
                      WebSocket support is enabled automatically for live sync. Ensure your Bitwarden clients use this domain: <code className="text-green-400">{formData.domain_names.split(',')[0]?.trim()}</code>
                    </p>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* SSL & Security Options */}
          <div className="space-y-3">
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.ssl_forced}
                onChange={e => setFormData(prev => ({ ...prev, ssl_forced: e.target.checked }))}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">Force SSL</span>
              <div title="Redirects visitors to the secure HTTPS version of your site. You should almost always turn this on to protect your data." className="text-gray-500 hover:text-gray-300 cursor-help">
                <CircleHelp size={14} />
              </div>
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.http2_support}
                onChange={e => setFormData(prev => ({ ...prev, http2_support: e.target.checked }))}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">HTTP/2 Support</span>
              <div title="Makes your site load faster by using a modern connection standard. Safe to leave on for most sites." className="text-gray-500 hover:text-gray-300 cursor-help">
                <CircleHelp size={14} />
              </div>
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.hsts_enabled}
                onChange={e => setFormData(prev => ({ ...prev, hsts_enabled: e.target.checked }))}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">HSTS Enabled</span>
              <div title="Tells browsers to REMEMBER to only use HTTPS for this site. Adds extra security but can be tricky if you ever want to go back to HTTP." className="text-gray-500 hover:text-gray-300 cursor-help">
                <CircleHelp size={14} />
              </div>
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.hsts_subdomains}
                onChange={e => setFormData(prev => ({ ...prev, hsts_subdomains: e.target.checked }))}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">HSTS Subdomains</span>
              <div title="Applies the HSTS rule to all subdomains (like blog.mysite.com). Only use this if ALL your subdomains are secure." className="text-gray-500 hover:text-gray-300 cursor-help">
                <CircleHelp size={14} />
              </div>
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.block_exploits}
                onChange={e => setFormData(prev => ({ ...prev, block_exploits: e.target.checked }))}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">Block Exploits</span>
              <div title="Automatically blocks common hacking attempts. Recommended to keep your site safe." className="text-gray-500 hover:text-gray-300 cursor-help">
                <CircleHelp size={14} />
              </div>
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.websocket_support}
                onChange={e => setFormData(prev => ({ ...prev, websocket_support: e.target.checked }))}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">Websockets Support</span>
              <div title="Needed for apps that update in real-time (like chat, notifications, or live status). If your app feels 'broken' or doesn't update, try turning this on." className="text-gray-500 hover:text-gray-300 cursor-help">
                <CircleHelp size={14} />
              </div>
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.enable_standard_headers ?? true}
                onChange={e => setFormData(prev => ({ ...prev, enable_standard_headers: e.target.checked }))}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">Enable Standard Proxy Headers</span>
              <div title="Adds X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, and X-Forwarded-Port headers to help backend applications detect client IPs, enforce HTTPS, and generate correct URLs. Recommended for all proxy hosts. Existing hosts: disabled by default for backward compatibility." className="text-gray-500 hover:text-gray-300 cursor-help">
                <CircleHelp size={14} />
              </div>
            </label>
          </div>

          {/* Legacy Headers Warning Banner */}
          {host && (formData.enable_standard_headers === false) && (
            <div className="bg-yellow-900/20 border border-yellow-600 rounded-lg p-3">
              <div className="flex items-start gap-2">
                <Info className="w-5 h-5 text-yellow-500 shrink-0 mt-0.5" />
                <div className="text-sm">
                  <p className="font-medium text-yellow-400">Standard Proxy Headers Disabled</p>
                  <p className="text-yellow-300/80 mt-1">
                    This proxy host is using the legacy behavior (headers only with WebSocket support).
                    Enable this option to ensure backend applications receive client IP and protocol information.
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* Advanced Config */}
          <div>
            <label htmlFor="advanced-config" className="block text-sm font-medium text-gray-300 mb-2">
              Advanced Caddy Config (Optional)
            </label>
            <div className="relative">
              {host?.advanced_config_backup && (
                <div className="absolute right-0 top-0 -translate-y-8">
                  <button
                    type="button"
                    onClick={handleRestoreBackup}
                    className="px-3 py-1 bg-gray-800 hover:bg-gray-700 text-white rounded text-xs"
                  >
                    Restore previous config
                  </button>
                </div>
              )}
              <textarea
              id="advanced-config"
              value={formData.advanced_config}
              onChange={e => setFormData(prev => ({ ...prev, advanced_config: e.target.value }))}
              placeholder="Additional Caddy directives..."
              rows={4}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white font-mono text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            </div>
          </div>

          {/* Enabled Toggle */}
          <div className="flex items-center justify-end pb-2">
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={formData.enabled}
                onChange={e => setFormData(prev => ({ ...prev, enabled: e.target.checked }))}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm font-medium text-white">Enable Proxy Host</span>
            </label>
          </div>

          {/* Uptime option */}
          <div className="border-t border-gray-800 pt-4">
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={addUptime}
                onChange={e => setAddUptime(e.target.checked)}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">Add Uptime monitoring for this host</span>
            </label>

            {addUptime && (
              <div className="grid grid-cols-2 gap-4 mt-3">
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Check Interval (seconds)</label>
                  <input
                    type="number"
                    min={10}
                    value={uptimeInterval}
                    onChange={e => setUptimeInterval(parseInt(e.target.value || '60'))}
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white"
                  />
                </div>
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Max Retries</label>
                  <input
                    type="number"
                    min={1}
                    value={uptimeMaxRetries}
                    onChange={e => setUptimeMaxRetries(parseInt(e.target.value || '3'))}
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white"
                  />
                </div>
              </div>
            )}
          </div>

          {/* Actions */}
          <div className="flex gap-3 justify-end pt-4 border-t border-gray-800">
            <button
              type="button"
              onClick={onCancel}
              disabled={loading}
              data-testid="proxy-host-cancel"
              className="px-6 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              Cancel
            </button>

            <button
              type="button"
              onClick={handleTestConnection}
              disabled={loading || testStatus === 'testing' || !formData.forward_host || !formData.forward_port}
              data-testid="proxy-host-test-connection"
              className={`px-4 py-2 rounded-lg font-medium transition-colors flex items-center gap-2 disabled:opacity-50 ${
                testStatus === 'success' ? 'bg-green-600 hover:bg-green-500 text-white' :
                testStatus === 'error' ? 'bg-red-600 hover:bg-red-500 text-white' :
                'bg-gray-700 hover:bg-gray-600 text-white'
              }`}
              title="Test connection to the forward host"
            >
              {testStatus === 'testing' ? <Loader2 size={18} className="animate-spin" /> :
               testStatus === 'success' ? <Check size={18} /> :
               testStatus === 'error' ? <X size={18} /> :
               'Test Connection'}
            </button>

            <button
              type="submit"
              disabled={loading}
              data-testid="proxy-host-save"
              className="px-6 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              {loading ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>

      {/* New Domain Prompt Modal */}
      {showDomainPrompt && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center p-4 z-60">
          <div className="bg-gray-800 rounded-lg border border-gray-700 max-w-md w-full p-6 shadow-xl">
            <div className="flex items-center gap-3 mb-4 text-blue-400">
              <AlertCircle size={24} />
              <h3 className="text-lg font-semibold text-white">New Base Domain Detected</h3>
            </div>

            <p className="text-gray-300 mb-4">
              You are using a new base domain: <span className="font-mono font-bold text-white">{pendingDomain}</span>
            </p>
            <p className="text-gray-400 text-sm mb-6">
              Would you like to save this to your domain list for easier selection in the future?
            </p>

            <div className="flex items-center gap-2 mb-6">
              <input
                type="checkbox"
                id="dont-ask"
                checked={dontAskAgain}
                onChange={e => handleDontAskToggle(e.target.checked)}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-600 rounded focus:ring-blue-500"
              />
              <label htmlFor="dont-ask" className="text-sm text-gray-400 select-none">
                Don't ask me again
              </label>
            </div>

            <div className="flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setShowDomainPrompt(false)}
                className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg text-sm transition-colors"
              >
                No, thanks
              </button>
              <button
                type="button"
                onClick={handleSaveDomain}
                className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors"
              >
                Yes, save it
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Preset Overwrite Confirmation Modal */}
      {showPresetConfirmModal && pendingPreset && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center p-4 z-60">
          <div className="bg-gray-800 rounded-lg border border-gray-700 max-w-lg w-full p-6 shadow-xl">
            <div className="flex items-center gap-3 mb-4 text-blue-400">
              <AlertCircle size={24} />
              <h3 className="text-lg font-semibold text-white">Confirm Preset Overwrite</h3>
            </div>

            <p className="text-gray-300 mb-4">
              You already have an Advanced Caddy Config for this host. Applying the <strong>{pendingPreset}</strong> preset will overwrite the existing configuration.
            </p>

            <p className="text-gray-400 text-sm mb-4">A backup of your existing configuration will be created when the host is saved.</p>

            <div className="mb-4">
              <label className="block text-sm font-medium text-gray-300 mb-2">Preset Preview</label>
              <pre className="bg-gray-900 px-3 py-2 rounded text-sm text-green-400 font-mono overflow-x-auto whitespace-pre-wrap">
                {PRESET_ADVANCED_CONFIG[pendingPreset]}
              </pre>
            </div>

            <div className="flex justify-end gap-3">
              <button
                type="button"
                onClick={handleCancelPresetOverwrite}
                className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg text-sm transition-colors"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleConfirmPresetOverwrite}
                className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors"
              >
                Overwrite
              </button>
            </div>
          </div>
        </div>
      )}
        </div>
      </div>
    </>
  )
}
