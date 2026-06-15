import { Check, CircleHelp, Loader2, X } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { type RemoteServer, testCustomRemoteServerConnection } from '../api/remoteServers'
import { useAgentList } from '../hooks/useOrthrus'
import {
  ConnectionTypeSelector,
  type ConnectionMode,
  type ConnectionType,
  type HecateProvider,
} from './hecate/ConnectionTypeSelector'
import { Tooltip, TooltipContent, TooltipTrigger, TooltipProvider } from './ui/Tooltip'

interface Props {
  server?: RemoteServer
  onSubmit: (data: Partial<RemoteServer>) => Promise<void>
  onCancel: () => void
}

export default function RemoteServerForm({ server, onSubmit, onCancel }: Props) {
  const { t } = useTranslation()

  const resolveConnectionMode = (): ConnectionMode => {
    if (!server?.connection_type || server.connection_type === 'direct') return 'direct'
    if (server.connection_type === 'orthrus') return 'agent'
    return 'provider'
  }

  const [formData, setFormData] = useState({
    name: server?.name || '',
    provider: server?.provider || 'generic',
    host: server?.host || '',
    port: server?.port ?? 22,
    username: server?.username || '',
    enabled: server?.enabled ?? true,
    connection_mode: resolveConnectionMode() as ConnectionMode,
    connection_type: (server?.connection_type ?? 'direct') as ConnectionType,
    orthrus_agent_uuid: server?.orthrus_agent_uuid ?? '',
    hecate_tunnel_uuid: server?.hecate_tunnel_uuid ?? '',
    device_id: '',
    resolved_address: '',
  })

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [testStatus, setTestStatus] = useState<'idle' | 'testing' | 'success' | 'error'>('idle')

  const { data: agents = [] } = useAgentList()

  const selectedAgent = agents.find(a => a.uuid === formData.orthrus_agent_uuid)
  const agentHasNoProvider =
    formData.connection_mode === 'agent' &&
    Boolean(formData.orthrus_agent_uuid) &&
    !selectedAgent?.resolved_address

  useEffect(() => {
    setFormData({
      name: server?.name || '',
      provider: server?.provider || 'generic',
      host: server?.host || '',
      port: server?.port ?? 22,
      username: server?.username || '',
      enabled: server?.enabled ?? true,
      connection_mode: resolveConnectionMode(),
      connection_type: (server?.connection_type ?? 'direct') as ConnectionType,
      orthrus_agent_uuid: server?.orthrus_agent_uuid ?? '',
      hecate_tunnel_uuid: server?.hecate_tunnel_uuid ?? '',
      device_id: '',
      resolved_address: '',
    })
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [server])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)
    try {
      const payload: Partial<RemoteServer> = {
        name: formData.name,
        provider: formData.provider,
        enabled: formData.enabled,
        connection_type: formData.connection_type,
        orthrus_agent_uuid: formData.orthrus_agent_uuid || undefined,
        hecate_tunnel_uuid: formData.hecate_tunnel_uuid || undefined,
      }
      if (formData.connection_mode === 'direct') {
        payload.host = formData.host
        payload.port = formData.port
        payload.username = formData.username
      } else if (formData.connection_mode === 'agent') {
        const agent = agents.find(a => a.uuid === formData.orthrus_agent_uuid)
        payload.host = agent?.resolved_address ?? formData.host
      } else if (formData.connection_mode === 'provider') {
        payload.host = formData.resolved_address
      }
      await onSubmit(payload)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save server')
    } finally {
      setLoading(false)
    }
  }

  const handleTestConnection = async () => {
    if (!formData.host || !formData.port) return
    setTestStatus('testing')
    setError(null)
    try {
      const result = await testCustomRemoteServerConnection(formData.host, formData.port)
      if (result.reachable) {
        setTestStatus('success')
        setTimeout(() => setTestStatus('idle'), 3000)
      } else {
        setTestStatus('error')
        setError(`Connection failed: ${result.error || 'Unknown error'}`)
      }
    } catch {
      setTestStatus('error')
      setError('Connection failed')
    }
  }

  return (
    <>
      {/* Layer 1: Background overlay (z-40) */}
      <div
        className="fixed inset-0 bg-black/50 z-40"
        onClick={onCancel}
        onKeyDown={(e) => e.key === 'Escape' && onCancel()}
        role="button"
        tabIndex={-1}
        aria-label={t('common.cancel')}
      />

      {/* Layer 2: Form container (z-50, pointer-events-none) */}
      <div className="fixed inset-0 flex items-center justify-center p-4 pointer-events-none z-50">

        {/* Layer 3: Form content (pointer-events-auto) */}
        <div className="bg-dark-card rounded-lg border border-gray-800 max-w-lg w-full pointer-events-auto">
        <div className="p-6 border-b border-gray-800">
          <h2 className="text-2xl font-bold text-white">
            {server ? 'Edit Remote Server' : 'Add Remote Server'}
          </h2>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-6 pointer-events-auto">
          {error && (
            <div className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded">
              {error}
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2" htmlFor="name">Name</label>
            <input
              id="name"
              type="text"
              required
              value={formData.name}
              onChange={e => setFormData({ ...formData, name: e.target.value })}
              placeholder="My Production Server"
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2" htmlFor="provider">
              {t('remoteServers.columnProvider')}
            </label>
            <select
              id="provider"
              value={formData.provider}
              onChange={e => {
                const newProvider = e.target.value;
                setFormData({
                  ...formData,
                  provider: newProvider,
                  port: newProvider === 'docker' ? 2375 : (newProvider === 'generic' ? 22 : formData.port)
                })
              }}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="generic">Generic</option>
              <option value="docker">Docker</option>
              <option value="kubernetes">Kubernetes</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              {t('remoteServers.connectionType')}
            </label>
          <ConnectionTypeSelector
              mode={formData.connection_mode}
              onModeChange={mode => {
                if (mode === 'direct') {
                  setFormData(prev => ({
                    ...prev,
                    connection_mode: 'direct',
                    connection_type: 'direct',
                    hecate_tunnel_uuid: '',
                    orthrus_agent_uuid: '',
                    device_id: '',
                    resolved_address: '',
                  }))
                } else if (mode === 'agent') {
                  setFormData(prev => ({
                    ...prev,
                    connection_mode: 'agent',
                    connection_type: 'orthrus',
                    hecate_tunnel_uuid: '',
                    device_id: '',
                    resolved_address: '',
                  }))
                } else {
                  setFormData(prev => ({
                    ...prev,
                    connection_mode: 'provider',
                    connection_type: 'direct',
                    orthrus_agent_uuid: '',
                    device_id: '',
                    resolved_address: '',
                  }))
                }
              }}
              selectedTunnelUUID={formData.hecate_tunnel_uuid || null}
              selectedAgentUUID={formData.orthrus_agent_uuid || null}
              selectedDeviceId={formData.device_id}
              onTunnelSelect={(uuid, provider) =>
                setFormData(prev => ({
                  ...prev,
                  connection_type: provider as HecateProvider,
                  hecate_tunnel_uuid: uuid,
                  orthrus_agent_uuid: '',
                  device_id: '',
                  resolved_address: '',
                }))
              }
              onAgentSelect={uuid =>
                setFormData(prev => ({
                  ...prev,
                  connection_type: 'orthrus',
                  orthrus_agent_uuid: uuid,
                  hecate_tunnel_uuid: '',
                  device_id: '',
                  resolved_address: '',
                }))
              }
              onDeviceSelect={(deviceId, address) =>
                setFormData(prev => ({
                  ...prev,
                  device_id: deviceId,
                  resolved_address: address,
                }))
              }
              disabled={loading}
            />
            {formData.connection_mode === 'provider' && formData.resolved_address && (
              <p
                role="status"
                aria-live="polite"
                className="mt-2 text-xs text-content-secondary"
              >
                {t('hecate.form.mode.resolvedAddressPreview', { address: formData.resolved_address })}
              </p>
            )}
          </div>

          {formData.connection_mode === 'direct' && (
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2" htmlFor="host">
                {t('remoteServers.host')}
              </label>
              <input
                id="host"
                type="text"
                required
                value={formData.host}
                onChange={e => setFormData({ ...formData, host: e.target.value })}
                placeholder="192.168.1.100"
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          )}

          {formData.connection_mode === 'direct' && (
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2" htmlFor="port">
                {t('remoteServers.port')}
              </label>
              <input
                id="port"
                type="number"
                min={1}
                max={65535}
                value={formData.port}
                onChange={e => {
                  const v = parseInt(e.target.value)
                  setFormData({ ...formData, port: Number.isNaN(v) ? 0 : v })
                }}
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            {formData.provider !== 'docker' && (
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2" htmlFor="username">
                  {t('remoteServers.user')}
                </label>
                <input
                  id="username"
                  type="text"
                  value={formData.username}
                  onChange={e => setFormData({ ...formData, username: e.target.value })}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            )}
          </div>
          )}

          <label className="flex items-center gap-3">
            <input
              type="checkbox"
              checked={formData.enabled}
              onChange={e => setFormData({ ...formData, enabled: e.target.checked })}
              className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
            />
            <span className="text-sm text-gray-300">{t('remoteServers.enabled', 'Enabled')}</span>
          </label>

          <div className="flex gap-3 justify-end pt-4 border-t border-gray-800">
            <button
              type="button"
              onClick={handleTestConnection}
              disabled={testStatus === 'testing' || formData.connection_mode !== 'direct' || !formData.host || !formData.port}
              className={`px-4 py-2 rounded-lg font-medium transition-colors flex items-center gap-2 mr-auto ${
                testStatus === 'success' ? 'bg-green-600 text-white' :
                testStatus === 'error' ? 'bg-red-600 text-white' :
                'bg-gray-700 hover:bg-gray-600 text-white'
              }`}
            >
              {testStatus === 'testing' ? <Loader2 className="w-4 h-4 animate-spin" /> :
               testStatus === 'success' ? <Check className="w-4 h-4" /> :
               testStatus === 'error' ? <X className="w-4 h-4" /> :
               <CircleHelp className="w-4 h-4" />}
              Test Connection
            </button>
            <button
              type="button"
              onClick={onCancel}
              disabled={loading}
              className="px-6 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              Cancel
            </button>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className={agentHasNoProvider ? 'cursor-not-allowed' : undefined}>
                    <button
                      type="submit"
                      disabled={loading}
                      className="px-6 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors disabled:opacity-50"
                      aria-describedby={agentHasNoProvider ? 'submit-no-provider-warning' : undefined}
                    >
                      {loading ? 'Saving...' : (server ? 'Update' : 'Create')}
                    </button>
                  </span>
                </TooltipTrigger>
                {agentHasNoProvider && (
                  <TooltipContent id="submit-no-provider-warning" role="tooltip">
                    {t('hecate.form.mode.agent.saveWithNoProviderTooltip')}
                  </TooltipContent>
                )}
              </Tooltip>
            </TooltipProvider>
          </div>
        </form>
        </div>
      </div>
    </>
  )
}
