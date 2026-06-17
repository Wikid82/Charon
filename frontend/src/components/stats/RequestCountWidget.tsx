import { Activity, Info } from 'lucide-react'

import { Card, CardContent, CardHeader, CardTitle, Skeleton } from '../ui'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../ui'

import type { StatsSummary } from '../../api/stats'

export interface RequestCountWidgetProps {
  summary: StatsSummary | undefined
  isLoading: boolean
}

interface StatItemProps {
  label: string
  value: number | undefined
  isLoading: boolean
}

function StatItem({ label, value, isLoading }: StatItemProps) {
  return (
    <div className="flex flex-col gap-1">
      <p className="text-sm font-medium text-content-secondary">{label}</p>
      {isLoading ? (
        <Skeleton className="h-8 w-24" />
      ) : (
        <p className="text-3xl font-bold text-content-primary tabular-nums">
          {value?.toLocaleString() ?? '—'}
        </p>
      )}
    </div>
  )
}

export function RequestCountWidget({ summary, isLoading }: RequestCountWidgetProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center gap-2">
          <div className="rounded-lg bg-brand-500/10 p-2 text-brand-500">
            <Activity className="h-5 w-5" />
          </div>
          <div className="flex items-center gap-1.5">
            <CardTitle>Request Counts</CardTitle>
            <TooltipProvider>
              <Tooltip>
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
                  This counts how many times people visited your sites — like counting how many
                  customers walked through your door today, this week, and this month.
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-3 gap-6">
          <StatItem
            label="Last 24h"
            value={summary?.requests_last_24h}
            isLoading={isLoading}
          />
          <StatItem
            label="Last 7 days"
            value={summary?.requests_last_7d}
            isLoading={isLoading}
          />
          <StatItem
            label="Last 30 days"
            value={summary?.requests_last_30d}
            isLoading={isLoading}
          />
        </div>
      </CardContent>
    </Card>
  )
}
