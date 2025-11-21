import { useState } from 'react'
import { useRemoteServers } from '../hooks/useRemoteServers'
import type { RemoteServer } from '../api/remoteServers'
import RemoteServerForm from '../components/RemoteServerForm'

export default function RemoteServers() {
  const { servers, loading, error, createServer, updateServer, deleteServer } = useRemoteServers()
  const [showForm, setShowForm] = useState(false)
  const [editingServer, setEditingServer] = useState<RemoteServer | undefined>()
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid')

  const handleAdd = () => {
    setEditingServer(undefined)
    setShowForm(true)
  }

  const handleEdit = (server: RemoteServer) => {
    setEditingServer(server)
    setShowForm(true)
  }

  const handleSubmit = async (data: Partial<RemoteServer>) => {
    if (editingServer) {
      await updateServer(editingServer.uuid, data)
    } else {
      await createServer(data)
    }
    setShowForm(false)
    setEditingServer(undefined)
  }

  const handleDelete = async (uuid: string) => {
    if (confirm('Are you sure you want to delete this remote server?')) {
      try {
        await deleteServer(uuid)
      } catch (err) {
        alert(err instanceof Error ? err.message : 'Failed to delete')
      }
    }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-3xl font-bold text-white">Remote Servers</h1>
        <div className="flex gap-3">
          <div className="flex bg-gray-800 rounded-lg p-1">
            <button
              onClick={() => setViewMode('grid')}
              className={`px-3 py-1 rounded text-sm ${
                viewMode === 'grid'
                  ? 'bg-blue-active text-white'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              Grid
            </button>
            <button
              onClick={() => setViewMode('list')}
              className={`px-3 py-1 rounded text-sm ${
                viewMode === 'list'
                  ? 'bg-blue-active text-white'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              List
            </button>
          </div>
          <button
            onClick={handleAdd}
            className="px-4 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors"
          >
            Add Server
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded mb-6">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-center text-gray-400 py-12">Loading...</div>
      ) : servers.length === 0 ? (
        <div className="bg-dark-card rounded-lg border border-gray-800 p-6">
          <div className="text-center text-gray-400 py-12">
            No remote servers configured. Add servers to quickly select backends when creating proxy hosts.
          </div>
        </div>
      ) : viewMode === 'grid' ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {servers.map((server) => (
            <div
              key={server.uuid}
              className="bg-dark-card rounded-lg border border-gray-800 p-6 hover:border-gray-700 transition-colors"
            >
              <div className="flex items-start justify-between mb-4">
                <div>
                  <h3 className="text-lg font-semibold text-white mb-1">{server.name}</h3>
                  <span className="inline-block px-2 py-1 text-xs bg-gray-800 text-gray-400 rounded">
                    {server.provider}
                  </span>
                </div>
                <span
                  className={`px-2 py-1 text-xs rounded ${
                    server.enabled
                      ? 'bg-green-900/30 text-green-400'
                      : 'bg-gray-700 text-gray-400'
                  }`}
                >
                  {server.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
              <div className="space-y-2 mb-4">
                <div className="flex items-center gap-2 text-sm">
                  <span className="text-gray-400">Host:</span>
                  <span className="text-white font-mono">{server.host}</span>
                </div>
                <div className="flex items-center gap-2 text-sm">
                  <span className="text-gray-400">Port:</span>
                  <span className="text-white font-mono">{server.port}</span>
                </div>
                {server.username && (
                  <div className="flex items-center gap-2 text-sm">
                    <span className="text-gray-400">User:</span>
                    <span className="text-white font-mono">{server.username}</span>
                  </div>
                )}
              </div>
              <div className="flex gap-2 pt-4 border-t border-gray-800">
                <button
                  onClick={() => handleEdit(server)}
                  className="flex-1 px-3 py-2 bg-gray-700 hover:bg-gray-600 text-white text-sm rounded-lg font-medium transition-colors"
                >
                  Edit
                </button>
                <button
                  onClick={() => handleDelete(server.uuid)}
                  className="flex-1 px-3 py-2 bg-red-900/20 hover:bg-red-900/30 text-red-400 text-sm rounded-lg font-medium transition-colors"
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="bg-dark-card rounded-lg border border-gray-800 overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-900 border-b border-gray-800">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                  Name
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                  Provider
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                  Host
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                  Port
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-right text-xs font-medium text-gray-400 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {servers.map((server) => (
                <tr key={server.uuid} className="hover:bg-gray-900/50">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm font-medium text-white">{server.name}</div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span className="px-2 py-1 text-xs bg-gray-800 text-gray-400 rounded">
                      {server.provider}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-gray-300 font-mono">{server.host}</div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-gray-300 font-mono">{server.port}</div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`px-2 py-1 text-xs rounded ${
                        server.enabled
                          ? 'bg-green-900/30 text-green-400'
                          : 'bg-gray-700 text-gray-400'
                      }`}
                    >
                      {server.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    <button
                      onClick={() => handleEdit(server)}
                      className="text-blue-400 hover:text-blue-300 mr-4"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => handleDelete(server.uuid)}
                      className="text-red-400 hover:text-red-300"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showForm && (
        <RemoteServerForm
          server={editingServer}
          onSubmit={handleSubmit}
          onCancel={() => {
            setShowForm(false)
            setEditingServer(undefined)
          }}
        />
      )}
    </div>
  )
}
