import { AlertTriangle, ChevronLeft, ChevronRight } from 'lucide-react'
import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { useAlerts } from '../../hooks/useCrowdsecDashboard'
import { formatAbsoluteTime, formatRelativeTime } from '../../utils/formatRelativeTime'
import { Skeleton } from '../ui'
import { ListErrorNote } from './ListErrorNote'

import type { TimeRange } from '../../api/crowdsecDashboard'

interface AlertsListProps {
  range: TimeRange
}

const PAGE_SIZE = 10

export function AlertsList({ range }: AlertsListProps) {
  const { t } = useTranslation()
  // The page is tied to the range it was chosen under; a range change reads as page 0
  // without an effect that resets state after the stale page has already rendered.
  const [pagination, setPagination] = useState<{ range: TimeRange; page: number }>({ range, page: 0 })
  const page = pagination.range === range ? pagination.page : 0

  const { data, isLoading, isError } = useAlerts({
    range,
    limit: PAGE_SIZE,
    offset: page * PAGE_SIZE,
  })

  const totalPages = useMemo(() => {
    return !data?.total ? 0 : Math.ceil(data.total / PAGE_SIZE)
  }, [data?.total])

  const handlePreviousPage = () => {
    setPagination({ range, page: Math.max(0, page - 1) })
  }

  const handleNextPage = () => {
    setPagination({ range, page: totalPages > 0 ? Math.min(totalPages - 1, page + 1) : 0 })
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
        <div className="mb-4 text-left"><ListErrorNote error={data?.error} error_code={data?.error_code} /></div>
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

      <div className="mb-3"><ListErrorNote error={data?.error} error_code={data?.error_code} /></div>

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
                  <time dateTime={alert.created_at} title={formatAbsoluteTime(alert.created_at)}>
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
