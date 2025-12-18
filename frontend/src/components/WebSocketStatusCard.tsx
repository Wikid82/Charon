import { useState } from 'react';
import { Wifi, WifiOff, Activity, Clock, Filter, Globe } from 'lucide-react';
import { useWebSocketConnections, useWebSocketStats } from '../hooks/useWebSocketStatus';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  Badge,
  Skeleton,
  Alert,
} from './ui';
import { formatDistanceToNow } from 'date-fns';

interface WebSocketStatusCardProps {
  className?: string;
  showDetails?: boolean;
}

/**
 * Component to display WebSocket connection status and statistics
 */
export function WebSocketStatusCard({ className = '', showDetails = false }: WebSocketStatusCardProps) {
  const [expanded, setExpanded] = useState(showDetails);
  const { data: connections, isLoading: connectionsLoading } = useWebSocketConnections();
  const { data: stats, isLoading: statsLoading } = useWebSocketStats();

  const isLoading = connectionsLoading || statsLoading;

  if (isLoading) {
    return (
      <Card className={className}>
        <CardHeader>
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-4 w-64" />
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-3/4" />
          </div>
        </CardContent>
      </Card>
    );
  }

  if (!stats) {
    return (
      <Alert variant="warning" className={className}>
        Unable to load WebSocket status
      </Alert>
    );
  }

  const hasActiveConnections = stats.total_active > 0;

  return (
    <Card className={className}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className={`p-2 rounded-lg ${hasActiveConnections ? 'bg-success/10' : 'bg-surface-muted'}`}>
              {hasActiveConnections ? (
                <Wifi className="w-5 h-5 text-success" />
              ) : (
                <WifiOff className="w-5 h-5 text-content-muted" />
              )}
            </div>
            <div>
              <CardTitle className="text-lg">WebSocket Connections</CardTitle>
              <CardDescription>
                Real-time connection monitoring
              </CardDescription>
            </div>
          </div>
          <Badge variant={hasActiveConnections ? 'success' : 'default'}>
            {stats.total_active} Active
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Statistics Grid */}
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-sm text-content-muted">
              <Activity className="w-4 h-4" />
              <span>General Logs</span>
            </div>
            <p className="text-2xl font-semibold">{stats.logs_connections}</p>
          </div>
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-sm text-content-muted">
              <Activity className="w-4 h-4" />
              <span>Security Logs</span>
            </div>
            <p className="text-2xl font-semibold">{stats.cerberus_connections}</p>
          </div>
        </div>

        {/* Oldest Connection */}
        {stats.oldest_connection && (
          <div className="pt-3 border-t border-border">
            <div className="flex items-center gap-2 text-sm text-content-muted mb-1">
              <Clock className="w-4 h-4" />
              <span>Oldest Connection</span>
            </div>
            <p className="text-sm">
              {formatDistanceToNow(new Date(stats.oldest_connection), { addSuffix: true })}
            </p>
          </div>
        )}

        {/* Connection Details */}
        {expanded && connections?.connections && connections.connections.length > 0 && (
          <div className="pt-3 border-t border-border space-y-3">
            <p className="text-sm font-medium">Active Connections</p>
            <div className="space-y-2 max-h-64 overflow-y-auto">
              {connections.connections.map((conn) => (
                <div
                  key={conn.id}
                  className="p-3 rounded-lg bg-surface-muted space-y-2 text-xs"
                >
                  <div className="flex items-center justify-between">
                    <Badge variant={conn.type === 'logs' ? 'default' : 'success'} size="sm">
                      {conn.type === 'logs' ? 'General' : 'Security'}
                    </Badge>
                    <span className="text-content-muted font-mono">
                      {conn.id.substring(0, 8)}...
                    </span>
                  </div>
                  {conn.remote_addr && (
                    <div className="flex items-center gap-2 text-content-muted">
                      <Globe className="w-3 h-3" />
                      <span>{conn.remote_addr}</span>
                    </div>
                  )}
                  {conn.filters && (
                    <div className="flex items-center gap-2 text-content-muted">
                      <Filter className="w-3 h-3" />
                      <span className="truncate">{conn.filters}</span>
                    </div>
                  )}
                  <div className="flex items-center gap-2 text-content-muted">
                    <Clock className="w-3 h-3" />
                    <span>
                      Connected {formatDistanceToNow(new Date(conn.connected_at), { addSuffix: true })}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Toggle Details Button */}
        {connections?.connections && connections.connections.length > 0 && (
          <button
            onClick={() => setExpanded(!expanded)}
            className="w-full pt-3 text-sm text-primary hover:text-primary/80 transition-colors"
          >
            {expanded ? 'Hide Details' : 'Show Details'}
          </button>
        )}

        {/* No Connections Message */}
        {!hasActiveConnections && (
          <div className="pt-3 text-center text-sm text-content-muted">
            No active WebSocket connections
          </div>
        )}
      </CardContent>
    </Card>
  );
}
