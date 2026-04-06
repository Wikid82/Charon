import { useQueryClient } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ActiveDecisionsTable } from './ActiveDecisionsTable'
import { AlertsList } from './AlertsList'
import { BanTimelineChart } from './BanTimelineChart'
import { DashboardSummaryCards } from './DashboardSummaryCards'
import { DashboardTimeRangeSelector } from './DashboardTimeRangeSelector'
import { DecisionsExportButton } from './DecisionsExportButton'
import { ScenarioBreakdownChart } from './ScenarioBreakdownChart'
import { TopAttackingIPsChart } from './TopAttackingIPsChart'
import {
  useDashboardSummary,
  useDashboardTimeline,
  useDashboardTopIPs,
  useDashboardScenarios,
} from '../../hooks/useCrowdsecDashboard'

// NOTE: Notification enrichment skipped — no frontend notification dispatch
// system exists. Gotify/webhook notifications are handled server-side in
// backend/internal/services/.

import type { TimeRange } from '../../api/crowdsecDashboard'

export function CrowdSecDashboard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [range, setRange] = useState<TimeRange>('24h')

  const summary = useDashboardSummary(range)
  const timeline = useDashboardTimeline(range)
  const topIPs = useDashboardTopIPs(range)
  const scenarios = useDashboardScenarios(range)

  const isAnyLoading = summary.isLoading || timeline.isLoading || topIPs.isLoading || scenarios.isLoading

  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: ['crowdsec-dashboard'] })
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <DashboardTimeRangeSelector value={range} onChange={setRange} />
        <div className="flex items-center gap-2">
          <DecisionsExportButton range={range} />
          <button
            type="button"
            onClick={handleRefresh}
            disabled={isAnyLoading}
            className="inline-flex items-center gap-2 rounded-md bg-gray-800 px-3 py-2 text-sm text-gray-300 hover:bg-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 focus-visible:ring-offset-gray-950 disabled:opacity-50 disabled:cursor-not-allowed"
            aria-label={t('security.crowdsec.dashboard.refresh', 'Refresh dashboard data')}
          >
            <RefreshCw className={`h-4 w-4 ${isAnyLoading ? 'animate-spin' : ''}`} aria-hidden="true" />
            {t('security.crowdsec.dashboard.refresh', 'Refresh')}
          </button>
        </div>
      </div>

      <DashboardSummaryCards
        data={summary.data}
        isLoading={summary.isLoading}
        isError={summary.isError}
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <BanTimelineChart
          data={timeline.data?.buckets}
          isLoading={timeline.isLoading}
          isError={timeline.isError}
          range={range}
        />
        <TopAttackingIPsChart
          data={topIPs.data?.ips}
          isLoading={topIPs.isLoading}
          isError={topIPs.isError}
        />
      </div>

      <ScenarioBreakdownChart
        data={scenarios.data?.scenarios}
        isLoading={scenarios.isLoading}
        isError={scenarios.isError}
      />

      {/* TODO: Replace useDashboardTopIPs with a dedicated useActiveDecisions hook backed by /v1/decisions once endpoints are available */}
      <ActiveDecisionsTable
        data={topIPs.data?.ips}
        isLoading={topIPs.isLoading}
        isError={topIPs.isError}
      />

      <AlertsList range={range} />
    </div>
  )
}

export default CrowdSecDashboard
