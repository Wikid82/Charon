import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  type TooltipContentProps,
} from 'recharts'


import { Card, CardContent, CardHeader, CardTitle, Skeleton } from '../ui'

import type { TrafficBucket, StatsBucket } from '../../api/stats'
import type { ValueType, NameType } from 'recharts/types/component/DefaultTooltipContent'

export interface TrafficVolumeChartProps {
  data: TrafficBucket[] | undefined
  isLoading: boolean
  bucket: StatsBucket
}

function formatBytes(bytes: number): string {
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`
  if (bytes >= 1_024) return `${(bytes / 1_024).toFixed(1)} KB`
  return `${bytes} B`
}

function formatTimestamp(iso: string, bucket: StatsBucket): string {
  const date = new Date(iso)
  if (bucket === '1d') {
    return new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric' }).format(date)
  }
  return new Intl.DateTimeFormat('en-US', { hour: '2-digit', minute: '2-digit', hour12: false }).format(date)
}

export function TrafficVolumeChart({ data, isLoading, bucket }: TrafficVolumeChartProps) {
  const chartData = (data ?? []).map((d) => ({
    ...d,
    label: formatTimestamp(d.bucket, bucket),
  }))

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle>Traffic Volume</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-3">
            <Skeleton className="h-48 w-full" />
          </div>
        ) : chartData.length === 0 ? (
          <p className="text-sm text-content-muted py-8 text-center">No data available</p>
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <LineChart
              data={chartData}
              margin={{ top: 4, right: 24, left: 8, bottom: 4 }}
            >
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis
                dataKey="label"
                tick={{ fontSize: 12 }}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                tickFormatter={formatBytes}
                tick={{ fontSize: 12 }}
                tickLine={false}
                axisLine={false}
                width={72}
              />
              <Tooltip
                content={(props: TooltipContentProps<ValueType, NameType>) => {
                  const { active, payload, label } = props
                  if (!active || !payload?.length) return null
                  const value = payload[0]?.value
                  const bytes = typeof value === 'number' ? value : 0
                  return (
                    <div className="rounded-lg border border-border bg-surface-elevated px-3 py-2 text-sm shadow">
                      <p className="font-medium text-content-primary">Time: {String(label ?? '')}</p>
                      <p className="text-content-secondary">{formatBytes(bytes)} sent</p>
                    </div>
                  )
                }}
              />
              <Line
                type="monotone"
                dataKey="bytes_sent"
                stroke="var(--color-brand-500, #6366f1)"
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 4 }}
              />
            </LineChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  )
}
