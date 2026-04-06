import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts'

import { Skeleton } from '../ui'

import type { ScenarioEntry } from '../../api/crowdsecDashboard'

interface ScenarioBreakdownChartProps {
  data: ScenarioEntry[] | undefined
  isLoading: boolean
  isError: boolean
}

const SCENARIO_COLORS = ['#6366f1', '#10b981', '#f43f5e', '#06b6d4', '#64748b', '#f59e0b', '#8b5cf6', '#ec4899']

export function ScenarioBreakdownChart({ data, isLoading, isError }: ScenarioBreakdownChartProps) {
  const { t } = useTranslation()

  const chartData = useMemo(() => {
    if (!data) return []
    return data.map((s) => ({
      name: s.name.split('/').pop() ?? s.name,
      fullName: s.name,
      count: s.count,
      percentage: s.percentage,
    }))
  }, [data])

  if (isLoading) {
    return (
      <div className="rounded-lg border border-gray-700 bg-gray-900 p-4">
        <Skeleton className="h-4 w-48 mb-4" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="rounded-lg border border-red-700/50 bg-red-900/20 p-4">
        <p className="text-sm text-red-300">{t('security.crowdsec.dashboard.scenariosError', 'Failed to load scenario data.')}</p>
      </div>
    )
  }

  if (!chartData.length) {
    return (
      <div className="rounded-lg border border-gray-700 bg-gray-900 p-4 text-center text-gray-400 py-12">
        <p>{t('security.crowdsec.dashboard.noScenarios', 'No scenario data for the selected period.')}</p>
      </div>
    )
  }

  return (
    <div className="rounded-lg border border-gray-700 bg-gray-900 p-4">
      <h3 className="text-sm font-medium text-gray-300 mb-4">
        {t('security.crowdsec.dashboard.scenarioBreakdown', 'Scenario Breakdown')}
      </h3>
      <div className="flex flex-col lg:flex-row items-center gap-4">
        <div
          role="img"
          aria-label={t('security.crowdsec.dashboard.scenarioChartLabel', 'Donut chart showing distribution of scenarios by decision count')}
          className="flex-shrink-0"
        >
          <ResponsiveContainer width={240} height={240}>
            <PieChart>
              <Pie
                data={chartData}
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={100}
                dataKey="count"
                nameKey="name"
                paddingAngle={2}
              >
                {chartData.map((entry, i) => (
                  <Cell key={entry.fullName} fill={SCENARIO_COLORS[i % SCENARIO_COLORS.length]} />
                ))}
              </Pie>
              <Tooltip
                contentStyle={{ backgroundColor: '#1f2937', border: '1px solid #374151', borderRadius: 8 }}
                labelStyle={{ color: '#d1d5db' }}
                formatter={(value, name) => [`${value} (${chartData.find(d => d.name === String(name))?.percentage.toFixed(1)}%)`, String(name)]}
              />
            </PieChart>
          </ResponsiveContainer>
        </div>
        <ul className="flex-1 space-y-2 text-sm w-full" aria-label={t('security.crowdsec.dashboard.scenarioLegend', 'Scenario legend')}>
          {chartData.map((entry, i) => (
            <li key={entry.fullName} className="flex items-center gap-2">
              <span
                className="inline-block h-3 w-3 rounded-full flex-shrink-0"
                style={{ backgroundColor: SCENARIO_COLORS[i % SCENARIO_COLORS.length] }}
                aria-hidden="true"
              />
              <span className="text-gray-300 truncate" title={entry.fullName}>{entry.name}</span>
              <span className="ml-auto text-gray-500 tabular-nums">{entry.count}</span>
              <span className="text-gray-600 w-12 text-right tabular-nums">{entry.percentage.toFixed(1)}%</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
