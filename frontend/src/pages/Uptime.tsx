import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { getMonitors, getMonitorHistory } from '../api/uptime';
import { Activity, ArrowUp, ArrowDown } from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';

const MonitorCard: React.FC<{ monitor: any }> = ({ monitor }) => {
  const { data: history } = useQuery({
    queryKey: ['uptimeHistory', monitor.id],
    queryFn: () => getMonitorHistory(monitor.id, 60),
    refetchInterval: 60000,
  });

  const isUp = monitor.status === 'up';

  return (
    <div className={`bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4 border-l-4 ${isUp ? 'border-l-green-500' : 'border-l-red-500'}`}>
      <div className="flex justify-between items-start mb-4">
        <div>
          <h3 className="font-semibold text-lg text-gray-900 dark:text-white">{monitor.name}</h3>
          <div className="text-sm text-gray-500 dark:text-gray-400 flex items-center gap-2">
            <a href={`http://${monitor.url}`} target="_blank" rel="noreferrer" className="hover:underline">
              {monitor.url}
            </a>
            <span className="px-2 py-0.5 rounded-full bg-gray-100 dark:bg-gray-700 text-xs">
              {monitor.type.toUpperCase()}
            </span>
          </div>
        </div>
        <div className={`flex items-center px-3 py-1 rounded-full text-sm font-medium ${
          isUp ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
        }`}>
          {isUp ? <ArrowUp className="w-4 h-4 mr-1" /> : <ArrowDown className="w-4 h-4 mr-1" />}
          {monitor.status.toUpperCase()}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 mb-4">
        <div className="bg-gray-50 dark:bg-gray-800 p-3 rounded-lg">
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">Latency</div>
          <div className="text-lg font-mono font-medium text-gray-900 dark:text-white">
            {monitor.latency}ms
          </div>
        </div>
        <div className="bg-gray-50 dark:bg-gray-800 p-3 rounded-lg">
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">Last Check</div>
          <div className="text-sm font-medium text-gray-900 dark:text-white">
            {monitor.last_check ? formatDistanceToNow(new Date(monitor.last_check), { addSuffix: true }) : 'Never'}
          </div>
        </div>
      </div>

      {/* Heartbeat Bar (Last 60 checks / 1 Hour) */}
      <div className="flex gap-[2px] h-8 items-end" title="Last 60 checks (1 Hour)">
        {/* Fill with empty bars if not enough history to keep alignment right-aligned */}
        {Array.from({ length: Math.max(0, 60 - (history?.length || 0)) }).map((_, i) => (
           <div key={`empty-${i}`} className="flex-1 bg-gray-100 dark:bg-gray-700 rounded-sm h-full opacity-50" />
        ))}

        {history?.slice().reverse().map((beat: any, i: number) => (
          <div
            key={i}
            className={`flex-1 rounded-sm transition-all duration-200 hover:scale-110 ${
              beat.status === 'up'
                ? 'bg-green-400 dark:bg-green-500 hover:bg-green-300'
                : 'bg-red-400 dark:bg-red-500 hover:bg-red-300'
            }`}
            style={{ height: '100%' }}
            title={`${new Date(beat.created_at).toLocaleString()}
Status: ${beat.status.toUpperCase()}
Latency: ${beat.latency}ms
Message: ${beat.message}`}
          />
        ))}
        {(!history || history.length === 0) && (
            <div className="absolute w-full text-center text-xs text-gray-400">No history available</div>
        )}
      </div>
    </div>
  );
};

const Uptime: React.FC = () => {
  const { data: monitors, isLoading } = useQuery({
    queryKey: ['monitors'],
    queryFn: getMonitors,
    refetchInterval: 30000,
  });

  if (isLoading) {
    return <div className="p-8 text-center">Loading monitors...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
          <Activity className="w-6 h-6" />
          Uptime Monitoring
        </h1>
        <div className="text-sm text-gray-500">
          Auto-refreshing every 30s
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {monitors?.map((monitor) => (
          <MonitorCard key={monitor.id} monitor={monitor} />
        ))}
        {monitors?.length === 0 && (
          <div className="col-span-full text-center py-12 text-gray-500">
            No monitors found. Add a Proxy Host to start monitoring.
          </div>
        )}
      </div>
    </div>
  );
};

export default Uptime;
