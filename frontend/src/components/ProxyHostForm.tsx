import { useState, useEffect } from 'react'
import { CircleHelp, AlertCircle, Check, X, Loader2 } from 'lucide-react'
import type { ProxyHost } from '../api/proxyHosts'
import { testProxyHostConnection } from '../api/proxyHosts'
import { useRemoteServers } from '../hooks/useRemoteServers'
import { useDomains } from '../hooks/useDomains'
import { useDocker } from '../hooks/useDocker'
import { parse } from 'tldts'

interface ProxyHostFormProps {
  host?: ProxyHost
  onSubmit: (data: Partial<ProxyHost>) => Promise<void>
  onCancel: () => void
}

export default function ProxyHostForm({ host, onSubmit, onCancel }: ProxyHostFormProps) {
  const [formData, setFormData] = useState({
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
    advanced_config: host?.advanced_config || '',
    enabled: host?.enabled ?? true,
  })

  const { servers: remoteServers } = useRemoteServers()
  const { domains, createDomain } = useDomains()
  const [connectionSource, setConnectionSource] = useState<'local' | 'custom' | string>('custom')
  const [selectedDomain, setSelectedDomain] = useState('')
  const [selectedContainerId, setSelectedContainerId] = useState<string>('')

  // New Domain Popup State
  const [showDomainPrompt, setShowDomainPrompt] = useState(false)
  const [pendingDomain, setPendingDomain] = useState('')
  const [dontAskAgain, setDontAskAgain] = useState(false)

  // Test Connection State
  const [testStatus, setTestStatus] = useState<'idle' | 'testing' | 'success' | 'error'>('idle')

  useEffect(() => {
    const stored = localStorage.getItem('cpmp_dont_ask_domain')
    if (stored === 'true') {
      setDontAskAgain(true)
    }
  }, [])

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

  // Fetch containers based on selected source
  // If 'local', host is undefined (which defaults to local socket in backend)
  // If remote UUID, we need to find the server and get its host address?
  // Actually, the backend ListContainers takes a 'host' query param.
  // If it's a remote server, we should probably pass the UUID or the host address.
  // Looking at backend/internal/services/docker_service.go, it takes a 'host' string.
  // If it's a remote server, we need to pass the TCP address (e.g. tcp://1.2.3.4:2375).

  const getDockerHostString = () => {
    if (connectionSource === 'local') return undefined;
    if (connectionSource === 'custom') return null;
    const server = remoteServers.find(s => s.uuid === connectionSource);
    if (!server) return null;
    // Construct the Docker host string
    return `tcp://${server.host}:${server.port}`;
  }

  const { containers: dockerContainers, isLoading: dockerLoading, error: dockerError } = useDocker(getDockerHostString())
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)

    try {
      await onSubmit(formData)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save proxy host')
    } finally {
      setLoading(false)
    }
  }

  const handleContainerSelect = (containerId: string) => {
    setSelectedContainerId(containerId)
    const container = dockerContainers.find(c => c.id === containerId)
    if (container) {
      // Prefer internal IP if available, otherwise use container name
      const host = container.ip || container.names[0]
      // Use the first exposed port if available, otherwise default to 80
      const port = container.ports && container.ports.length > 0 ? container.ports[0].private_port : 80

      let newDomainNames = formData.domain_names
      if (selectedDomain) {
        const subdomain = container.names[0].replace(/^\//, '')
        newDomainNames = `${subdomain}.${selectedDomain}`
      }

      setFormData({
        ...formData,
        forward_host: host,
        forward_port: port,
        forward_scheme: 'http',
        domain_names: newDomainNames,
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
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
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
              <label className="block text-sm font-medium text-gray-300 mb-2">
                Domain Names (comma-separated)
              </label>
              <input
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
        <div className="fixed inset-0 bg-black bg-opacity-70 flex items-center justify-center p-4 z-[60]">
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
