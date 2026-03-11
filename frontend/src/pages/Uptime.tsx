import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';
import { Activity, ArrowUp, ArrowDown, Settings, X, Pause, RefreshCw, Plus, Loader } from 'lucide-react';
import { useMemo, useState, type FC, type FormEvent } from 'react';
import { toast } from 'react-hot-toast'
import { useTranslation } from 'react-i18next';

import { getMonitors, getMonitorHistory, updateMonitor, deleteMonitor, checkMonitor, createMonitor, syncMonitors, type UptimeMonitor } from '../api/uptime';


type BaseMonitorStatus = 'up' | 'down' | 'pending';
type EffectiveMonitorStatus = BaseMonitorStatus | 'paused';

const normalizeMonitorStatus = (status: string | undefined): BaseMonitorStatus => {
  const normalized = status?.toLowerCase();
  if (normalized === 'up' || normalized === 'down' || normalized === 'pending') {
    return normalized;
  }

  return 'down';
};

const MonitorCard: FC<{ monitor: UptimeMonitor; onEdit: (monitor: UptimeMonitor) => void; t: (key: string, options?: Record<string, unknown>) => string }> = ({ monitor, onEdit, t }) => {
  const { data: history } = useQuery({
    queryKey: ['uptimeHistory', monitor.id],
    queryFn: () => getMonitorHistory(monitor.id, 60),
    refetchInterval: 60000,
  });

  const queryClient = useQueryClient()

  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      return await deleteMonitor(id)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['monitors'] })
      toast.success(t('uptime.monitorDeleted'))
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : t('uptime.failedToDeleteMonitor'))
    }
  })

  const toggleMutation = useMutation({
    mutationFn: async ({ id, enabled }: { id: string; enabled: boolean }) => {
      return await updateMonitor(id, { enabled })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['monitors'] })
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : t('uptime.failedToUpdateMonitor'))
    }
  })

  const checkMutation = useMutation({
    mutationFn: async (id: string) => {
      return await checkMonitor(id)
    },
    onSuccess: () => {
      toast.success(t('uptime.healthCheckTriggered'))
      // Refetch monitor and history after a short delay to get updated results
      setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: ['monitors'] })
        queryClient.invalidateQueries({ queryKey: ['uptimeHistory', monitor.id] })
      }, 2000)
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : t('uptime.failedToTriggerCheck'))
    }
  })

  const [showMenu, setShowMenu] = useState(false)

  // Determine current status from most recent heartbeat when available
  const latestBeat = history && history.length > 0
    ? history.reduce((a, b) => new Date(a.created_at) > new Date(b.created_at) ? a : b)
    : null

  const hasHistory = Boolean(history && history.length > 0);
  const isPaused = monitor.enabled === false;
  const effectiveStatus: EffectiveMonitorStatus = isPaused
    ? 'paused'
    : latestBeat
      ? (latestBeat.status === 'up' ? 'up' : 'down')
      : monitor.status === 'pending' && !hasHistory
        ? 'pending'
        : normalizeMonitorStatus(monitor.status);

  return (
    <div className={`bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4 border-l-4 ${effectiveStatus === 'paused' ? 'border-l-yellow-400' : effectiveStatus === 'pending' ? 'border-l-amber-500' : effectiveStatus === 'up' ? 'border-l-green-500' : 'border-l-red-500'}`} data-testid="monitor-card">
      {/* Top Row: Name (left), Badge (center-right), Settings (right) */}
      <div className="flex items-center justify-between mb-4">
        <h3 className="font-semibold text-lg text-gray-900 dark:text-white flex-1 min-w-0 truncate">{monitor.name}</h3>
        <div className="flex items-center gap-2 shrink-0">
          <div className={`flex items-center justify-center px-3 py-1 rounded-full text-sm font-medium min-w-[90px] ${
            effectiveStatus === 'paused'
              ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
              : effectiveStatus === 'pending'
                ? 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200 animate-pulse motion-reduce:animate-none'
                : effectiveStatus === 'up'
                  ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                  : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
          }`} data-testid="status-badge" data-status={effectiveStatus} role="status" aria-label={effectiveStatus === 'paused' ? t('uptime.paused') : effectiveStatus === 'pending' ? t('uptime.pending') : effectiveStatus === 'up' ? 'UP' : 'DOWN'}>
            {effectiveStatus === 'paused' ? <Pause className="w-4 h-4 mr-1" /> : effectiveStatus === 'pending' ? <Loader className="w-4 h-4 mr-1 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : effectiveStatus === 'up' ? <ArrowUp className="w-4 h-4 mr-1" /> : <ArrowDown className="w-4 h-4 mr-1" />}
            {effectiveStatus === 'paused' ? t('uptime.paused') : effectiveStatus === 'pending' ? t('uptime.pending') : effectiveStatus.toUpperCase()}
          </div>
          <button
            onClick={async () => {
              try {
                await checkMutation.mutateAsync(monitor.id)
              } catch {
                // handled in onError
              }
            }}
            disabled={checkMutation.isPending}
            className="p-1 text-gray-400 hover:text-blue-400 transition-colors disabled:opacity-50"
            title={t('uptime.triggerHealthCheck')}
          >
            <RefreshCw size={16} className={checkMutation.isPending ? 'animate-spin' : ''} />
          </button>
          <div className="relative">
            <button
              onClick={() => setShowMenu(prev => !prev)}
              className="p-1 text-gray-400 hover:text-gray-200 transition-colors"
              title={t('uptime.monitorSettings')}
              aria-haspopup="menu"
              aria-expanded={showMenu}
            >
              <Settings size={16} />
            </button>

            {showMenu && (
              <div className="absolute right-0 mt-2 w-48 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded shadow-lg z-20">
                <button
                  onClick={() => { setShowMenu(false); onEdit(monitor) }}
                  className="w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-900"
                >
                  {t('common.configure')}
                </button>
                <button
                  onClick={async () => {
                    setShowMenu(false)
                    try {
                      await toggleMutation.mutateAsync({ id: monitor.id, enabled: !monitor.enabled })
                      toast.success(monitor.enabled ? t('uptime.paused') : t('uptime.unpaused'))
                    } catch {
                      // handled in onError
                    }
                  }}
                  className="w-full text-left px-4 py-2 text-sm text-yellow-600 hover:bg-gray-100 dark:hover:bg-gray-900 flex items-center gap-2"
                >
                  <Pause className="w-4 h-4 mr-1" />
                  {monitor.enabled ? t('uptime.pause') : t('uptime.unpause')}
                </button>
                <button
                  onClick={async () => {
                    setShowMenu(false)
                    const confirmDelete = confirm(t('uptime.deleteConfirmation'))
                    if (!confirmDelete) return
                    try {
                      await deleteMutation.mutateAsync(monitor.id)
                    } catch {
                      // handled in onError
                    }
                  }}
                  className="w-full text-left px-4 py-2 text-sm text-red-600 hover:bg-gray-100 dark:hover:bg-gray-900"
                >
                  {t('common.delete')}
                </button>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* URL and Type */}
      <div className="text-sm text-gray-500 dark:text-gray-400 flex items-center gap-2 mb-4">
        <a href={monitor.url} target="_blank" rel="noreferrer" className="hover:underline">
          {monitor.url}
        </a>
        <span className="px-2 py-0.5 rounded-full bg-gray-100 dark:bg-gray-700 text-xs">
          {monitor.type.toUpperCase()}
        </span>
      </div>

      <div className="grid grid-cols-2 gap-4 mb-4">
        <div className="bg-gray-50 dark:bg-gray-800 p-3 rounded-lg">
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">{t('uptime.latency')}</div>
          <div className="text-lg font-mono font-medium text-gray-900 dark:text-white">
            {monitor.latency}ms
          </div>
        </div>
        <div className="bg-gray-50 dark:bg-gray-800 p-3 rounded-lg" data-testid="last-check">
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">{t('uptime.lastCheck')}</div>
          <div className="text-sm font-medium text-gray-900 dark:text-white">
            {monitor.last_check ? formatDistanceToNow(new Date(monitor.last_check), { addSuffix: true }) : t('uptime.never')}
          </div>
        </div>
      </div>

      {/* Heartbeat Bar (Last 60 checks / 1 Hour) */}
      <div className="flex gap-0.5 h-8 items-end relative" title={t('uptime.last60Checks')} data-testid="heartbeat-bar">
        {/* Fill with empty bars if not enough history to keep alignment right-aligned */}
        {Array.from({ length: Math.max(0, 60 - (history?.length || 0)) }).map((_, i) => (
           <div key={`empty-${i}`} className="flex-1 bg-gray-100 dark:bg-gray-700 rounded-sm h-full opacity-50" />
        ))}

        {history?.slice().reverse().map((beat: { status: string; created_at: string; latency: number; message: string }, i: number) => (
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
          <div className="absolute w-full text-center text-xs text-gray-400">{effectiveStatus === 'pending' ? t('uptime.pendingFirstCheck') : t('uptime.noHistoryAvailable')}</div>
        )}
      </div>
    </div>
  );
};

const EditMonitorModal: FC<{ monitor: UptimeMonitor; onClose: () => void; t: (key: string) => string }> = ({ monitor, onClose, t }) => {
    const queryClient = useQueryClient();
    const [name, setName] = useState(monitor.name || '')
    const [maxRetries, setMaxRetries] = useState(monitor.max_retries || 3);
    const [interval, setInterval] = useState(monitor.interval || 60);

    const mutation = useMutation({
      mutationFn: (data: Partial<UptimeMonitor>) => updateMonitor(monitor.id, data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['monitors'] });
            onClose();
        },
    });

    const handleSubmit = (e: FormEvent) => {
        e.preventDefault();
      mutation.mutate({ name, max_retries: maxRetries, interval });
    };

    return (
        <>
            {/* Layer 1: Background overlay (z-40) */}
            <div className="fixed inset-0 bg-black/50 z-40" onClick={onClose} />

            {/* Layer 2: Form container (z-50, pointer-events-none) */}
            <div className="fixed inset-0 flex items-center justify-center p-4 pointer-events-none z-50">

                {/* Layer 3: Form content (pointer-events-auto) */}
                <div className="bg-gray-800 rounded-lg border border-gray-700 max-w-md w-full p-6 shadow-xl pointer-events-auto">
                <div className="flex justify-between items-center mb-6">
                    <h2 className="text-xl font-bold text-white">{t('uptime.configureMonitor')}</h2>
                    <button onClick={onClose} className="text-gray-400 hover:text-white">
                        <X size={24} />
                    </button>
                </div>

                <form onSubmit={handleSubmit} className="space-y-4 pointer-events-auto">
                <div>
                  <label htmlFor="monitor-name" className="block text-sm font-medium text-gray-300 mb-1">
                    {t('common.name')}
                  </label>
                  <input
                    id="monitor-name"
                    type="text"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-300 mb-1">
                            {t('uptime.maxRetries')}
                        </label>
                        <input
                            type="number"
                            min="1"
                            max="10"
                            value={maxRetries}
                            onChange={(e) => {
                                const v = parseInt(e.target.value)
                                setMaxRetries(Number.isNaN(v) ? 0 : v)
                            }}
                            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                        <p className="text-xs text-gray-500 mt-1">
                            {t('uptime.maxRetriesHelper')}
                        </p>
                    </div>

                    <div>
                        <label className="block text-sm font-medium text-gray-300 mb-1">
                            {t('uptime.checkInterval')}
                        </label>
                        <input
                            type="number"
                            min="10"
                            max="3600"
                            value={interval}
                            onChange={(e) => {
                                const v = parseInt(e.target.value)
                                setInterval(Number.isNaN(v) ? 0 : v)
                            }}
                            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                    </div>

                    <div className="flex justify-end gap-3 pt-4">
                        <button
                            type="button"
                            onClick={onClose}
                            className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg text-sm transition-colors"
                        >
                            {t('common.cancel')}
                        </button>
                        <button
                            type="submit"
                            disabled={mutation.isPending}
                            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50"
                        >
                            {mutation.isPending ? t('common.saving') : t('uptime.saveChanges')}
                        </button>
                    </div>
                </form>
                </div>
            </div>
        </>
    );
};

const CreateMonitorModal: FC<{ onClose: () => void; t: (key: string) => string }> = ({ onClose, t }) => {
    const queryClient = useQueryClient();
    const [name, setName] = useState('');
    const [url, setUrl] = useState('');
    const [type, setType] = useState<'http' | 'tcp'>('http');
    const [interval, setInterval] = useState(60);
    const [maxRetries, setMaxRetries] = useState(3);

    const mutation = useMutation({
        mutationFn: (data: { name: string; url: string; type: string; interval?: number; max_retries?: number }) =>
            createMonitor(data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['monitors'] });
            toast.success(t('uptime.monitorCreated'));
            onClose();
        },
        onError: (err: unknown) => {
            toast.error(err instanceof Error ? err.message : t('errors.genericError'));
        },
    });

    const handleSubmit = (e: FormEvent) => {
        e.preventDefault();
        if (!name.trim() || !url.trim()) return;
        mutation.mutate({ name: name.trim(), url: url.trim(), type, interval, max_retries: maxRetries });
    };

    return (
        <>
            {/* Layer 1: Background overlay (z-40) */}
            <div className="fixed inset-0 bg-black/50 z-40" onClick={onClose} />

            {/* Layer 2: Form container (z-50, pointer-events-none) */}
            <div className="fixed inset-0 flex items-center justify-center p-4 pointer-events-none z-50">

                {/* Layer 3: Form content (pointer-events-auto) */}
                <div className="bg-gray-800 rounded-lg border border-gray-700 max-w-md w-full p-6 shadow-xl pointer-events-auto">
                    <div className="flex justify-between items-center mb-6">
                        <h2 className="text-xl font-bold text-white">{t('uptime.createMonitor')}</h2>
                        <button onClick={onClose} className="text-gray-400 hover:text-white" aria-label={t('common.close')}>
                            <X size={24} />
                        </button>
                    </div>

                    <form onSubmit={handleSubmit} className="space-y-4 pointer-events-auto">
                    <div>
                        <label htmlFor="create-monitor-name" className="block text-sm font-medium text-gray-300 mb-1">
                            {t('common.name')} *
                        </label>
                        <input
                            id="create-monitor-name"
                            type="text"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            required
                            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                            placeholder="My Service"
                        />
                    </div>

                    <div>
                        <label htmlFor="create-monitor-url" className="block text-sm font-medium text-gray-300 mb-1">
                            {t('uptime.monitorUrl')} *
                        </label>
                        <input
                            id="create-monitor-url"
                            type="text"
                            value={url}
                            onChange={(e) => setUrl(e.target.value)}
                            required
                            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                            placeholder={t('uptime.urlPlaceholder')}
                        />
                    </div>

                    <div>
                        <label htmlFor="create-monitor-type" className="block text-sm font-medium text-gray-300 mb-1">
                            {t('uptime.monitorType')} *
                        </label>
                        <select
                            id="create-monitor-type"
                            value={type}
                            onChange={(e) => setType(e.target.value as 'http' | 'tcp')}
                            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        >
                            <option value="http">{t('uptime.monitorTypeHttp')}</option>
                            <option value="tcp">{t('uptime.monitorTypeTcp')}</option>
                        </select>
                    </div>

                    <div>
                        <label htmlFor="create-monitor-interval" className="block text-sm font-medium text-gray-300 mb-1">
                            {t('uptime.checkInterval')}
                        </label>
                        <input
                            id="create-monitor-interval"
                            type="number"
                            min="10"
                            max="3600"
                            value={interval}
                            onChange={(e) => {
                                const v = parseInt(e.target.value);
                                setInterval(Number.isNaN(v) ? 60 : v);
                            }}
                            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                    </div>

                    <div>
                        <label htmlFor="create-monitor-retries" className="block text-sm font-medium text-gray-300 mb-1">
                            {t('uptime.maxRetries')}
                        </label>
                        <input
                            id="create-monitor-retries"
                            type="number"
                            min="1"
                            max="10"
                            value={maxRetries}
                            onChange={(e) => {
                                const v = parseInt(e.target.value);
                                setMaxRetries(Number.isNaN(v) ? 3 : v);
                            }}
                            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                        <p className="text-xs text-gray-500 mt-1">
                            {t('uptime.maxRetriesHelper')}
                        </p>
                    </div>

                    <div className="flex justify-end gap-3 pt-4">
                        <button
                            type="button"
                            onClick={onClose}
                            className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg text-sm transition-colors"
                        >
                            {t('common.cancel')}
                        </button>
                        <button
                            type="submit"
                            disabled={mutation.isPending || !name.trim() || !url.trim()}
                            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50"
                        >
                            {mutation.isPending ? t('common.saving') : t('common.create')}
                        </button>
                    </div>
                </form>
                </div>
            </div>
        </>
    );
};

const Uptime: FC = () => {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data: monitors, isLoading } = useQuery({
    queryKey: ['monitors'],
    queryFn: getMonitors,
    refetchInterval: 30000,
  });

  const [editingMonitor, setEditingMonitor] = useState<UptimeMonitor | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);

  const syncMutation = useMutation({
    mutationFn: () => syncMonitors(),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['monitors'] });
      toast.success(data.message || t('uptime.syncComplete'));
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : t('errors.genericError'));
    },
  });

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
    return <div className="p-8 text-center">{t('uptime.loadingMonitors')}</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center" data-testid="uptime-summary">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
          <Activity className="w-6 h-6" />
          {t('uptime.title')}
        </h1>
        <div className="flex items-center gap-2">
          <button
            onClick={() => syncMutation.mutate()}
            disabled={syncMutation.isPending}
            data-testid="sync-button"
            className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            <RefreshCw size={16} className={syncMutation.isPending ? 'animate-spin' : ''} />
            {syncMutation.isPending ? t('uptime.syncing') : t('uptime.syncWithHosts')}
          </button>
          <button
            onClick={() => setShowCreateModal(true)}
            data-testid="add-monitor-button"
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors flex items-center gap-2"
          >
            <Plus size={16} />
            {t('uptime.addMonitor')}
          </button>
          <span className="text-sm text-gray-500">{t('uptime.autoRefreshing')}</span>
        </div>
      </div>

      {sortedMonitors.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          {t('uptime.noMonitorsFound')}
        </div>
      ) : (
        <>
          {proxyHostMonitors.length > 0 && (
            <div className="mb-8">
              <h2 className="text-xl font-semibold text-gray-800 dark:text-gray-200 mb-4 border-b border-gray-200 dark:border-gray-700 pb-2">{t('uptime.proxyHosts')}</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {proxyHostMonitors.map((monitor) => (
                  <MonitorCard key={monitor.id} monitor={monitor} onEdit={setEditingMonitor} t={t} />
                ))}
              </div>
            </div>
          )}

          {remoteServerMonitors.length > 0 && (
            <div className="mb-8">
              <h2 className="text-xl font-semibold text-gray-800 dark:text-gray-200 mb-4 border-b border-gray-200 dark:border-gray-700 pb-2">{t('uptime.remoteServers')}</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {remoteServerMonitors.map((monitor) => (
                  <MonitorCard key={monitor.id} monitor={monitor} onEdit={setEditingMonitor} t={t} />
                ))}
              </div>
            </div>
          )}

          {otherMonitors.length > 0 && (
            <div className="mb-8">
              <h2 className="text-xl font-semibold text-gray-800 dark:text-gray-200 mb-4 border-b border-gray-200 dark:border-gray-700 pb-2">{t('uptime.otherMonitors')}</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {otherMonitors.map((monitor) => (
                  <MonitorCard key={monitor.id} monitor={monitor} onEdit={setEditingMonitor} t={t} />
                ))}
              </div>
            </div>
          )}
        </>
      )}

      {editingMonitor && (
        <EditMonitorModal monitor={editingMonitor} onClose={() => setEditingMonitor(null)} t={t} />
      )}

      {showCreateModal && (
        <CreateMonitorModal onClose={() => setShowCreateModal(false)} t={t} />
      )}
    </div>
  );
};

export default Uptime;
