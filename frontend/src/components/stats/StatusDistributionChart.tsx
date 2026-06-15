import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, type TooltipContentProps } from 'recharts'


import { Card, CardContent, CardHeader, CardTitle, Skeleton } from '../ui'

import type { StatusStat } from '../../api/stats'
import type { ValueType, NameType } from 'recharts/types/component/DefaultTooltipContent'

export interface StatusDistributionChartProps {
  data: StatusStat[] | undefined
  isLoading: boolean
}

function statusClass(code: number): string {
  if (code >= 200 && code < 300) return '2xx'
  if (code >= 300 && code < 400) return '3xx'
  if (code >= 400 && code < 500) return '4xx'
  if (code >= 500) return '5xx'
  return 'other'
}

const STATUS_COLORS: Record<string, string> = {
  '2xx': '#22c55e', // green
  '3xx': '#3b82f6', // blue
  '4xx': '#f59e0b', // amber
  '5xx': '#ef4444', // red
  other: '#94a3b8', // slate
}

interface AggregatedStatus {
  name: string
  count: number
  color: string
}

function aggregateByClass(data: StatusStat[]): AggregatedStatus[] {
  const map = new Map<string, number>()
  for (const s of data) {
    const cls = statusClass(s.code)
    map.set(cls, (map.get(cls) ?? 0) + s.count)
  }
  return Array.from(map.entries()).map(([name, count]) => ({
    name,
    count,
    color: STATUS_COLORS[name] ?? STATUS_COLORS.other,
  }))
}

export function StatusDistributionChart({ data, isLoading }: StatusDistributionChartProps) {
  const chartData = aggregateByClass(data ?? [])
  const total = chartData.reduce((sum, d) => sum + d.count, 0)

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle>Status Distribution</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="flex items-center justify-center py-8">
            <Skeleton variant="circular" className="h-40 w-40" />
          </div>
        ) : chartData.length === 0 ? (
          <p className="text-sm text-content-muted py-8 text-center">No data available</p>
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <PieChart>
              <Pie
                data={chartData}
                dataKey="count"
                nameKey="name"
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={100}
                paddingAngle={2}
                label={({ name, percent }) =>
                  `${name} ${((percent ?? 0) * 100).toFixed(0)}%`
                }
                labelLine={false}
              >
                {chartData.map((entry) => (
                  <Cell key={entry.name} fill={entry.color} />
                ))}
              </Pie>
              <Tooltip
                content={(props: TooltipContentProps<ValueType, NameType>) => {
                  const { active, payload } = props
                  if (!active || !payload?.length) return null
                  const name = payload[0]?.name
                  const value = payload[0]?.value
                  const count = typeof value === 'number' ? value : 0
                  return (
                    <div className="rounded-lg border border-border bg-surface-elevated px-3 py-2 text-sm shadow">
                      <p className="font-medium text-content-primary">{String(name ?? '')}</p>
                      <p className="text-content-secondary">
                        {count.toLocaleString()} ({total > 0 ? ((count / total) * 100).toFixed(1) : 0}%) Requests
                      </p>
                    </div>
                  )
                }}
              />
            </PieChart>
          </ResponsiveContainer>
        )}
        {/* Accessible status summary — renders as HTML so screen readers and tests can read it */}
        {!isLoading && chartData.length > 0 && (
          <ul
            aria-label="Status class breakdown"
            className="mt-4 flex flex-wrap gap-x-4 gap-y-2"
          >
            {chartData.map((d) => (
              <li key={d.name} className="flex items-center gap-1.5 text-sm">
                <span
                  aria-hidden="true"
                  className="inline-block h-3 w-3 rounded-sm shrink-0"
                  style={{ backgroundColor: d.color }}
                />
                <span className="font-medium text-content-primary">{d.name}</span>
                <span className="text-content-muted tabular-nums">{d.count.toLocaleString()}</span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
