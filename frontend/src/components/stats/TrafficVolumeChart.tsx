import { Info } from 'lucide-react'
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
import {
  Tooltip as UITooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '../ui'

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
  return new Intl.DateTimeFormat('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

export function TrafficVolumeChart({ data, isLoading, bucket }: TrafficVolumeChartProps) {
  const chartData = (data ?? []).map((d) => ({
    ...d,
    label: formatTimestamp(d.bucket, bucket),
  }))

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center gap-1.5">
          <CardTitle>Traffic Volume</CardTitle>
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
              <TooltipContent side="top" className="max-w-xs text-sm space-y-2">
                <p>
                  This shows how much data your sites sent over time — like checking how much water
                  flowed through a pipe. A big spike means lots of visitors or large files being
                  downloaded.
                </p>
                {chartData.length === 0 && (
                  <p className="text-xs border-t border-border/40 pt-2">
                    Data is being collected. It may take several hours to populate depending on traffic volume.
                  </p>
                )}
              </TooltipContent>
            </UITooltip>
          </TooltipProvider>
        </div>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-3">
            <Skeleton className="h-48 w-full" />
          </div>
        ) : chartData.length === 0 ? (
          <div className="space-y-3 py-8 text-center">
            <p className="text-sm text-content-muted">No data available yet</p>
            <p className="text-xs text-content-muted">
              Data is being collected. Check back in a few hours as traffic flows through your proxy.
            </p>
          </div>
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
