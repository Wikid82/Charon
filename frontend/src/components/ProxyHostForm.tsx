import { useState, useEffect } from 'react'
import { CircleHelp, AlertCircle, Check, X, Loader2, Copy, Info } from 'lucide-react'
import type { ProxyHost, ApplicationPreset } from '../api/proxyHosts'
import { testProxyHostConnection } from '../api/proxyHosts'
import { useRemoteServers } from '../hooks/useRemoteServers'
import { useDomains } from '../hooks/useDomains'
import { useCertificates } from '../hooks/useCertificates'
import { useDocker } from '../hooks/useDocker'
import AccessListSelector from './AccessListSelector'
import { parse } from 'tldts'

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

interface ProxyHostFormProps {
  host?: ProxyHost
  onSubmit: (data: Partial<ProxyHost>) => Promise<void>
  onCancel: () => void
}

export default function ProxyHostForm({ host, onSubmit, onCancel }: ProxyHostFormProps) {
  const [formData, setFormData] = useState({
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
    application: (host?.application || 'none') as ApplicationPreset,
    advanced_config: host?.advanced_config || '',
    enabled: host?.enabled ?? true,
    certificate_id: host?.certificate_id,
    access_list_id: host?.access_list_id,
  })

  // CPMP internal IP for config helpers
  const [cpmpInternalIP, setCpmpInternalIP] = useState<string>('')
  const [copiedField, setCopiedField] = useState<string | null>(null)

  // Fetch CPMP internal IP on mount
  useEffect(() => {
    fetch('/api/v1/health')
      .then(res => res.json())
      .then(data => {
        if (data.internal_ip) {
          setCpmpInternalIP(data.internal_ip)
        }
      })
      .catch(() => {})
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
      console.error('Failed to copy to clipboard')
    }
  }

  // Get the external URL for this proxy host
  const getExternalUrl = () => {
    const domain = formData.domain_names.split(',')[0]?.trim()
    if (!domain) return ''
    return `https://${domain}:443`
  }

  const { servers: remoteServers } = useRemoteServers()
  const { domains, createDomain } = useDomains()
  const { certificates } = useCertificates()

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
    const stored = localStorage.getItem('cpmp_dont_ask_domain')
    if (stored === 'true') {
      setDontAskAgain(true)
    }
  }, [])

  const [testStatus, setTestStatus] = useState<'idle' | 'testing' | 'success' | 'error'>('idle')

  const checkNewDomains = (input: string) => {
    if (dontAskAgain) return

    const domainList = input.split(',').map(d => d.trim()).filter(d => d)
    for (const domain of domainList) {
      const parsed = parse(domain)
      if (parsed.domain && parsed.domain !== domain) {
        // It's a subdomain, check if the base domain exists
        const baseDomain = parsed.domain
        const exists = domains.some(d => d.name === baseDomain)
        if (!exists) {
          setPendingDomain(baseDomain)
          setShowDomainPrompt(true)
          return // Only prompt for one at a time
        }
      } else if (parsed.domain && parsed.domain === domain) {
         // It is a base domain, check if it exists
         const exists = domains.some(d => d.name === domain)
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
    } catch (err) {
      console.error("Failed to save domain", err)
      // Optionally show error
    }
  }

  const handleDontAskToggle = (checked: boolean) => {
    setDontAskAgain(checked)
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
    } catch (err) {
      console.error("Test connection failed", err)
      setTestStatus('error')
      // Reset status after 3 seconds
      setTimeout(() => setTestStatus('idle'), 3000)
    }
  }

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [nameError, setNameError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)
    setNameError(null)

    // Validate name is required
    if (!formData.name.trim()) {
      setNameError('Name is required')
      setLoading(false)
      return
    }

    try {
      await onSubmit(formData)
    } catch (err: unknown) {
      console.error("Submit error:", err)
      // Extract error message from axios response if available
      const errorObj = err as { response?: { data?: { error?: string } }; message?: string }
      const message = errorObj.response?.data?.error || errorObj.message || 'Failed to save proxy host'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleContainerSelect = (containerId: string) => {
    setSelectedContainerId(containerId)
    const container = dockerContainers.find(c => c.id === containerId)
    if (container) {
      // Default to internal IP and private port
      let host = container.ip || container.names[0]
      let port = container.ports && container.ports.length > 0 ? container.ports[0].private_port : 80

      // If using a Remote Server, try to use the Host IP and Mapped Public Port
      if (connectionSource !== 'local' && connectionSource !== 'custom') {
        const server = remoteServers.find(s => s.uuid === connectionSource)
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
            console.warn('No public port mapped for container on remote server')
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

      setFormData({
        ...formData,
        forward_host: host,
        forward_port: port,
        forward_scheme: 'http',
        domain_names: newDomainNames,
        application: detectedPreset,
        websocket_support: needsWebsockets || formData.websocket_support,
      })
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
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
      <div className="bg-dark-card rounded-lg border border-gray-800 max-w-2xl w-full max-h-[90vh] overflow-y-auto">
        <div className="p-6 border-b border-gray-800">
          <h2 className="text-2xl font-bold text-white">
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
                setFormData({ ...formData, name: e.target.value })
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
              <label htmlFor="connection-source" className="block text-sm font-medium text-gray-300 mb-2">
                Source
              </label>
              <select
                id="connection-source"
                value={connectionSource}
                onChange={e => setConnectionSource(e.target.value)}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="custom">Custom / Manual</option>
                <option value="local">Local (Docker Socket)</option>
                {remoteServers
                  .filter(s => s.provider === 'docker' && s.enabled)
                  .map(server => (
                    <option key={server.uuid} value={server.uuid}>
                      {server.name} ({server.host})
                    </option>
                  ))
                }
              </select>
            </div>

            <div>
              <label htmlFor="quick-select-docker" className="block text-sm font-medium text-gray-300 mb-2">
                Containers
              </label>

              <select
                id="quick-select-docker"
                onChange={e => handleContainerSelect(e.target.value)}
                disabled={dockerLoading || connectionSource === 'custom'}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
              >
                <option value="">
                  {connectionSource === 'custom'
                    ? 'Select a source to view containers'
                    : (dockerLoading ? 'Loading containers...' : '-- Select a container --')}
                </option>
                {dockerContainers.map(container => (
                  <option key={container.id} value={container.id}>
                    {container.names[0]} ({container.image})
                  </option>
                ))}
              </select>
              {dockerError && connectionSource !== 'custom' && (
                <p className="text-xs text-red-400 mt-1">
                  Failed to connect: {(dockerError as Error).message}
                </p>
              )}
            </div>
          </div>

          {/* Domain Names */}
          <div className="space-y-4">
            {domains.length > 0 && (
              <div>
                <label htmlFor="base-domain" className="block text-sm font-medium text-gray-300 mb-2">
                  Base Domain (Auto-fill)
                </label>
                <select
                  id="base-domain"
                  value={selectedDomain}
                  onChange={e => handleBaseDomainChange(e.target.value)}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="">-- Select a base domain --</option>
                  {domains.map(domain => (
                    <option key={domain.uuid} value={domain.name}>
                      {domain.name}
                    </option>
                  ))}
                </select>
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
                onChange={e => setFormData({ ...formData, domain_names: e.target.value })}
                onBlur={e => checkNewDomains(e.target.value)}
                placeholder="example.com, www.example.com"
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>

          {/* Forward Details */}
          <div className="grid grid-cols-3 gap-4">
            <div>
              <label htmlFor="forward-scheme" className="block text-sm font-medium text-gray-300 mb-2">Scheme</label>
              <select
                id="forward-scheme"
                value={formData.forward_scheme}
                onChange={e => setFormData({ ...formData, forward_scheme: e.target.value })}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="http">HTTP</option>
                <option value="https">HTTPS</option>
              </select>
            </div>
            <div>
              <label htmlFor="forward-host" className="block text-sm font-medium text-gray-300 mb-2">Host</label>
              <input
                id="forward-host"
                type="text"
                required
                value={formData.forward_host}
                onChange={e => setFormData({ ...formData, forward_host: e.target.value })}
                placeholder="192.168.1.100"
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label htmlFor="forward-port" className="block text-sm font-medium text-gray-300 mb-2">Port</label>
              <input
                id="forward-port"
                type="number"
                required
                min="1"
                max="65535"
                value={formData.forward_port}
                onChange={e => setFormData({ ...formData, forward_port: parseInt(e.target.value) })}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>

          {/* SSL Certificate Selection */}
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              SSL Certificate (Custom Only)
            </label>
            <select
              value={formData.certificate_id || 0}
              onChange={e => setFormData({ ...formData, certificate_id: parseInt(e.target.value) || null })}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value={0}>Request a new SSL Certificate (Let's Encrypt)</option>
              {certificates.filter(c => c.provider === 'custom').map(cert => (
                <option key={cert.id} value={cert.id}>
                  {cert.name} (Custom)
                </option>
              ))}
            </select>
            <p className="text-xs text-gray-500 mt-1">
              Let's Encrypt certificates are managed automatically. Use custom certificates for self-signed or other providers.
            </p>
          </div>

          {/* Access Control List */}
          <AccessListSelector
            value={formData.access_list_id || null}
            onChange={id => setFormData({ ...formData, access_list_id: id })}
          />

          {/* Application Preset */}
          <div>
            <label htmlFor="application-preset" className="block text-sm font-medium text-gray-300 mb-2">
              Application Preset
              <span className="text-gray-500 font-normal ml-2">(Optional)</span>
            </label>
            <select
              id="application-preset"
              value={formData.application}
              onChange={e => {
                const preset = e.target.value as ApplicationPreset
                // Auto-enable websockets for apps that need it
                const needsWebsockets = ['plex', 'jellyfin', 'emby', 'homeassistant', 'vaultwarden'].includes(preset)
                setFormData({
                  ...formData,
                  application: preset,
                  websocket_support: needsWebsockets || formData.websocket_support,
                })
              }}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              {APPLICATION_PRESETS.map(preset => (
                <option key={preset.value} value={preset.value}>
                  {preset.label} - {preset.description}
                </option>
              ))}
            </select>
            <p className="text-xs text-gray-500 mt-1">
              Presets automatically configure headers for remote access behind tunnels/CGNAT.
            </p>
          </div>

          {/* Application Config Helper */}
          {formData.application !== 'none' && formData.domain_names && (
            <div className="bg-blue-900/20 border border-blue-700 rounded-lg p-4">
              <div className="flex items-start gap-3">
                <Info className="w-5 h-5 text-blue-400 flex-shrink-0 mt-0.5" />
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
                    </>
                  )}

                  {/* Jellyfin/Emby Helper */}
                  {(formData.application === 'jellyfin' || formData.application === 'emby') && cpmpInternalIP && (
                    <>
                      <p className="text-xs text-gray-300">
                        Add this IP to <strong>{formData.application === 'jellyfin' ? 'Jellyfin' : 'Emby'} → Dashboard → Networking → Known Proxies</strong>
                      </p>
                      <div className="flex items-center gap-2">
                        <code className="flex-1 bg-gray-900 px-3 py-2 rounded text-sm text-green-400 font-mono">
                          {cpmpInternalIP}
                        </code>
                        <button
                          type="button"
                          onClick={() => copyToClipboard(cpmpInternalIP, 'proxy-ip')}
                          className="px-3 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded text-sm flex items-center gap-1"
                        >
                          {copiedField === 'proxy-ip' ? <Check size={14} /> : <Copy size={14} />}
                          {copiedField === 'proxy-ip' ? 'Copied!' : 'Copy'}
                        </button>
                      </div>
                    </>
                  )}

                  {/* Home Assistant Helper */}
                  {formData.application === 'homeassistant' && cpmpInternalIP && (
                    <>
                      <p className="text-xs text-gray-300">
                        Add this to your <strong>configuration.yaml</strong> under <code>http:</code>
                      </p>
                      <div className="relative">
                        <pre className="bg-gray-900 px-3 py-2 rounded text-sm text-green-400 font-mono overflow-x-auto">
{`http:
  use_x_forwarded_for: true
  trusted_proxies:
    - ${cpmpInternalIP}`}
                        </pre>
                        <button
                          type="button"
                          onClick={() => copyToClipboard(`http:\n  use_x_forwarded_for: true\n  trusted_proxies:\n    - ${cpmpInternalIP}`, 'ha-yaml')}
                          className="absolute top-2 right-2 px-2 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded text-xs flex items-center gap-1"
                        >
                          {copiedField === 'ha-yaml' ? <Check size={12} /> : <Copy size={12} />}
                          {copiedField === 'ha-yaml' ? 'Copied!' : 'Copy'}
                        </button>
                      </div>
                    </>
                  )}

                  {/* Nextcloud Helper */}
                  {formData.application === 'nextcloud' && cpmpInternalIP && (
                    <>
                      <p className="text-xs text-gray-300">
                        Add this to your <strong>config/config.php</strong>
                      </p>
                      <div className="relative">
                        <pre className="bg-gray-900 px-3 py-2 rounded text-sm text-green-400 font-mono overflow-x-auto">
{`'trusted_proxies' => ['${cpmpInternalIP}'],
'overwriteprotocol' => 'https',`}
                        </pre>
                        <button
                          type="button"
                          onClick={() => copyToClipboard(`'trusted_proxies' => ['${cpmpInternalIP}'],\n'overwriteprotocol' => 'https',`, 'nc-php')}
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
                onChange={e => setFormData({ ...formData, ssl_forced: e.target.checked })}
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
                onChange={e => setFormData({ ...formData, http2_support: e.target.checked })}
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
                onChange={e => setFormData({ ...formData, hsts_enabled: e.target.checked })}
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
                onChange={e => setFormData({ ...formData, hsts_subdomains: e.target.checked })}
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
                onChange={e => setFormData({ ...formData, block_exploits: e.target.checked })}
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
                onChange={e => setFormData({ ...formData, websocket_support: e.target.checked })}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">Websockets Support</span>
              <div title="Needed for apps that update in real-time (like chat, notifications, or live status). If your app feels 'broken' or doesn't update, try turning this on." className="text-gray-500 hover:text-gray-300 cursor-help">
                <CircleHelp size={14} />
              </div>
            </label>
          </div>

          {/* Advanced Config */}
          <div>
            <label htmlFor="advanced-config" className="block text-sm font-medium text-gray-300 mb-2">
              Advanced Caddy Config (Optional)
            </label>
            <textarea
              id="advanced-config"
              value={formData.advanced_config}
              onChange={e => setFormData({ ...formData, advanced_config: e.target.value })}
              placeholder="Additional Caddy directives..."
              rows={4}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white font-mono text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          {/* Enabled Toggle */}
          <div className="flex items-center justify-end pb-2">
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={formData.enabled}
                onChange={e => setFormData({ ...formData, enabled: e.target.checked })}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm font-medium text-white">Enable Proxy Host</span>
            </label>
          </div>

          {/* Actions */}
          <div className="flex gap-3 justify-end pt-4 border-t border-gray-800">
            <button
              type="button"
              onClick={onCancel}
              disabled={loading}
              className="px-6 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              Cancel
            </button>

            <button
              type="button"
              onClick={handleTestConnection}
              disabled={loading || testStatus === 'testing' || !formData.forward_host || !formData.forward_port}
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
              className="px-6 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              {loading ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>

      {/* New Domain Prompt Modal */}
      {showDomainPrompt && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center p-4 z-[60]">
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
    </div>
  )
}
