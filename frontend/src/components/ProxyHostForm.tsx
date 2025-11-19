import { useState, useEffect } from 'react'
import { ProxyHost, Location } from '../hooks/useProxyHosts'
import { remoteServersAPI } from '../services/api'

interface ProxyHostFormProps {
  host?: ProxyHost
  onSubmit: (data: Partial<ProxyHost>) => Promise<void>
  onCancel: () => void
}

interface RemoteServer {
  uuid: string
  name: string
  provider: string
  host: string
  port: number
  enabled: boolean
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
    websocket_support: host?.websocket_support ?? false,
    locations: host?.locations || [] as Location[],
    enabled: host?.enabled ?? true,
  })

  const [remoteServers, setRemoteServers] = useState<RemoteServer[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchServers = async () => {
      try {
        const servers = await remoteServersAPI.list(true)
        setRemoteServers(servers)
      } catch (err) {
        console.error('Failed to fetch remote servers:', err)
      }
    }
    fetchServers()
  }, [])

  const addLocation = () => {
    setFormData({
      ...formData,
      locations: [
        ...formData.locations,
        { path: '/', forward_scheme: 'http', forward_host: '', forward_port: 80 }
      ]
    })
  }

  const removeLocation = (index: number) => {
    const newLocations = [...formData.locations]
    newLocations.splice(index, 1)
    setFormData({ ...formData, locations: newLocations })
  }

  const updateLocation = (index: number, field: keyof Location, value: any) => {
    const newLocations = [...formData.locations]
    newLocations[index] = { ...newLocations[index], [field]: value }
    setFormData({ ...formData, locations: newLocations })
  }

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

          {/* Remote Server Quick Select */}
          {remoteServers.length > 0 && (
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">
                Quick Select from Remote Servers
              </label>
              <select
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

          {/* Forward Details */}
          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">Scheme</label>
              <select
                value={formData.forward_scheme}
                onChange={e => setFormData({ ...formData, forward_scheme: e.target.value })}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="http">HTTP</option>
                <option value="https">HTTPS</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">Host</label>
              <input
                type="text"
                required
                value={formData.forward_host}
                onChange={e => setFormData({ ...formData, forward_host: e.target.value })}
                placeholder="192.168.1.100"
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">Port</label>
              <input
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
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.http2_support}
                onChange={e => setFormData({ ...formData, http2_support: e.target.checked })}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">HTTP/2 Support</span>
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.hsts_enabled}
                onChange={e => setFormData({ ...formData, hsts_enabled: e.target.checked })}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">HSTS Enabled</span>
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.hsts_subdomains}
                onChange={e => setFormData({ ...formData, hsts_subdomains: e.target.checked })}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">HSTS Subdomains</span>
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.block_exploits}
                onChange={e => setFormData({ ...formData, block_exploits: e.target.checked })}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">Block Common Exploits</span>
            </label>
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={formData.websocket_support}
                onChange={e => setFormData({ ...formData, websocket_support: e.target.checked })}
                className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">WebSocket Support</span>
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

          {/* Custom Locations */}
          <div>
            <div className="flex items-center justify-between mb-4">
              <label className="block text-sm font-medium text-gray-300">
                Custom Locations
              </label>
              <button
                type="button"
                onClick={addLocation}
                className="px-3 py-1 bg-gray-700 hover:bg-gray-600 text-white text-sm rounded transition-colors"
              >
                Add Location
              </button>
            </div>

            <div className="space-y-4">
              {formData.locations.map((location, index) => (
                <div key={index} className="bg-gray-900/50 p-4 rounded-lg border border-gray-700">
                  <div className="grid grid-cols-12 gap-4 mb-2">
                    <div className="col-span-4">
                      <label className="block text-xs text-gray-400 mb-1">Path</label>
                      <input
                        type="text"
                        value={location.path}
                        onChange={e => updateLocation(index, 'path', e.target.value)}
                        placeholder="/api"
                        className="w-full bg-gray-800 border border-gray-600 rounded px-3 py-1 text-sm text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
                      />
                    </div>
                    <div className="col-span-2">
                      <label className="block text-xs text-gray-400 mb-1">Scheme</label>
                      <select
                        value={location.forward_scheme}
                        onChange={e => updateLocation(index, 'forward_scheme', e.target.value)}
                        className="w-full bg-gray-800 border border-gray-600 rounded px-3 py-1 text-sm text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
                      >
                        <option value="http">HTTP</option>
                        <option value="https">HTTPS</option>
                      </select>
                    </div>
                    <div className="col-span-4">
                      <label className="block text-xs text-gray-400 mb-1">Forward Host</label>
                      <input
                        type="text"
                        value={location.forward_host}
                        onChange={e => updateLocation(index, 'forward_host', e.target.value)}
                        placeholder="10.0.0.1"
                        className="w-full bg-gray-800 border border-gray-600 rounded px-3 py-1 text-sm text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
                      />
                    </div>
                    <div className="col-span-2">
                      <label className="block text-xs text-gray-400 mb-1">Port</label>
                      <input
                        type="number"
                        value={location.forward_port}
                        onChange={e => updateLocation(index, 'forward_port', parseInt(e.target.value))}
                        className="w-full bg-gray-800 border border-gray-600 rounded px-3 py-1 text-sm text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
                      />
                    </div>
                  </div>
                  <div className="flex justify-end">
                    <button
                      type="button"
                      onClick={() => removeLocation(index)}
                      className="text-red-400 hover:text-red-300 text-xs"
                    >
                      Remove Location
                    </button>
                  </div>
                </div>
              ))}
              {formData.locations.length === 0 && (
                <div className="text-center text-gray-500 text-sm py-4 border border-dashed border-gray-700 rounded-lg">
                  No custom locations defined
                </div>
              )}
            </div>
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
