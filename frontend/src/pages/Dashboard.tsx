import { useMemo, useEffect } from 'react'
import { useProxyHosts } from '../hooks/useProxyHosts'
import { useRemoteServers } from '../hooks/useRemoteServers'
import { useCertificates } from '../hooks/useCertificates'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { checkHealth } from '../api/health'
import { Link } from 'react-router-dom'
import UptimeWidget from '../components/UptimeWidget'
import CertificateStatusCard from '../components/CertificateStatusCard'

export default function Dashboard() {
  const { hosts } = useProxyHosts()
  const { servers } = useRemoteServers()
  const queryClient = useQueryClient()

  // Fetch certificates (polling interval managed via effect below)
  const { certificates } = useCertificates()

  // Build set of certified domains for pending detection
  // ACME certificates (Let's Encrypt) are auto-managed and don't set certificate_id,
  // so we match by domain name instead
  const hasPendingCerts = useMemo(() => {
    const certifiedDomains = new Set<string>()
    certificates.forEach(cert => {
      // Handle missing or undefined domain field
      if (!cert.domain) return
      cert.domain.split(',').forEach(d => {
        const trimmed = d.trim().toLowerCase()
        if (trimmed) certifiedDomains.add(trimmed)
      })
    })

    // Check if any SSL host lacks a certificate
    const sslHosts = hosts.filter(h => h.ssl_forced && h.enabled)
    return sslHosts.some(host => {
      const hostDomains = host.domain_names.split(',').map(d => d.trim().toLowerCase())
      return !hostDomains.some(domain => certifiedDomains.has(domain))
    })
  }, [hosts, certificates])

  // Poll certificates every 15s when there are pending certs
  useEffect(() => {
    if (!hasPendingCerts) return

    const interval = setInterval(() => {
      queryClient.invalidateQueries({ queryKey: ['certificates'] })
    }, 15000)

    return () => clearInterval(interval)
  }, [hasPendingCerts, queryClient])

  // Use React Query for health check - benefits from global caching
  const { data: health } = useQuery({
    queryKey: ['health'],
    queryFn: checkHealth,
    staleTime: 1000 * 60, // 1 minute for health checks
    refetchInterval: 1000 * 60, // Auto-refresh every minute
  })

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

        <CertificateStatusCard certificates={certificates} hosts={hosts} />

        <div className="bg-dark-card p-6 rounded-lg border border-gray-800">
          <div className="text-sm text-gray-400 mb-2">System Status</div>
          <div className={`text-lg font-bold ${health?.status === 'ok' ? 'text-green-400' : 'text-red-400'}`}>
            {health?.status === 'ok' ? 'Healthy' : health ? 'Error' : 'Checking...'}
          </div>
        </div>
      </div>

      {/* Uptime Widget */}
      <div className="mb-8">
        <UptimeWidget />
      </div>

      {/* Quick Actions removed per UI update; Security quick-look will be added later */}
    </div>
  )
}
