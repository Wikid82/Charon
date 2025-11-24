import { useState } from 'react'
import { Loader2, ExternalLink } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useProxyHosts } from '../hooks/useProxyHosts'
import { getSettings } from '../api/settings'
import type { ProxyHost } from '../api/proxyHosts'
import ProxyHostForm from '../components/ProxyHostForm'
import { Switch } from '../components/ui/Switch'

export default function ProxyHosts() {
  const { hosts, loading, isFetching, error, createHost, updateHost, deleteHost } = useProxyHosts()
  const [showForm, setShowForm] = useState(false)
  const [editingHost, setEditingHost] = useState<ProxyHost | undefined>()

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
  })

  const linkBehavior = settings?.['ui.domain_link_behavior'] || 'new_tab'

  const handleDomainClick = (e: React.MouseEvent, url: string) => {
    if (linkBehavior === 'new_window') {
      e.preventDefault()
      window.open(url, '_blank', 'noopener,noreferrer,width=1024,height=768')
    }
  }

  const handleAdd = () => {
    setEditingHost(undefined)
    setShowForm(true)
  }

  const handleEdit = (host: ProxyHost) => {
    setEditingHost(host)
    setShowForm(true)
  }

  const handleSubmit = async (data: Partial<ProxyHost>) => {
    if (editingHost) {
      await updateHost(editingHost.uuid, data)
    } else {
      await createHost(data)
    }
    setShowForm(false)
    setEditingHost(undefined)
  }

  const handleDelete = async (uuid: string) => {
    if (confirm('Are you sure you want to delete this proxy host?')) {
      try {
        await deleteHost(uuid)
      } catch (err) {
        alert(err instanceof Error ? err.message : 'Failed to delete')
      }
    }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <h1 className="text-3xl font-bold text-white">Proxy Hosts</h1>
          {isFetching && !loading && <Loader2 className="animate-spin text-blue-400" size={24} />}
        </div>
        <button
          onClick={handleAdd}
          className="px-4 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors"
        >
          Add Proxy Host
        </button>
      </div>

      {error && (
        <div className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded mb-6">
          {error}
        </div>
      )}

      <div className="bg-dark-card rounded-lg border border-gray-800 overflow-hidden">
        {loading ? (
          <div className="text-center text-gray-400 py-12">Loading...</div>
        ) : hosts.length === 0 ? (
          <div className="text-center text-gray-400 py-12">
            No proxy hosts configured yet. Click "Add Proxy Host" to get started.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-gray-900 border-b border-gray-800">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                    Domain
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                    Forward To
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                    SSL
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
                {hosts.map((host) => (
                  <tr key={host.uuid} className="hover:bg-gray-900/50">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-medium text-white">
                        {host.domain_names.split(',').map((domain, i) => {
                          const url = `${host.ssl_forced ? 'https' : 'http'}://${domain.trim()}`
                          return (
                            <div key={i} className="flex items-center gap-1">
                              <a
                                href={url}
                                target={linkBehavior === 'same_tab' ? '_self' : '_blank'}
                                rel="noopener noreferrer"
                                onClick={(e) => handleDomainClick(e, url)}
                                className="hover:text-blue-400 hover:underline flex items-center gap-1"
                              >
                                {domain.trim()}
                                <ExternalLink size={12} className="opacity-50" />
                              </a>
                            </div>
                          )
                        })}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-gray-300">
                        {host.forward_scheme}://{host.forward_host}:{host.forward_port}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex gap-2">
                        {host.ssl_forced && (
                          <span className="px-2 py-1 text-xs bg-green-900/30 text-green-400 rounded">
                            SSL
                          </span>
                        )}
                        {host.websocket_support && (
                          <span className="px-2 py-1 text-xs bg-blue-900/30 text-blue-400 rounded">
                            WS
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center gap-2">
                        <Switch
                          checked={host.enabled}
                          onCheckedChange={(checked) => updateHost(host.uuid, { enabled: checked })}
                        />
                        <span className={`text-sm ${host.enabled ? 'text-green-400' : 'text-gray-400'}`}>
                          {host.enabled ? 'Enabled' : 'Disabled'}
                        </span>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <button
                        onClick={() => handleEdit(host)}
                        className="text-blue-400 hover:text-blue-300 mr-4"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(host.uuid)}
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
      </div>

      {showForm && (
        <ProxyHostForm
          host={editingHost}
          onSubmit={handleSubmit}
          onCancel={() => {
            setShowForm(false)
            setEditingHost(undefined)
          }}
        />
      )}
    </div>
  )
}
