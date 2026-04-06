import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react'
import { useState, useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '../ui'

import type { TopIP } from '../../api/crowdsecDashboard'

// TODO: Replace TopIP[] with a dedicated ActiveDecision[] type backed by /v1/decisions once endpoints are available
interface ActiveDecisionsTableProps {
  data: TopIP[] | undefined
  isLoading: boolean
  isError: boolean
}

type SortKey = 'ip' | 'count' | 'last_seen' | 'country'
type SortDir = 'asc' | 'desc'

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

export function ActiveDecisionsTable({ data, isLoading, isError }: ActiveDecisionsTableProps) {
  const { t } = useTranslation()
  const [sortKey, setSortKey] = useState<SortKey>('count')
  const [sortDir, setSortDir] = useState<SortDir>('desc')

  const toggleSort = useCallback((key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir('desc')
    }
  }, [sortKey])

  const sorted = useMemo(() => {
    if (!data) return []
    return [...data].sort((a, b) => {
      const aVal = a[sortKey]
      const bVal = b[sortKey]
      let cmp: number
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        cmp = aVal - bVal
      } else {
        cmp = String(aVal ?? '').localeCompare(String(bVal ?? ''), undefined, { numeric: true })
      }
      return sortDir === 'asc' ? cmp : -cmp
    })
  }, [data, sortKey, sortDir])

  if (isLoading) {
    return (
      <div className="rounded-lg border border-gray-700 bg-gray-900 p-4">
        <Skeleton className="h-4 w-40 mb-4" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="rounded-lg border border-red-700/50 bg-red-900/20 p-4">
        <p className="text-sm text-red-300">{t('security.crowdsec.dashboard.decisionsError', 'Failed to load decisions.')}</p>
      </div>
    )
  }

  if (!sorted.length) {
    return (
      <div className="rounded-lg border border-gray-700 bg-gray-900 p-4 text-center text-gray-400 py-12">
        <p>{t('security.crowdsec.dashboard.noDecisions', 'No active decisions.')}</p>
      </div>
    )
  }

  const columns: { key: SortKey; label: string }[] = [
    { key: 'ip', label: t('security.crowdsec.dashboard.colIP', 'IP') },
    { key: 'count', label: t('security.crowdsec.dashboard.colCount', 'Alerts') },
    { key: 'last_seen', label: t('security.crowdsec.dashboard.colLastSeen', 'Last Seen') },
    { key: 'country', label: t('security.crowdsec.dashboard.colCountry', 'Country') },
  ]

  return (
    <div className="rounded-lg border border-gray-700 bg-gray-900 p-4">
      <h3 className="text-sm font-medium text-gray-300 mb-4">
        {t('security.crowdsec.dashboard.activeDecisionsTable', 'Active Decisions')}
      </h3>
      <div className="overflow-x-auto">
        <table className="w-full text-sm text-left">
          <thead>
            <tr className="border-b border-gray-700">
              {columns.map((col) => {
                const isSorted = sortKey === col.key
                const ariaSortValue = isSorted ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'
                return (
                  <th
                    key={col.key}
                    scope="col"
                    aria-sort={ariaSortValue}
                    className="py-2 px-3 text-gray-400 font-medium whitespace-nowrap"
                  >
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 hover:text-gray-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 focus-visible:ring-offset-gray-900 rounded"
                      onClick={() => toggleSort(col.key)}
                      aria-label={`${t('security.crowdsec.dashboard.sortBy', 'Sort by')} ${col.label}`}
                    >
                      {col.label}
                      {isSorted
                        ? sortDir === 'asc'
                          ? <ArrowUp className="h-3 w-3" aria-hidden="true" />
                          : <ArrowDown className="h-3 w-3" aria-hidden="true" />
                        : <ArrowUpDown className="h-3 w-3 text-gray-600" aria-hidden="true" />
                      }
                    </button>
                  </th>
                )
              })}
            </tr>
          </thead>
          <tbody>
            {sorted.map((row, i) => (
              <tr key={`${row.ip}-${i}`} className="border-b border-gray-800 hover:bg-gray-800/50">
                <td className="py-2 px-3 font-mono text-gray-300 whitespace-nowrap">{row.ip}</td>
                <td className="py-2 px-3 text-gray-400 tabular-nums">{row.count}</td>
                <td className="py-2 px-3 text-gray-400 whitespace-nowrap tabular-nums">
                  <time dateTime={row.last_seen} title={new Date(row.last_seen).toLocaleString()}>
                    {formatRelativeTime(row.last_seen)}
                  </time>
                </td>
                <td className="py-2 px-3 text-gray-400">{row.country || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
