import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Globe, Server, FileKey, Activity, CheckCircle2, AlertTriangle, Settings } from 'lucide-react'
import { useMemo, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { checkHealth } from '../api/health'
import type { StatsPeriod, StatsBucket } from '../api/stats'
import { PageShell } from '../components/layout/PageShell'
import {
  RequestCountWidget,
  ServiceHealthWidget,
  CertExpiryList,
  TrafficVolumeChart,
  TopHostsChart,
  StatusDistributionChart,
  PeriodSelector,
  BucketSelector,
} from '../components/stats'
import { StatsCard, Skeleton, Switch } from '../components/ui'
import UptimeWidget from '../components/UptimeWidget'
import { useAccessLists } from '../hooks/useAccessLists'
import { useCertificates } from '../hooks/useCertificates'
import { useProxyHosts } from '../hooks/useProxyHosts'
import { useRemoteServers } from '../hooks/useRemoteServers'
import {
  useStatsSummary,
  useTopHosts,
  useStatusDistribution,
  useTrafficVolume,
  useCertExpiry,
  useStatsHealth,
} from '../hooks/useStats'
import { useStatsWebSocket } from '../hooks/useStatsWebSocket'
import { useWidgetVisibility } from '../hooks/useWidgetVisibility'
import type { WidgetKey } from '../hooks/useWidgetVisibility'

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

const WIDGET_LABELS: Record<WidgetKey, string> = {
  requestCount: 'Request Counts',
  trafficVolume: 'Traffic Volume',
  topHosts: 'Top Hosts',
  statusDistribution: 'Status Distribution',
  certExpiry: 'Certificate Expiry',
  serviceHealth: 'Service Health',
}

const WIDGET_KEYS: WidgetKey[] = [
  'requestCount',
  'serviceHealth',
  'certExpiry',
  'trafficVolume',
  'topHosts',
  'statusDistribution',
]

export default function Dashboard() {
  const { t } = useTranslation()
  const { hosts, loading: hostsLoading } = useProxyHosts()
  const { servers, loading: serversLoading } = useRemoteServers()
  const { data: accessLists, isLoading: accessListsLoading } = useAccessLists()
  const queryClient = useQueryClient()

  // Stats state
  const [period, setPeriod] = useState<StatsPeriod>('24h')
  const [bucket, setBucket] = useState<StatsBucket>('1h')
  const [customizeOpen, setCustomizeOpen] = useState(false)

  // Widget visibility
  const { visibility, toggleWidget } = useWidgetVisibility()

  // WebSocket for live stats updates
  const { connected: wsConnected } = useStatsWebSocket()

  // Stats data hooks
  const summaryResult = useStatsSummary(wsConnected)
  const topHostsResult = useTopHosts(period)
  const statusDistResult = useStatusDistribution(period)
  const trafficResult = useTrafficVolume(bucket)
  const certExpiryResult = useCertExpiry(30)
  const statsHealthResult = useStatsHealth()

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

  const allHidden = WIDGET_KEYS.every(key => !visibility[key])

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

      {/* Dashboard Statistics */}
      <section aria-labelledby="stats-section-heading" className="space-y-4">
        {/* Section header with period selector and customize button */}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 id="stats-section-heading" className="text-lg font-semibold text-content-primary">
            {t('dashboard.statistics', 'Statistics')}
          </h2>
          <div className="flex items-center gap-2">
            <PeriodSelector value={period} onChange={setPeriod} />
            <button
              type="button"
              onClick={() => setCustomizeOpen(prev => !prev)}
              aria-expanded={customizeOpen}
              aria-controls="stats-customize-panel"
              className="flex items-center gap-1.5 rounded-md border border-border bg-surface-elevated px-3 py-1.5 text-sm text-content-secondary transition-colors hover:bg-surface-hover hover:text-content-primary"
            >
              <Settings className="h-4 w-4" />
              Customize
            </button>
          </div>
        </div>

        {/* Inline customization panel */}
        {customizeOpen && (
          <div
            id="stats-customize-panel"
            className="rounded-lg border border-border bg-surface-elevated p-4 space-y-3"
          >
            <p className="text-sm font-medium text-content-primary">Show / hide widgets</p>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {WIDGET_KEYS.map((key) => (
                <label
                  key={key}
                  className="flex items-center gap-3 cursor-pointer select-none"
                  htmlFor={`widget-toggle-${key}`}
                >
                  <Switch
                    id={`widget-toggle-${key}`}
                    checked={visibility[key]}
                    onCheckedChange={() => toggleWidget(key)}
                  />
                  <span className="text-sm text-content-secondary">{WIDGET_LABELS[key]}</span>
                </label>
              ))}
            </div>
          </div>
        )}

        {allHidden ? (
          <p className="text-sm text-content-muted py-8 text-center">(all hidden)</p>
        ) : (
          <>
            {/* Top row: Request counts, WS health, cert expiry — 1 col → 3 cols on lg */}
            {(visibility.requestCount || visibility.serviceHealth || visibility.certExpiry) && (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                {visibility.requestCount && (
                  <RequestCountWidget
                    summary={summaryResult.data}
                    isLoading={summaryResult.isLoading}
                  />
                )}
                {visibility.serviceHealth && (
                  <ServiceHealthWidget
                    health={statsHealthResult.data}
                    isLoading={statsHealthResult.isLoading}
                    wsConnected={wsConnected}
                  />
                )}
                {visibility.certExpiry && (
                  <CertExpiryList
                    data={certExpiryResult.data}
                    isLoading={certExpiryResult.isLoading}
                  />
                )}
              </div>
            )}

            {/* Traffic volume chart with bucket selector */}
            {visibility.trafficVolume && (
              <div className="space-y-2">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <h3 className="text-sm font-medium text-content-secondary">
                    {t('dashboard.trafficVolume', 'Traffic Volume')}
                  </h3>
                  <BucketSelector value={bucket} onChange={setBucket} />
                </div>
                <TrafficVolumeChart
                  data={trafficResult.data}
                  isLoading={trafficResult.isLoading}
                  bucket={bucket}
                />
              </div>
            )}

            {/* Bottom row: Top hosts + status distribution — 1 col → 2 cols on sm */}
            {(visibility.topHosts || visibility.statusDistribution) && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {visibility.topHosts && (
                  <TopHostsChart
                    data={topHostsResult.data}
                    isLoading={topHostsResult.isLoading}
                  />
                )}
                {visibility.statusDistribution && (
                  <StatusDistributionChart
                    data={statusDistResult.data}
                    isLoading={statusDistResult.isLoading}
                  />
                )}
              </div>
            )}
          </>
        )}
      </section>
    </PageShell>
  )
}
