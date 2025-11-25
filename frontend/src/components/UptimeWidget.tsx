import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Activity, CheckCircle2, XCircle, AlertCircle } from 'lucide-react'
import { getMonitors } from '../api/uptime'

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

  return (
    <Link
      to="/uptime"
      className="bg-dark-card p-6 rounded-lg border border-gray-800 hover:border-gray-700 transition-colors block"
    >
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Activity className="w-4 h-4 text-gray-400" />
          <span className="text-sm text-gray-400">Uptime Status</span>
        </div>
        {hasDown && (
          <span className="px-2 py-0.5 text-xs font-medium bg-red-900/30 text-red-400 rounded-full animate-pulse">
            Issues
          </span>
        )}
      </div>

      {isLoading ? (
        <div className="text-gray-500 text-sm">Loading...</div>
      ) : totalCount === 0 ? (
        <div className="text-gray-500 text-sm">No monitors configured</div>
      ) : (
        <>
          {/* Status indicator */}
          <div className="flex items-center gap-2 mb-3">
            {allUp ? (
              <>
                <CheckCircle2 className="w-6 h-6 text-green-400" />
                <span className="text-lg font-bold text-green-400">All Systems Operational</span>
              </>
            ) : hasDown ? (
              <>
                <XCircle className="w-6 h-6 text-red-400" />
                <span className="text-lg font-bold text-red-400">
                  {downCount} {downCount === 1 ? 'Site' : 'Sites'} Down
                </span>
              </>
            ) : (
              <>
                <AlertCircle className="w-6 h-6 text-yellow-400" />
                <span className="text-lg font-bold text-yellow-400">Unknown Status</span>
              </>
            )}
          </div>

          {/* Quick stats */}
          <div className="flex gap-4 text-xs">
            <div className="flex items-center gap-1">
              <span className="w-2 h-2 rounded-full bg-green-400"></span>
              <span className="text-gray-400">{upCount} up</span>
            </div>
            {downCount > 0 && (
              <div className="flex items-center gap-1">
                <span className="w-2 h-2 rounded-full bg-red-400"></span>
                <span className="text-gray-400">{downCount} down</span>
              </div>
            )}
            <div className="text-gray-500">
              {totalCount} total
            </div>
          </div>

          {/* Mini status bars */}
          {monitors && monitors.length > 0 && (
            <div className="flex gap-1 mt-3">
              {monitors.slice(0, 20).map((monitor) => (
                <div
                  key={monitor.id}
                  className={`flex-1 h-2 rounded-sm ${
                    monitor.status === 'up' ? 'bg-green-500' : 'bg-red-500'
                  }`}
                  title={`${monitor.name}: ${monitor.status.toUpperCase()}`}
                />
              ))}
              {monitors.length > 20 && (
                <div className="text-xs text-gray-500 ml-1">+{monitors.length - 20}</div>
              )}
            </div>
          )}
        </>
      )}

      <div className="text-xs text-gray-500 mt-3">Click for detailed view →</div>
    </Link>
  )
}
