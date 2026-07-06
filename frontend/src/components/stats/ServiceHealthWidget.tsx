import { Info, Wifi, WifiOff, AlertTriangle, CheckCircle2 } from 'lucide-react'

import { Card, CardContent, CardHeader, CardTitle, Skeleton, Badge } from '../ui'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../ui'

import type { StatsHealth } from '../../api/stats'

export interface ServiceHealthWidgetProps {
  health: StatsHealth | undefined
  isLoading: boolean
  wsConnected: boolean
}

export function ServiceHealthWidget({ health, isLoading, wsConnected }: ServiceHealthWidgetProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1.5">
            <CardTitle>Stats Service Health</CardTitle>
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
                  This shows whether the real-time stats connection is working. A green &apos;Live&apos;
                  dot means stats are updating instantly. If it says &apos;Offline&apos;, stats still
                  work but refresh every 30 seconds instead.
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          {!isLoading && (
            <Badge variant={wsConnected ? 'success' : 'default'} size="sm">
              {wsConnected ? 'Live' : 'Offline'}
            </Badge>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* WebSocket status */}
        <div className="flex items-center gap-3">
          {wsConnected ? (
            <Wifi className="h-5 w-5 text-success shrink-0" />
          ) : (
            <WifiOff className="h-5 w-5 text-content-muted shrink-0" />
          )}
          <div className="min-w-0">
            <p className="text-sm font-medium text-content-primary">
              Real-time feed
            </p>
            <p className="text-xs text-content-muted">
              {wsConnected ? 'Connected via WebSocket' : 'Polling every 30 s'}
            </p>
          </div>
          <span
            aria-hidden="true"
            className={`ml-auto h-2.5 w-2.5 rounded-full shrink-0 ${
              wsConnected ? 'bg-success animate-pulse' : 'bg-content-muted'
            }`}
          />
        </div>

        {/* Dropped count */}
        <div className="flex items-center gap-3">
          {isLoading ? (
            <>
              <Skeleton variant="circular" className="h-5 w-5 shrink-0" />
              <div className="flex-1 space-y-1">
                <Skeleton className="h-4 w-32" />
                <Skeleton className="h-3 w-24" />
              </div>
            </>
          ) : (
            <>
              {(health?.dropped_count ?? 0) > 0 ? (
                <AlertTriangle className="h-5 w-5 text-warning shrink-0" />
              ) : (
                <CheckCircle2 className="h-5 w-5 text-success shrink-0" />
              )}
              <div className="min-w-0">
                <p className="text-sm font-medium text-content-primary">
                  Dropped events
                </p>
                <p
                  className={`text-xs ${
                    (health?.dropped_count ?? 0) > 0
                      ? 'text-warning'
                      : 'text-content-muted'
                  }`}
                >
                  {health?.dropped_count ?? 0} events dropped
                  {(health?.dropped_count ?? 0) > 0 && ' — ingester may be overloaded'}
                </p>
              </div>
              <span
                aria-label={`${health?.dropped_count ?? 0} dropped`}
                className="ml-auto tabular-nums text-sm font-bold text-content-primary"
              >
                {health?.dropped_count ?? 0}
              </span>
            </>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
