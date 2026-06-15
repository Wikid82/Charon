import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  type TooltipContentProps,
} from 'recharts'


import { Card, CardContent, CardHeader, CardTitle, Skeleton } from '../ui'

import type { HostStat } from '../../api/stats'
import type { ValueType, NameType } from 'recharts/types/component/DefaultTooltipContent'

export interface TopHostsChartProps {
  data: HostStat[] | undefined
  isLoading: boolean
}

const MAX_LABEL_LEN = 24

function truncate(hostname: string): string {
  return hostname.length > MAX_LABEL_LEN
    ? `${hostname.slice(0, MAX_LABEL_LEN - 1)}…`
    : hostname
}

export function TopHostsChart({ data, isLoading }: TopHostsChartProps) {
  const chartData = (data ?? []).map((h) => ({
    ...h,
    label: truncate(h.hostname),
  }))

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle>Top Hosts</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-3">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-6 w-full" />
            ))}
          </div>
        ) : chartData.length === 0 ? (
          <p className="text-sm text-content-muted py-8 text-center">No data available</p>
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <BarChart
              data={chartData}
              layout="vertical"
              margin={{ top: 4, right: 24, left: 8, bottom: 4 }}
            >
              <CartesianGrid strokeDasharray="3 3" horizontal={false} />
              <XAxis
                type="number"
                tick={{ fontSize: 12 }}
                tickLine={false}
                axisLine={false}
                allowDecimals={false}
              />
              <YAxis
                type="category"
                dataKey="label"
                width={130}
                tick={{ fontSize: 12 }}
                tickLine={false}
                axisLine={false}
              />
              <Tooltip
                content={(props: TooltipContentProps<ValueType, NameType>) => {
                  const { active, payload, label } = props
                  if (!active || !payload?.length) return null
                  const hostname = (payload[0]?.payload as { hostname?: string })?.hostname ?? String(label)
                  const count = payload[0]?.value
                  return (
                    <div className="rounded-lg border border-border bg-surface-elevated px-3 py-2 text-sm shadow">
                      <p className="font-medium text-content-primary">{hostname}</p>
                      <p className="text-content-secondary">
                        {typeof count === 'number' ? count.toLocaleString() : String(count ?? '')} Requests
                      </p>
                    </div>
                  )
                }}
              />
              <Bar dataKey="count" fill="var(--color-brand-500, #6366f1)" radius={[0, 4, 4, 0]} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  )
}
