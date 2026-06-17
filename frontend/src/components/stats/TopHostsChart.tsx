import { Info } from 'lucide-react'
import {
  BarChart,
  Bar,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  type TooltipContentProps,
} from 'recharts'

import { Card, CardContent, CardHeader, CardTitle, Skeleton } from '../ui'
import { Tooltip as UITooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../ui'

import type { HostStat } from '../../api/stats'
import type { ValueType, NameType } from 'recharts/types/component/DefaultTooltipContent'

export interface TopHostsChartProps {
  data: HostStat[] | undefined
  isLoading: boolean
}

const MAX_LABEL_LEN = 24

const HOST_COLORS = [
  '#6366f1',
  '#22c55e',
  '#f59e0b',
  '#ef4444',
  '#3b82f6',
  '#a855f7',
  '#14b8a6',
  '#f97316',
]

function truncate(hostname: string): string {
  return hostname.length > MAX_LABEL_LEN
    ? `${hostname.slice(0, MAX_LABEL_LEN - 1)}…`
    : hostname
}

export function TopHostsChart({ data, isLoading }: TopHostsChartProps) {
  const chartData = (data ?? []).map((h, i) => ({
    ...h,
    label: truncate(h.hostname),
    color: HOST_COLORS[i % HOST_COLORS.length],
  }))

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center gap-1.5">
          <CardTitle>Top Hosts</CardTitle>
          <TooltipProvider>
            <UITooltip>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  className="text-content-muted hover:text-content-secondary transition-colors"
                  aria-label="About this widget"
                >
                  <Info className="h-3.5 w-3.5" />
                </button>
              </TooltipTrigger>
              <TooltipContent side="top" className="max-w-xs text-sm">
                This shows which of your sites get the most visitors. Think of it like a popularity
                contest — the longer the bar, the busier that site is.
              </TooltipContent>
            </UITooltip>
          </TooltipProvider>
        </div>
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
          <>
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
                    const hostname =
                      (payload[0]?.payload as { hostname?: string })?.hostname ?? String(label)
                    const count = payload[0]?.value
                    return (
                      <div className="rounded-lg border border-border bg-surface-elevated px-3 py-2 text-sm shadow">
                        <p className="font-medium text-content-primary">{hostname}</p>
                        <p className="text-content-secondary">
                          {typeof count === 'number'
                            ? count.toLocaleString()
                            : String(count ?? '')}{' '}
                          Requests
                        </p>
                      </div>
                    )
                  }}
                />
                <Bar dataKey="count" radius={[0, 4, 4, 0]}>
                  {chartData.map((entry, index) => (
                    <Cell key={entry.host_id} fill={HOST_COLORS[index % HOST_COLORS.length]} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>

            <ul
              aria-label="Host color legend"
              className="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1"
            >
              {chartData.map((entry) => (
                <li key={entry.host_id} className="flex items-center gap-1.5 text-xs">
                  <span
                    aria-hidden="true"
                    className="inline-block h-2.5 w-2.5 rounded-full shrink-0"
                    style={{ backgroundColor: entry.color }}
                  />
                  <span className="truncate text-content-secondary" title={entry.hostname}>
                    {entry.hostname}
                  </span>
                </li>
              ))}
            </ul>
          </>
        )}
      </CardContent>
    </Card>
  )
}
