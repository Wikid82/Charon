import React, { useMemo, useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getMonitors, getMonitorHistory, updateMonitor, UptimeMonitor } from '../api/uptime';
import { Activity, ArrowUp, ArrowDown, Settings, X } from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';

const MonitorCard: React.FC<{ monitor: UptimeMonitor; onEdit: (monitor: UptimeMonitor) => void }> = ({ monitor, onEdit }) => {
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
            <a href={monitor.url} target="_blank" rel="noreferrer" className="hover:underline">
              {monitor.url}
            </a>
            <span className="px-2 py-0.5 rounded-full bg-gray-100 dark:bg-gray-700 text-xs">
              {monitor.type.toUpperCase()}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-2">
            <button
                onClick={() => onEdit(monitor)}
                className="p-1 text-gray-400 hover:text-gray-200 transition-colors"
                title="Configure Monitor"
            >
                <Settings size={16} />
            </button>
            <div className={`flex items-center px-3 py-1 rounded-full text-sm font-medium ${
            isUp ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
            }`}>
            {isUp ? <ArrowUp className="w-4 h-4 mr-1" /> : <ArrowDown className="w-4 h-4 mr-1" />}
            {monitor.status.toUpperCase()}
            </div>
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

const EditMonitorModal: React.FC<{ monitor: UptimeMonitor; onClose: () => void }> = ({ monitor, onClose }) => {
    const queryClient = useQueryClient();
    const [maxRetries, setMaxRetries] = useState(monitor.max_retries || 3);
    const [interval, setInterval] = useState(monitor.interval || 60);

    const mutation = useMutation({
        mutationFn: (data: Partial<UptimeMonitor>) => updateMonitor(monitor.id, data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['monitors'] });
            onClose();
        },
    });

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        mutation.mutate({ max_retries: maxRetries, interval });
    };

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
            <div className="bg-gray-800 rounded-lg border border-gray-700 max-w-md w-full p-6 shadow-xl">
                <div className="flex justify-between items-center mb-6">
                    <h2 className="text-xl font-bold text-white">Configure Monitor</h2>
                    <button onClick={onClose} className="text-gray-400 hover:text-white">
                        <X size={24} />
                    </button>
                </div>

                <form onSubmit={handleSubmit} className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-300 mb-1">
                            Max Retries
                        </label>
                        <input
                            type="number"
                            min="1"
                            max="10"
                            value={maxRetries}
                            onChange={(e) => setMaxRetries(parseInt(e.target.value))}
                            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                        <p className="text-xs text-gray-500 mt-1">
                            Number of consecutive failures before sending an alert.
                        </p>
                    </div>

                    <div>
                        <label className="block text-sm font-medium text-gray-300 mb-1">
                            Check Interval (seconds)
                        </label>
                        <input
                            type="number"
                            min="10"
                            max="3600"
                            value={interval}
                            onChange={(e) => setInterval(parseInt(e.target.value))}
                            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                    </div>

                    <div className="flex justify-end gap-3 pt-4">
                        <button
                            type="button"
                            onClick={onClose}
                            className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg text-sm transition-colors"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            disabled={mutation.isPending}
                            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50"
                        >
                            {mutation.isPending ? 'Saving...' : 'Save Changes'}
                        </button>
                    </div>
                </form>
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

  const [editingMonitor, setEditingMonitor] = useState<UptimeMonitor | null>(null);

  // Sort monitors alphabetically by name
  const sortedMonitors = useMemo(() => {
    if (!monitors) return [];
    return [...monitors].sort((a, b) =>
      (a.name || '').toLowerCase().localeCompare((b.name || '').toLowerCase())
    );
  }, [monitors]);

  const proxyHostMonitors = useMemo(() => sortedMonitors.filter(m => m.proxy_host_id), [sortedMonitors]);
  const remoteServerMonitors = useMemo(() => sortedMonitors.filter(m => m.remote_server_id), [sortedMonitors]);
  const otherMonitors = useMemo(() => sortedMonitors.filter(m => !m.proxy_host_id && !m.remote_server_id), [sortedMonitors]);

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

      {sortedMonitors.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          No monitors found. Add a Proxy Host or Remote Server to start monitoring.
        </div>
      ) : (
        <>
          {proxyHostMonitors.length > 0 && (
            <div className="mb-8">
              <h2 className="text-xl font-semibold text-gray-800 dark:text-gray-200 mb-4 border-b border-gray-200 dark:border-gray-700 pb-2">Proxy Hosts</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {proxyHostMonitors.map((monitor) => (
                  <MonitorCard key={monitor.id} monitor={monitor} onEdit={setEditingMonitor} />
                ))}
              </div>
            </div>
          )}

          {remoteServerMonitors.length > 0 && (
            <div className="mb-8">
              <h2 className="text-xl font-semibold text-gray-800 dark:text-gray-200 mb-4 border-b border-gray-200 dark:border-gray-700 pb-2">Remote Servers</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {remoteServerMonitors.map((monitor) => (
                  <MonitorCard key={monitor.id} monitor={monitor} onEdit={setEditingMonitor} />
                ))}
              </div>
            </div>
          )}

          {otherMonitors.length > 0 && (
            <div className="mb-8">
              <h2 className="text-xl font-semibold text-gray-800 dark:text-gray-200 mb-4 border-b border-gray-200 dark:border-gray-700 pb-2">Other Monitors</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {otherMonitors.map((monitor) => (
                  <MonitorCard key={monitor.id} monitor={monitor} onEdit={setEditingMonitor} />
                ))}
              </div>
            </div>
          )}
        </>
      )}

      {editingMonitor && (
        <EditMonitorModal monitor={editingMonitor} onClose={() => setEditingMonitor(null)} />
      )}
    </div>
  );
};

export default Uptime;
