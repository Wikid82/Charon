import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

import { Skeleton } from '../ui'

import type { TopIP } from '../../api/crowdsecDashboard'

interface TopAttackingIPsChartProps {
  data: TopIP[] | undefined
  isLoading: boolean
  isError: boolean
}

const BAR_COLOR = '#6366f1'

export function TopAttackingIPsChart({ data, isLoading, isError }: TopAttackingIPsChartProps) {
  const { t } = useTranslation()

  const chartData = useMemo(() => {
    if (!data) return []
    return data.slice(0, 10).map((ip) => ({
      ip: ip.ip,
      decisions: ip.count,
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
        <p className="text-sm text-red-300">{t('security.crowdsec.dashboard.topIPsError', 'Failed to load top IPs data.')}</p>
      </div>
    )
  }

  if (!chartData.length) {
    return (
      <div className="rounded-lg border border-gray-700 bg-gray-900 p-4 text-center text-gray-400 py-12">
        <p>{t('security.crowdsec.dashboard.noTopIPs', 'No attacking IPs in the selected period.')}</p>
      </div>
    )
  }

  return (
    <div className="rounded-lg border border-gray-700 bg-gray-900 p-4">
      <h3 className="text-sm font-medium text-gray-300 mb-4">
        {t('security.crowdsec.dashboard.topAttackingIPs', 'Top Attacking IPs')}
      </h3>
      <div
        role="img"
        aria-label={t('security.crowdsec.dashboard.topIPsChartLabel', 'Horizontal bar chart showing top attacking IPs by decision count')}
      >
        <ResponsiveContainer width="100%" height={280}>
          <BarChart data={chartData} layout="vertical" margin={{ top: 5, right: 20, left: 10, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#374151" horizontal={false} />
            <XAxis type="number" tick={{ fill: '#9ca3af', fontSize: 12 }} stroke="#4b5563" allowDecimals={false} />
            <YAxis
              dataKey="ip"
              type="category"
              tick={{ fill: '#9ca3af', fontSize: 11 }}
              stroke="#4b5563"
              width={120}
            />
            <Tooltip
              contentStyle={{ backgroundColor: '#1f2937', border: '1px solid #374151', borderRadius: 8 }}
              labelStyle={{ color: '#d1d5db' }}
            />
            <Bar
              dataKey="decisions"
              name={t('security.crowdsec.dashboard.decisions', 'Decisions')}
              fill={BAR_COLOR}
              radius={[0, 4, 4, 0]}
            />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
