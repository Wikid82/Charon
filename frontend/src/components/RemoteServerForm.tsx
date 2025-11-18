import { useState } from 'react'
import { RemoteServer } from '../hooks/useRemoteServers'
import { remoteServersAPI } from '../services/api'

interface RemoteServerFormProps {
  server?: RemoteServer
  onSubmit: (data: Partial<RemoteServer>) => Promise<void>
  onCancel: () => void
}

export default function RemoteServerForm({ server, onSubmit, onCancel }: RemoteServerFormProps) {
  const [formData, setFormData] = useState({
    name: server?.name || '',
    provider: server?.provider || 'generic',
    host: server?.host || '',
    port: server?.port || 80,
    username: server?.username || '',
    enabled: server?.enabled ?? true,
  })

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [testResult, setTestResult] = useState<any | null>(null)
  const [testing, setTesting] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)

    try {
      await onSubmit(formData)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save remote server')
    } finally {
      setLoading(false)
    }
  }

  const handleTestConnection = async () => {
    if (!server) return

    setTesting(true)
    setTestResult(null)
    setError(null)

    try {
      const result = await remoteServersAPI.test(server.uuid)
      setTestResult(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to test connection')
    } finally {
      setTesting(false)
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

        <form onSubmit={handleSubmit} className="p-6 space-y-4">
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

          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">Provider</label>
            <select
              value={formData.provider}
              onChange={e => setFormData({ ...formData, provider: e.target.value })}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="generic">Generic</option>
              <option value="docker">Docker</option>
              <option value="kubernetes">Kubernetes</option>
              <option value="aws">AWS</option>
              <option value="gcp">GCP</option>
              <option value="azure">Azure</option>
            </select>
          </div>

          <div className="grid grid-cols-2 gap-4">
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
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">Port</label>
              <input
                type="number"
                required
                min="1"
                max="65535"
                value={formData.port}
                onChange={e => setFormData({ ...formData, port: parseInt(e.target.value) })}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Username (Optional)
            </label>
            <input
              type="text"
              value={formData.username}
              onChange={e => setFormData({ ...formData, username: e.target.value })}
              placeholder="admin"
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
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

          {/* Connection Test */}
          {server && (
            <div className="pt-4 border-t border-gray-800">
              <button
                type="button"
                onClick={handleTestConnection}
                disabled={testing}
                className="w-full px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
              >
                {testing ? (
                  <>
                    <span className="animate-spin">⏳</span>
                    Testing Connection...
                  </>
                ) : (
                  <>
                    <span>🔌</span>
                    Test Connection
                  </>
                )}
              </button>
              {testResult && (
                <div className={`mt-3 p-3 rounded-lg ${testResult.reachable ? 'bg-green-900/20 border border-green-500' : 'bg-red-900/20 border border-red-500'}`}>
                  <div className="flex items-center gap-2">
                    <span className={testResult.reachable ? 'text-green-400' : 'text-red-400'}>
                      {testResult.reachable ? '✓ Connection Successful' : '✗ Connection Failed'}
                    </span>
                  </div>
                  {testResult.error && (
                    <div className="text-xs text-red-300 mt-1">{testResult.error}</div>
                  )}
                  {testResult.address && (
                    <div className="text-xs text-gray-400 mt-1">Address: {testResult.address}</div>
                  )}
                </div>
              )}
            </div>
          )}

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
              {loading ? 'Saving...' : (server ? 'Update' : 'Create')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
