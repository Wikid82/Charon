import { Activity, Shield, ShieldAlert, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '../ui'

import type { DashboardSummary } from '../../api/crowdsecDashboard'

interface DashboardSummaryCardsProps {
  data: DashboardSummary | undefined
  isLoading: boolean
  isError: boolean
}

export function DashboardSummaryCards({ data, isLoading, isError }: DashboardSummaryCardsProps) {
  const { t } = useTranslation()

  if (isLoading) {
    return (
      <div data-testid="dashboard-summary-cards" className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="rounded-lg border border-gray-700 bg-gray-900 p-4 space-y-3">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-8 w-16" />
            <Skeleton className="h-3 w-32" />
          </div>
        ))}
      </div>
    )
  }

  if (isError || !data) {
    return (
      <div data-testid="dashboard-summary-cards" className="rounded-lg border border-red-700/50 bg-red-900/20 p-4">
        <p className="text-sm text-red-300">{t('security.crowdsec.dashboard.summaryError', 'Failed to load summary data.')}</p>
      </div>
    )
  }

  const trendLabel = data.decisions_trend > 0
    ? `+${data.decisions_trend.toFixed(1)}%`
    : data.decisions_trend < 0
      ? `${data.decisions_trend.toFixed(1)}%`
      : '0%'

  const trendColor = data.decisions_trend > 0
    ? 'text-red-400'
    : data.decisions_trend < 0
      ? 'text-green-400'
      : 'text-gray-400'

  const cards = [
    {
      title: t('security.crowdsec.dashboard.totalDecisions', 'Total Decisions'),
      value: data.total_decisions.toLocaleString(),
      icon: Activity,
      subtitle: trendLabel,
      subtitleColor: trendColor,
    },
    {
      title: t('security.crowdsec.dashboard.activeDecisions', 'Active Decisions'),
      value: data.active_decisions === -1 ? 'N/A' : data.active_decisions.toLocaleString(),
      icon: ShieldAlert,
      subtitle: data.active_decisions === -1
        ? t('security.crowdsec.dashboard.lapiUnavailable', 'LAPI unavailable')
        : t('security.crowdsec.dashboard.currentlyEnforced', 'Currently enforced'),
      subtitleColor: data.active_decisions === -1 ? 'text-yellow-400' : 'text-gray-400',
    },
    {
      title: t('security.crowdsec.dashboard.uniqueIPs', 'Unique IPs'),
      value: data.unique_ips.toLocaleString(),
      icon: Users,
      subtitle: t('security.crowdsec.dashboard.distinctAttackers', 'Distinct attackers'),
      subtitleColor: 'text-gray-400',
    },
    {
      title: t('security.crowdsec.dashboard.topScenario', 'Top Scenario'),
      value: data.top_scenario ? data.top_scenario.split('/').pop() ?? data.top_scenario : '—',
      icon: Shield,
      subtitle: data.top_scenario || t('security.crowdsec.dashboard.noData', 'No data'),
      subtitleColor: 'text-gray-400',
    },
  ]

  return (
    <div data-testid="dashboard-summary-cards" aria-live="polite" className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      {cards.map((card) => (
        <div key={card.title} className="rounded-lg border border-gray-700 bg-gray-900 p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm text-gray-400">{card.title}</span>
            <card.icon className="h-4 w-4 text-gray-500" aria-hidden="true" />
          </div>
          <p className="text-2xl font-bold text-white">{card.value}</p>
          <p className={`text-xs mt-1 ${card.subtitleColor}`}>{card.subtitle}</p>
        </div>
      ))}
    </div>
  )
}
