import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

import { Skeleton } from '../ui'

import type { TimelineBucket, TimeRange } from '../../api/crowdsecDashboard'

interface BanTimelineChartProps {
  data: TimelineBucket[] | undefined
  isLoading: boolean
  isError: boolean
  range: TimeRange
}

const BAN_COLOR = '#3b82f6'
const CAPTCHA_COLOR = '#f59e0b'

function formatTick(value: string, range: TimeRange): string {
  const d = new Date(value)
  if (range === '1h' || range === '6h') return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  if (range === '24h') return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

export function BanTimelineChart({ data, isLoading, isError, range }: BanTimelineChartProps) {
  const { t } = useTranslation()

  const chartData = useMemo(() => {
    if (!data) return []
    return data.map((b) => ({
      time: b.timestamp,
      bans: b.bans,
      captchas: b.captchas,
    }))
  }, [data])

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
        <p className="text-sm text-red-300">{t('security.crowdsec.dashboard.timelineError', 'Failed to load timeline data.')}</p>
      </div>
    )
  }

  if (!chartData.length) {
    return (
      <div className="rounded-lg border border-gray-700 bg-gray-900 p-4 text-center text-gray-400 py-12">
        <p>{t('security.crowdsec.dashboard.noTimelineData', 'No decision data for the selected period.')}</p>
      </div>
    )
  }

  return (
    <div className="rounded-lg border border-gray-700 bg-gray-900 p-4">
      <h3 className="text-sm font-medium text-gray-300 mb-4">
        {t('security.crowdsec.dashboard.decisionTimeline', 'Decision Timeline')}
      </h3>
      <div
        role="img"
        aria-label={t('security.crowdsec.dashboard.timelineChartLabel', 'Area chart showing bans and captchas over time')}
      >
        <ResponsiveContainer width="100%" height={280}>
          <AreaChart data={chartData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
            <XAxis
              dataKey="time"
              tickFormatter={(v: string) => formatTick(v, range)}
              tick={{ fill: '#9ca3af', fontSize: 12 }}
              stroke="#4b5563"
            />
            <YAxis tick={{ fill: '#9ca3af', fontSize: 12 }} stroke="#4b5563" allowDecimals={false} />
            <Tooltip
              contentStyle={{ backgroundColor: '#1f2937', border: '1px solid #374151', borderRadius: 8 }}
              labelStyle={{ color: '#d1d5db' }}
              labelFormatter={(v) => new Date(String(v)).toLocaleString()}
            />
            <Area
              type="monotone"
              dataKey="bans"
              name={t('security.crowdsec.dashboard.bans', 'Bans')}
              stroke={BAN_COLOR}
              fill={BAN_COLOR}
              fillOpacity={0.3}
              strokeWidth={2}
            />
            <Area
              type="monotone"
              dataKey="captchas"
              name={t('security.crowdsec.dashboard.captchas', 'Captchas')}
              stroke={CAPTCHA_COLOR}
              fill={CAPTCHA_COLOR}
              fillOpacity={0.2}
              strokeWidth={2}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
