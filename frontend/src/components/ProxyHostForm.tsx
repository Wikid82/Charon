import { useState } from 'react'
import { CircleHelp } from 'lucide-react'
import type { ProxyHost } from '../api/proxyHosts'
import { useRemoteServers } from '../hooks/useRemoteServers'
import { useDocker } from '../hooks/useDocker'

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
    ssl_forced: host?.ssl_forced ?? false,
    http2_support: host?.http2_support ?? false,
    hsts_enabled: host?.hsts_enabled ?? false,
    hsts_subdomains: host?.hsts_subdomains ?? false,
    block_exploits: host?.block_exploits ?? true,
    websocket_support: host?.websocket_support ?? true,
    advanced_config: host?.advanced_config || '',
    enabled: host?.enabled ?? true,
  })

  const { servers: remoteServers } = useRemoteServers()
  const [connectionSource, setConnectionSource] = useState<'local' | string>('local')

  // Fetch containers based on selected source
  // If 'local', host is undefined (which defaults to local socket in backend)
  // If remote UUID, we need to find the server and get its host address?
  // Actually, the backend ListContainers takes a 'host' query param.
  // If it's a remote server, we should probably pass the UUID or the host address.
  // Looking at backend/internal/services/docker_service.go, it takes a 'host' string.
  // If it's a remote server, we need to pass the TCP address (e.g. tcp://1.2.3.4:2375).

  const getDockerHostString = () => {
    if (connectionSource === 'local') return undefined;
    const server = remoteServers.find(s => s.uuid === connectionSource);
    if (!server) return undefined;
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

  const handleServerSelect = (serverUuid: string) => {
    const server = remoteServers.find(s => s.uuid === serverUuid)
    if (server) {
      setFormData({
        ...formData,
        forward_host: server.host,
        forward_port: server.port,
        forward_scheme: 'http',
      })
    }
  }

  const handleContainerSelect = (containerId: string) => {
    const container = dockerContainers.find(c => c.id === containerId)
    if (container) {
      // Prefer internal IP if available, otherwise use container name
      const host = container.ip || container.names[0]
      // Use the first exposed port if available, otherwise default to 80
      const port = container.ports && container.ports.length > 0 ? container.ports[0].private_port : 80

      setFormData({
        ...formData,
        forward_host: host,
        forward_port: port,
        forward_scheme: 'http',
      })
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

          {/* Domain Names */}
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Domain Names (comma-separated)
            </label>
            <input
              type="text"
              required
              value={formData.domain_names}
              onChange={e => setFormData({ ...formData, domain_names: e.target.value })}
              placeholder="example.com, www.example.com"
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* Remote Server Quick Select */}
            {remoteServers.length > 0 && (
              <div>
                <label htmlFor="quick-select-server" className="block text-sm font-medium text-gray-300 mb-2">
                  Quick Select: Remote Server
                </label>
                <select
                  id="quick-select-server"
                  onChange={e => handleServerSelect(e.target.value)}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="">-- Select a server --</option>
                  {remoteServers.map(server => (
                    <option key={server.uuid} value={server.uuid}>
                      {server.name} ({server.host}:{server.port})
                    </option>
                  ))}
                </select>
              </div>
            )}

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
                Quick Select: Container
              </label>

              <select
                id="quick-select-docker"
                onChange={e => handleContainerSelect(e.target.value)}
                disabled={dockerLoading}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
              >
                <option value="">
                  {dockerLoading ? 'Loading containers...' : '-- Select a container --'}
                </option>
                {dockerContainers.map(container => (
                  <option key={container.id} value={container.id}>
                    {container.names[0]} ({container.image})
                  </option>
                ))}
              </select>
              {dockerError && (
                <p className="text-xs text-red-400 mt-1">
                  Failed to connect: {(dockerError as Error).message}
                </p>
              )}
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
              <div title="Redirects all HTTP traffic to HTTPS" className="text-gray-500 hover:text-gray-300 cursor-help">
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
              <div title="Enables HTTP/2 support for better performance" className="text-gray-500 hover:text-gray-300 cursor-help">
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
              <div title="Enables HTTP Strict Transport Security (HSTS)" className="text-gray-500 hover:text-gray-300 cursor-help">
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
              <div title="Applies HSTS to all subdomains" className="text-gray-500 hover:text-gray-300 cursor-help">
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
              <div title="Blocks common exploit attempts (XSS, SQLi, etc.)" className="text-gray-500 hover:text-gray-300 cursor-help">
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
              <div title="Enables WebSocket support for real-time applications" className="text-gray-500 hover:text-gray-300 cursor-help">
                <CircleHelp size={14} />
              </div>
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.enabled}
                onChange={e => setFormData({ ...formData, enabled: e.target.checked })}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">Enabled</span>
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
              type="submit"
              disabled={loading}
              className="px-6 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              {loading ? 'Saving...' : (host ? 'Update' : 'Create')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
