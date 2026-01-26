import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Activity, CheckCircle2, XCircle, AlertCircle, ArrowRight } from 'lucide-react'
import { getMonitors } from '../api/uptime'
import { Card, CardHeader, CardContent, Badge, Skeleton } from './ui'

export default function UptimeWidget() {
  const { data: monitors, isLoading } = useQuery({
    queryKey: ['monitors'],
    queryFn: getMonitors,
    refetchInterval: 30000,
  })

  const upCount = monitors?.filter(m => m.status === 'up').length || 0
  const downCount = monitors?.filter(m => m.status === 'down').length || 0
  const totalCount = monitors?.length || 0

  const allUp = totalCount > 0 && downCount === 0
  const hasDown = downCount > 0

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="pb-2">
          <div className="flex items-center gap-2">
            <Skeleton className="h-5 w-5 rounded" />
            <Skeleton className="h-4 w-24" />
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-6 w-48" />
          <div className="flex gap-4">
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-4 w-20" />
          </div>
          <div className="flex gap-1">
            {Array.from({ length: 10 }).map((_, i) => (
              <Skeleton key={i} className="h-3 flex-1 rounded-sm" />
            ))}
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Link to="/uptime" className="block group">
      <Card variant="interactive" className="h-full">
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="rounded-lg bg-brand-500/10 p-2 text-brand-500">
                <Activity className="h-5 w-5" />
              </div>
              <span className="text-sm font-medium text-content-secondary">Uptime Status</span>
            </div>
            {hasDown && (
              <Badge variant="error" size="sm" className="animate-pulse">
                Issues
              </Badge>
            )}
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {totalCount === 0 ? (
            <p className="text-content-muted text-sm">No monitors configured</p>
          ) : (
            <>
              {/* Status indicator */}
              <div className="flex items-center gap-2">
                {allUp ? (
                  <>
                    <CheckCircle2 className="h-6 w-6 text-success" />
                    <span className="text-lg font-bold text-success">All Systems Operational</span>
                  </>
                ) : hasDown ? (
                  <>
                    <XCircle className="h-6 w-6 text-error" />
                    <span className="text-lg font-bold text-error">
                      {downCount} {downCount === 1 ? 'Site' : 'Sites'} Down
                    </span>
                  </>
                ) : (
                  <>
                    <AlertCircle className="h-6 w-6 text-warning" />
                    <span className="text-lg font-bold text-warning">Unknown Status</span>
                  </>
                )}
              </div>

              {/* Quick stats */}
              <div className="flex gap-4 text-sm">
                <div className="flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full bg-success"></span>
                  <span className="text-content-secondary">{upCount} up</span>
                </div>
                {downCount > 0 && (
                  <div className="flex items-center gap-1.5">
                    <span className="w-2 h-2 rounded-full bg-error"></span>
                    <span className="text-content-secondary">{downCount} down</span>
                  </div>
                )}
                <div className="text-content-muted">
                  {totalCount} total
                </div>
              </div>

              {/* Mini status bars */}
              {monitors && monitors.length > 0 && (
                <div className="flex gap-1">
                  {monitors.slice(0, 20).map((monitor) => (
                    <div
                      key={monitor.id}
                      className={`flex-1 h-2.5 rounded-sm transition-colors duration-fast ${
                        monitor.status === 'up' ? 'bg-success' : 'bg-error'
                      }`}
                      title={`${monitor.name}: ${monitor.status.toUpperCase()}`}
                    />
                  ))}
                  {monitors.length > 20 && (
                    <div className="text-xs text-content-muted ml-1">+{monitors.length - 20}</div>
                  )}
                </div>
              )}
            </>
          )}

          <div className="flex items-center gap-1 text-xs text-content-muted group-hover:text-brand-400 transition-colors duration-fast">
            <span>View detailed status</span>
            <ArrowRight className="h-3 w-3" />
          </div>
        </CardContent>
      </Card>
    </Link>
  )
}
