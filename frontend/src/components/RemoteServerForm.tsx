import { useEffect, useState } from 'react'
import { type RemoteServer, testRemoteServerConnection } from '../api/remoteServers'

interface Props {
  server?: RemoteServer
  onSubmit: (data: Partial<RemoteServer>) => Promise<void>
  onCancel: () => void
}

export default function RemoteServerForm({ server, onSubmit, onCancel }: Props) {
  const [formData, setFormData] = useState({
    name: server?.name || '',
    provider: server?.provider || 'generic',
    host: server?.host || '',
    port: server?.port ?? 22,
    username: server?.username || '',
    enabled: server?.enabled ?? true,
  })

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setFormData({
      name: server?.name || '',
      provider: server?.provider || 'generic',
      host: server?.host || '',
      port: server?.port ?? 22,
      username: server?.username || '',
      enabled: server?.enabled ?? true,
    })
  }, [server])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)
    try {
      await onSubmit(formData)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save server')
    } finally {
      setLoading(false)
    }
  }

  const handleTestConnection = async () => {
    if (!server?.uuid) return
    setLoading(true)
    setError(null)
    try {
      const result = await testRemoteServerConnection(server.uuid)
      alert(`Connection successful: ${result.address}`)
    } catch {
      setError('Connection failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
      <div className="bg-dark-card rounded-lg border border-gray-800 max-w-lg w-full">
        <div className="p-6 border-b border-gray-800">
          <h2 className="text-2xl font-bold text-white">
            {server ? 'Edit Remote Server' : 'Add Remote Server'}
          </h2>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          {error && (
            <div className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded">
              {error}
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">Name</label>
            <input
              type="text"
              required
              value={formData.name}
              onChange={e => setFormData({ ...formData, name: e.target.value })}
              placeholder="My Production Server"
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">Provider</label>
              <select
                value={formData.provider}
                onChange={e => {
                  const newProvider = e.target.value;
                  setFormData({
                    ...formData,
                    provider: newProvider,
                    // Set default port for Docker
                    port: newProvider === 'docker' ? 2375 : (newProvider === 'generic' ? 22 : formData.port)
                  })
                }}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="generic">Generic</option>
                <option value="docker">Docker</option>
                <option value="kubernetes">Kubernetes</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">Host</label>
              <input
                type="text"
                required
                value={formData.host}
                onChange={e => setFormData({ ...formData, host: e.target.value })}
                placeholder="192.168.1.100"
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">Port</label>
              <input
                type="number"
                min={1}
                max={65535}
                value={formData.port}
                onChange={e => setFormData({ ...formData, port: parseInt(e.target.value) })}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            {formData.provider !== 'docker' && (
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">Username</label>
                <input
                  type="text"
                  value={formData.username}
                  onChange={e => setFormData({ ...formData, username: e.target.value })}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            )}
          </div>

          <label className="flex items-center gap-3">
            <input
              type="checkbox"
              checked={formData.enabled}
              onChange={e => setFormData({ ...formData, enabled: e.target.checked })}
              className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
            />
            <span className="text-sm text-gray-300">Enabled</span>
          </label>

          <div className="flex gap-3 justify-end pt-4 border-t border-gray-800">
            {server && (
              <button
                type="button"
                onClick={handleTestConnection}
                disabled={loading}
                className="px-6 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors disabled:opacity-50 mr-auto"
              >
                Test Connection
              </button>
            )}
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
              {loading ? 'Saving...' : (server ? 'Update' : 'Create')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
