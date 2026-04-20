import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Globe, Server, FileKey, Activity, CheckCircle2, AlertTriangle } from 'lucide-react'
import { useMemo, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { checkHealth } from '../api/health'
import { PageShell } from '../components/layout/PageShell'
import { StatsCard, Skeleton } from '../components/ui'
import UptimeWidget from '../components/UptimeWidget'
import { useAccessLists } from '../hooks/useAccessLists'
import { useCertificates } from '../hooks/useCertificates'
import { useProxyHosts } from '../hooks/useProxyHosts'
import { useRemoteServers } from '../hooks/useRemoteServers'

function StatsCardSkeleton() {
  return (
    <div className="rounded-xl border border-border bg-surface-elevated p-6">
      <div className="flex items-start justify-between">
        <div className="min-w-0 flex-1 space-y-3">
          <Skeleton className="h-4 w-24" />
          <Skeleton className="h-8 w-16" />
          <Skeleton className="h-3 w-20" />
        </div>
        <Skeleton className="h-12 w-12 rounded-lg" />
      </div>
    </div>
  )
}

export default function Dashboard() {
  const { t } = useTranslation()
  const { hosts, loading: hostsLoading } = useProxyHosts()
  const { servers, loading: serversLoading } = useRemoteServers()
  const { data: accessLists, isLoading: accessListsLoading } = useAccessLists()
  const queryClient = useQueryClient()

  // Fetch certificates (polling interval managed via effect below)
  const { certificates, isLoading: certificatesLoading } = useCertificates()

  // Build set of certified domains for pending detection
  // ACME certificates (Let's Encrypt) are auto-managed and don't set certificate_id,
  // so we match by domain name instead
  const hasPendingCerts = useMemo(() => {
    const certifiedDomains = new Set<string>()
    for (const cert of certificates) {
      // Handle missing or undefined domain field
      if (!cert.domains) continue
      for (const d of cert.domains.split(',')) {
        const trimmed = d.trim().toLowerCase()
        if (trimmed) certifiedDomains.add(trimmed)
      }
    }

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
  const { data: health, isLoading: healthLoading } = useQuery({
    queryKey: ['health'],
    queryFn: checkHealth,
    staleTime: 1000 * 60, // 1 minute for health checks
    refetchInterval: 1000 * 60, // Auto-refresh every minute
  })

  const enabledHosts = hosts.filter(h => h.enabled).length
  const enabledServers = servers.filter(s => s.enabled).length
  const enabledAccessLists = accessLists?.filter(a => a.enabled).length ?? 0
  const validCertificates = certificates.filter(c => c.status === 'valid').length

  const isInitialLoading = hostsLoading || serversLoading || accessListsLoading || certificatesLoading

  return (
    <PageShell
      title={t('dashboard.title')}
      description={t('dashboard.description')}
    >
      {/* Stats Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
        {isInitialLoading ? (
          <>
            <StatsCardSkeleton />
            <StatsCardSkeleton />
            <StatsCardSkeleton />
            <StatsCardSkeleton />
            <StatsCardSkeleton />
          </>
        ) : (
          <>
            <StatsCard
              title={t('dashboard.proxyHosts')}
              value={hosts.length}
              icon={<Globe className="h-6 w-6" />}
              href="/proxy-hosts"
              change={enabledHosts > 0 ? {
                value: Math.round((enabledHosts / hosts.length) * 100) || 0,
                trend: 'neutral',
                label: t('common.enabledCount', { count: enabledHosts }),
              } : undefined}
            />

            <StatsCard
              title={t('dashboard.certificateStatus')}
              value={certificates.length}
              icon={<FileKey className="h-6 w-6" />}
              href="/certificates"
              change={validCertificates > 0 ? {
                value: Math.round((validCertificates / certificates.length) * 100) || 0,
                trend: 'neutral',
                label: t('common.validCount', { count: validCertificates }),
              } : undefined}
            />


            <StatsCard
              title={t('dashboard.remoteServers')}
              value={servers.length}
              icon={<Server className="h-6 w-6" />}
              href="/remote-servers"
              change={enabledServers > 0 ? {
                value: Math.round((enabledServers / servers.length) * 100) || 0,
                trend: 'neutral',
                label: t('common.enabledCount', { count: enabledServers }),
              } : undefined}
            />

            <StatsCard
              title={t('dashboard.accessLists')}
              value={accessLists?.length ?? 0}
              icon={<FileKey className="h-6 w-6" />}
              href="/access-lists"
              change={enabledAccessLists > 0 ? {
                value: Math.round((enabledAccessLists / (accessLists?.length ?? 1)) * 100) || 0,
                trend: 'neutral',
                label: t('common.activeCount', { count: enabledAccessLists }),
              } : undefined}
            />

            <StatsCard
              title={t('dashboard.systemStatus')}
              value={healthLoading ? '...' : health?.status === 'ok' ? t('dashboard.healthy') : t('common.error')}
              icon={
                healthLoading ? (
                  <Activity className="h-6 w-6 animate-pulse" />
                ) : health?.status === 'ok' ? (
                  <CheckCircle2 className="h-6 w-6 text-success" />
                ) : (
                  <AlertTriangle className="h-6 w-6 text-error" />
                )
              }
            />
          </>
        )}
      </div>

      {/* Uptime Widget */}
      <div className="grid grid-cols-1 lg:grid-cols-1 gap-4">

        <UptimeWidget />

      </div>
    </PageShell>
  )
}
