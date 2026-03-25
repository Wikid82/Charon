import { AlertTriangle, ChevronLeft, ChevronRight } from 'lucide-react'
import { useState, useMemo, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { useAlerts } from '../../hooks/useCrowdsecDashboard'
import { Skeleton } from '../ui'

import type { TimeRange } from '../../api/crowdsecDashboard'

interface AlertsListProps {
  range: TimeRange
}

const PAGE_SIZE = 10

function formatRelativeTime(dateStr: string): string {
  const now = Date.now()
  const then = new Date(dateStr).getTime()
  const diffMs = now - then
  if (diffMs < 0) return 'just now'

  const seconds = Math.floor(diffMs / 1000)
  if (seconds < 60) return `${seconds}s ago`

  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`

  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`

  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

export function AlertsList({ range }: AlertsListProps) {
  const { t } = useTranslation()
  const [page, setPage] = useState(0)

  useEffect(() => {
    setPage(0)
  }, [range])

  const { data, isLoading, isError } = useAlerts({
    range,
    limit: PAGE_SIZE,
    offset: page * PAGE_SIZE,
  })

  const totalPages = useMemo(() => {
    if (!data?.total) return 0
    return Math.ceil(data.total / PAGE_SIZE)
  }, [data?.total])

  const handlePreviousPage = () => {
    setPage((p) => Math.max(0, p - 1))
  }

  const handleNextPage = () => {
    setPage((p) => (totalPages > 0 ? Math.min(totalPages - 1, p + 1) : 0))
  }

  if (isLoading) {
    return (
      <div className="rounded-lg border border-gray-700 bg-gray-900 p-4" data-testid="alerts-list">
        <Skeleton className="h-4 w-32 mb-4" />
        <Skeleton className="h-48 w-full" />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="rounded-lg border border-red-700/50 bg-red-900/20 p-4" data-testid="alerts-list">
        <p className="text-sm text-red-300">
          {t('security.crowdsec.dashboard.alertsError', 'Failed to load alerts.')}
        </p>
      </div>
    )
  }

  const alerts = data?.alerts ?? []

  if (!alerts.length && page === 0) {
    return (
      <div
        className="rounded-lg border border-gray-700 bg-gray-900 p-4 text-center text-gray-400 py-12"
        data-testid="alerts-list"
      >
        <AlertTriangle className="mx-auto h-8 w-8 mb-2 text-gray-500" aria-hidden="true" />
        <p>{t('security.crowdsec.dashboard.noAlerts', 'No alerts for the selected period.')}</p>
      </div>
    )
  }

  return (
    <div className="rounded-lg border border-gray-700 bg-gray-900 p-4" data-testid="alerts-list">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-medium text-gray-300">
          {t('security.crowdsec.dashboard.recentAlerts', 'Recent Alerts')}
        </h3>
        <span className="text-xs text-gray-500" aria-live="polite">
          {t('security.crowdsec.dashboard.alertsCount', '{{count}} total', { count: data?.total ?? 0 })}
        </span>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-sm text-left">
          <thead>
            <tr className="border-b border-gray-700">
              <th scope="col" className="py-2 px-3 text-gray-400 font-medium">
                {t('security.crowdsec.dashboard.colIP', 'IP')}
              </th>
              <th scope="col" className="py-2 px-3 text-gray-400 font-medium">
                {t('security.crowdsec.dashboard.colScenario', 'Scenario')}
              </th>
              <th scope="col" className="py-2 px-3 text-gray-400 font-medium">
                {t('security.crowdsec.dashboard.colTime', 'Time')}
              </th>
              <th scope="col" className="py-2 px-3 text-gray-400 font-medium">
                {t('security.crowdsec.dashboard.colEvents', 'Events')}
              </th>
            </tr>
          </thead>
          <tbody>
            {alerts.map((alert, i) => (
              <tr
                key={`${alert.id}-${i}`}
                className="border-b border-gray-800 hover:bg-gray-800/50"
              >
                <td className="py-2 px-3 font-mono text-gray-300 whitespace-nowrap">
                  {alert.ip}
                </td>
                <td className="py-2 px-3 text-gray-300 truncate max-w-[200px]" title={alert.scenario}>
                  {alert.scenario.split('/').pop()}
                </td>
                <td className="py-2 px-3 text-gray-400 whitespace-nowrap tabular-nums">
                  <time dateTime={alert.created_at} title={new Date(alert.created_at).toLocaleString()}>
                    {formatRelativeTime(alert.created_at)}
                  </time>
                </td>
                <td className="py-2 px-3 text-gray-400 tabular-nums">
                  {alert.events_count}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <nav
          className="flex items-center justify-between mt-4 pt-3 border-t border-gray-800"
          aria-label={t('security.crowdsec.dashboard.alertsPagination', 'Alerts pagination')}
        >
          <span className="text-xs text-gray-500">
            {t('security.crowdsec.dashboard.pageInfo', 'Page {{current}} of {{total}}', {
              current: page + 1,
              total: totalPages,
            })}
          </span>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={handlePreviousPage}
              disabled={page === 0}
              className="inline-flex items-center gap-1 rounded-md bg-gray-800 px-2.5 py-1.5 text-xs text-gray-300 hover:bg-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 focus-visible:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed"
              aria-label={t('security.crowdsec.dashboard.previousPage', 'Previous page')}
            >
              <ChevronLeft className="h-3 w-3" aria-hidden="true" />
              {t('security.crowdsec.dashboard.previous', 'Previous')}
            </button>
            <button
              type="button"
              onClick={handleNextPage}
              disabled={page >= totalPages - 1}
              className="inline-flex items-center gap-1 rounded-md bg-gray-800 px-2.5 py-1.5 text-xs text-gray-300 hover:bg-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 focus-visible:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed"
              aria-label={t('security.crowdsec.dashboard.nextPage', 'Next page')}
            >
              {t('security.crowdsec.dashboard.next', 'Next')}
              <ChevronRight className="h-3 w-3" aria-hidden="true" />
            </button>
          </div>
        </nav>
      )}
    </div>
  )
}
