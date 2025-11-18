import { useEffect, useState } from 'react'
import { useProxyHosts } from '../hooks/useProxyHosts'
import { useRemoteServers } from '../hooks/useRemoteServers'
import { healthAPI } from '../services/api'
import { Link } from 'react-router-dom'

export default function Dashboard() {
  const { hosts } = useProxyHosts()
  const { servers } = useRemoteServers()
  const [health, setHealth] = useState<{ status: string } | null>(null)

  useEffect(() => {
    const checkHealth = async () => {
      try {
        const result = await healthAPI.check()
        setHealth(result)
      } catch (err) {
        setHealth({ status: 'error' })
      }
    }
    checkHealth()
  }, [])

  const enabledHosts = hosts.filter(h => h.enabled).length
  const enabledServers = servers.filter(s => s.enabled).length

  return (
    <div className="p-8">
      <h1 className="text-3xl font-bold text-white mb-6">Dashboard</h1>
      
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        <Link to="/proxy-hosts" className="bg-dark-card p-6 rounded-lg border border-gray-800 hover:border-gray-700 transition-colors">
          <div className="text-sm text-gray-400 mb-2">Proxy Hosts</div>
          <div className="text-3xl font-bold text-white mb-1">{hosts.length}</div>
          <div className="text-xs text-gray-500">{enabledHosts} enabled</div>
        </Link>
        
        <Link to="/remote-servers" className="bg-dark-card p-6 rounded-lg border border-gray-800 hover:border-gray-700 transition-colors">
          <div className="text-sm text-gray-400 mb-2">Remote Servers</div>
          <div className="text-3xl font-bold text-white mb-1">{servers.length}</div>
          <div className="text-xs text-gray-500">{enabledServers} enabled</div>
        </Link>
        
        <div className="bg-dark-card p-6 rounded-lg border border-gray-800">
          <div className="text-sm text-gray-400 mb-2">SSL Certificates</div>
          <div className="text-3xl font-bold text-white mb-1">0</div>
          <div className="text-xs text-gray-500">Coming soon</div>
        </div>
        
        <div className="bg-dark-card p-6 rounded-lg border border-gray-800">
          <div className="text-sm text-gray-400 mb-2">System Status</div>
          <div className={`text-lg font-bold ${health?.status === 'ok' ? 'text-green-400' : 'text-red-400'}`}>
            {health?.status === 'ok' ? 'Healthy' : health ? 'Error' : 'Checking...'}
          </div>
        </div>
      </div>

      {/* Quick Actions */}
      <div className="bg-dark-card rounded-lg border border-gray-800 p-6">
        <h2 className="text-xl font-semibold text-white mb-4">Quick Actions</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Link
            to="/proxy-hosts"
            className="flex items-center gap-3 p-4 bg-gray-900 hover:bg-gray-800 rounded-lg transition-colors"
          >
            <span className="text-2xl">🌐</span>
            <div>
              <div className="font-medium text-white">Add Proxy Host</div>
              <div className="text-xs text-gray-400">Create a new reverse proxy</div>
            </div>
          </Link>
          
          <Link
            to="/remote-servers"
            className="flex items-center gap-3 p-4 bg-gray-900 hover:bg-gray-800 rounded-lg transition-colors"
          >
            <span className="text-2xl">🖥️</span>
            <div>
              <div className="font-medium text-white">Add Remote Server</div>
              <div className="text-xs text-gray-400">Register a backend server</div>
            </div>
          </Link>
          
          <Link
            to="/import"
            className="flex items-center gap-3 p-4 bg-gray-900 hover:bg-gray-800 rounded-lg transition-colors"
          >
            <span className="text-2xl">📥</span>
            <div>
              <div className="font-medium text-white">Import Caddyfile</div>
              <div className="text-xs text-gray-400">Bulk import from existing config</div>
            </div>
          </Link>
        </div>
      </div>
    </div>
  )
}
