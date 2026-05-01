import { useQuery } from '@tanstack/react-query'
import { Check, CircleHelp, Loader2, X } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import {
  listTailscaleDevices,
  type NetBirdPeer,
  type TailscaleDevice,
  type ZeroTierMember,
  type ZeroTierNetwork,
} from '../api/hecate'
import { type RemoteServer, testCustomRemoteServerConnection } from '../api/remoteServers'
import { useHecate } from '../hooks/useHecate'
import { useAgentList } from '../hooks/useOrthrus'
import {
  ConnectionTypeSelector,
  type ConnectionMode,
  type ConnectionType,
  type HecateProvider,
} from './hecate/ConnectionTypeSelector'
import { NetBirdPeerPicker } from './hecate/NetBirdPeerPicker'
import { TailscaleDevicePicker } from './hecate/TailscaleDevicePicker'
import { TunnelStatusBadge } from './hecate/TunnelStatusBadge'
import { ZeroTierMemberPicker } from './hecate/ZeroTierMemberPicker'

interface Props {
  server?: RemoteServer
  onSubmit: (data: Partial<RemoteServer>) => Promise<void>
  onCancel: () => void
}

export default function RemoteServerForm({ server, onSubmit, onCancel }: Props) {
  const { t } = useTranslation()

  const resolveConnectionMode = (): ConnectionMode => {
    if (!server?.connection_type || server.connection_type === 'direct') return 'direct'
    return 'agent'
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
    selected_device_name: '',
    selected_device_address: '',
    orthrus_ip_mode: '' as '' | 'tailscale' | 'netbird' | 'zerotier' | 'manual',
  })

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [testStatus, setTestStatus] = useState<'idle' | 'testing' | 'success' | 'error'>('idle')
  const [showTailscalePicker, setShowTailscalePicker] = useState(false)
  const [showNetBirdPicker, setShowNetBirdPicker] = useState(false)
  const [showZeroTierPicker, setShowZeroTierPicker] = useState(false)

  const { tunnels, getStatus } = useHecate()
  const { data: agents = [] } = useAgentList()

  const { data: tailscaleDevices = [] } = useQuery({
    queryKey: ['hecate', 'tailscale', 'devices'],
    queryFn: listTailscaleDevices,
    enabled: formData.connection_type === 'tailscale' || (formData.connection_type === 'orthrus' && formData.orthrus_ip_mode === 'tailscale'),
    staleTime: 60_000,
  })

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
      selected_device_name: '',
      selected_device_address: '',
      orthrus_ip_mode: '' as '' | 'tailscale' | 'netbird' | 'zerotier' | 'manual',
    })
  // eslint-disable-next-line react-compiler/react-compiler
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
      } else if (formData.connection_type === 'orthrus') {
        payload.host = formData.orthrus_ip_mode === 'manual' ? formData.host : formData.selected_device_address
      } else if (['tailscale', 'netbird', 'zerotier'].includes(formData.connection_type)) {
        payload.host = formData.selected_device_address
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

  const selectedTunnel = tunnels.find(t => t.uuid === formData.hecate_tunnel_uuid)
  const selectedAgent = agents.find(a => a.uuid === formData.orthrus_agent_uuid)
  const tunnelStatus = selectedTunnel ? getStatus(formData.hecate_tunnel_uuid) : undefined

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
                    selected_device_name: '',
                    selected_device_address: '',
                    orthrus_ip_mode: '' as '' | 'tailscale' | 'netbird' | 'zerotier' | 'manual',
                  }))
                } else {
                  setFormData(prev => ({ ...prev, connection_mode: 'agent' }))
                }
              }}
              selectedTunnelUUID={formData.hecate_tunnel_uuid || null}
              selectedAgentUUID={formData.orthrus_agent_uuid || null}
              onTunnelSelect={(uuid, provider) =>
                setFormData(prev => ({
                  ...prev,
                  connection_type: provider as HecateProvider,
                  hecate_tunnel_uuid: uuid,
                  orthrus_agent_uuid: '',
                  selected_device_name: '',
                  selected_device_address: '',
                }))
              }
              onAgentSelect={uuid =>
                setFormData(prev => ({
                  ...prev,
                  connection_type: 'orthrus',
                  orthrus_agent_uuid: uuid,
                  hecate_tunnel_uuid: '',
                  selected_device_name: '',
                  selected_device_address: '',
                  orthrus_ip_mode: '' as '' | 'tailscale' | 'netbird' | 'zerotier' | 'manual',
                }))
              }
              disabled={loading}
            />
          </div>

          {/* Tier 3: Device pickers for specific providers */}
          {formData.connection_mode === 'agent' && formData.connection_type === 'tailscale' && (
            <div className="space-y-2">
              {formData.selected_device_name ? (
                <div className="flex items-center gap-2 text-sm">
                  <span className="text-gray-300">{t('hecate.form.mode.selectedDevice')}:</span>
                  <span className="font-medium text-white">{formData.selected_device_name}</span>
                  <span className="text-gray-500">{formData.selected_device_address}</span>
                  <button
                    type="button"
                    onClick={() => setShowTailscalePicker(true)}
                    className="text-blue-400 hover:text-blue-300 underline text-xs focus:outline-none focus:ring-2 focus:ring-blue-500 rounded"
                  >
                    {t('hecate.form.mode.changeDevice')}
                  </button>
                </div>
              ) : (
                <button
                  type="button"
                  onClick={() => setShowTailscalePicker(true)}
                  className="text-sm text-blue-400 hover:text-blue-300 underline focus:outline-none focus:ring-2 focus:ring-blue-500 rounded"
                >
                  {t('hecate.form.mode.selectDevice')}
                </button>
              )}
              <TailscaleDevicePicker
                devices={tailscaleDevices}
                open={showTailscalePicker}
                onClose={() => setShowTailscalePicker(false)}
                onSelect={(device: TailscaleDevice) => {
                  setFormData(prev => ({
                    ...prev,
                    selected_device_name: device.hostname,
                    selected_device_address: device.addresses[0] ?? '',
                  }))
                  setShowTailscalePicker(false)
                }}
                selectedId={formData.selected_device_name}
              />
            </div>
          )}

          {formData.connection_mode === 'agent' && formData.connection_type === 'netbird' && (
            <div className="space-y-2">
              {formData.selected_device_name ? (
                <div className="flex items-center gap-2 text-sm">
                  <span className="text-gray-300">{t('hecate.form.mode.selectedDevice')}:</span>
                  <span className="font-medium text-white">{formData.selected_device_name}</span>
                  <span className="text-gray-500">{formData.selected_device_address}</span>
                  <button
                    type="button"
                    onClick={() => setShowNetBirdPicker(true)}
                    className="text-blue-400 hover:text-blue-300 underline text-xs focus:outline-none focus:ring-2 focus:ring-blue-500 rounded"
                  >
                    {t('hecate.form.mode.changeDevice')}
                  </button>
                </div>
              ) : (
                <button
                  type="button"
                  onClick={() => setShowNetBirdPicker(true)}
                  className="text-sm text-blue-400 hover:text-blue-300 underline focus:outline-none focus:ring-2 focus:ring-blue-500 rounded"
                >
                  {t('hecate.form.mode.selectPeer')}
                </button>
              )}
              <NetBirdPeerPicker
                open={showNetBirdPicker}
                onClose={() => setShowNetBirdPicker(false)}
                onSelect={(peer: NetBirdPeer) => {
                  setFormData(prev => ({
                    ...prev,
                    selected_device_name: peer.name,
                    selected_device_address: peer.ip,
                  }))
                  setShowNetBirdPicker(false)
                }}
                selectedId={formData.selected_device_name}
              />
            </div>
          )}

          {formData.connection_mode === 'agent' && formData.connection_type === 'zerotier' && (
            <div className="space-y-2">
              {formData.selected_device_name ? (
                <div className="flex items-center gap-2 text-sm">
                  <span className="text-gray-300">{t('hecate.form.mode.selectedDevice')}:</span>
                  <span className="font-medium text-white">{formData.selected_device_name}</span>
                  <span className="text-gray-500">{formData.selected_device_address}</span>
                  <button
                    type="button"
                    onClick={() => setShowZeroTierPicker(true)}
                    className="text-blue-400 hover:text-blue-300 underline text-xs focus:outline-none focus:ring-2 focus:ring-blue-500 rounded"
                  >
                    {t('hecate.form.mode.changeDevice')}
                  </button>
                </div>
              ) : (
                <button
                  type="button"
                  onClick={() => setShowZeroTierPicker(true)}
                  className="text-sm text-blue-400 hover:text-blue-300 underline focus:outline-none focus:ring-2 focus:ring-blue-500 rounded"
                >
                  {t('hecate.form.mode.selectMember')}
                </button>
              )}
              <ZeroTierMemberPicker
                open={showZeroTierPicker}
                onClose={() => setShowZeroTierPicker(false)}
                onSelect={(member: ZeroTierMember, _network: ZeroTierNetwork) => {
                  setFormData(prev => ({
                    ...prev,
                    selected_device_name: member.name,
                    selected_device_address: member.ip_assignments[0] ?? member.node_id,
                  }))
                  setShowZeroTierPicker(false)
                }}
                selectedId={formData.selected_device_name}
              />
            </div>
          )}

          {/* Agent info panels */}
          {formData.connection_mode === 'agent' && formData.connection_type === 'orthrus' && selectedAgent && (
            <div className="text-sm text-gray-300 space-y-1 p-3 bg-gray-800 rounded-lg">
              <p>
                {t('hecate.form.selectAgent')}: <span className="font-medium text-white">{selectedAgent.name}</span>
              </p>
              <Link to="/hecate" className="text-blue-400 hover:text-blue-300 underline text-xs">
                {t('hecate.form.manageAgents', { count: agents.length })}
              </Link>
            </div>
          )}

          {/* Address source for Orthrus — how to reach this server via the tunnel */}
          {formData.connection_mode === 'agent' && formData.connection_type === 'orthrus' && formData.orthrus_agent_uuid && (
            <div className="space-y-3">
              <fieldset>
                <legend className="block text-sm font-medium text-gray-300 mb-2">
                  Address Source
                </legend>
                <div className="space-y-2">
                  {(['tailscale', 'netbird', 'zerotier', 'manual'] as const).map(mode => (
                    <label key={mode} className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="radio"
                        name="orthrus-ip-mode"
                        value={mode}
                        checked={formData.orthrus_ip_mode === mode}
                        onChange={() => setFormData(prev => ({
                          ...prev,
                          orthrus_ip_mode: mode,
                          selected_device_name: '',
                          selected_device_address: '',
                        }))}
                        className="w-4 h-4"
                      />
                      <span className="text-sm text-gray-300 capitalize">{mode === 'manual' ? 'Manual IP / Hostname' : mode.charAt(0).toUpperCase() + mode.slice(1)}</span>
                    </label>
                  ))}
                </div>
              </fieldset>

              {formData.orthrus_ip_mode === 'manual' && (
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2" htmlFor="orthrus-host">
                    Host (IP or Hostname)
                  </label>
                  <input
                    id="orthrus-host"
                    type="text"
                    required
                    value={formData.host}
                    onChange={e => setFormData({ ...formData, host: e.target.value })}
                    placeholder="192.168.1.100"
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                </div>
              )}

              {formData.orthrus_ip_mode === 'tailscale' && (
                <div className="space-y-2">
                  {formData.selected_device_name ? (
                    <div className="flex items-center gap-2 text-sm">
                      <span className="text-gray-300">Selected:</span>
                      <span className="font-medium text-white">{formData.selected_device_name}</span>
                      <span className="text-gray-500">{formData.selected_device_address}</span>
                      <button type="button" onClick={() => setShowTailscalePicker(true)} className="text-blue-400 hover:text-blue-300 underline text-xs focus:outline-none focus:ring-2 focus:ring-blue-500 rounded">
                        Change
                      </button>
                    </div>
                  ) : (
                    <button type="button" onClick={() => setShowTailscalePicker(true)} className="text-sm text-blue-400 hover:text-blue-300 underline focus:outline-none focus:ring-2 focus:ring-blue-500 rounded">
                      {t('hecate.form.mode.selectDevice')}
                    </button>
                  )}
                  <TailscaleDevicePicker
                    devices={tailscaleDevices}
                    open={showTailscalePicker}
                    onClose={() => setShowTailscalePicker(false)}
                    onSelect={(device: TailscaleDevice) => {
                      setFormData(prev => ({ ...prev, selected_device_name: device.hostname, selected_device_address: device.addresses[0] ?? '' }))
                      setShowTailscalePicker(false)
                    }}
                    selectedId={formData.selected_device_name}
                  />
                </div>
              )}

              {formData.orthrus_ip_mode === 'netbird' && (
                <div className="space-y-2">
                  {formData.selected_device_name ? (
                    <div className="flex items-center gap-2 text-sm">
                      <span className="text-gray-300">Selected:</span>
                      <span className="font-medium text-white">{formData.selected_device_name}</span>
                      <span className="text-gray-500">{formData.selected_device_address}</span>
                      <button type="button" onClick={() => setShowNetBirdPicker(true)} className="text-blue-400 hover:text-blue-300 underline text-xs focus:outline-none focus:ring-2 focus:ring-blue-500 rounded">
                        Change
                      </button>
                    </div>
                  ) : (
                    <button type="button" onClick={() => setShowNetBirdPicker(true)} className="text-sm text-blue-400 hover:text-blue-300 underline focus:outline-none focus:ring-2 focus:ring-blue-500 rounded">
                      {t('hecate.form.mode.selectPeer')}
                    </button>
                  )}
                  <NetBirdPeerPicker
                    open={showNetBirdPicker}
                    onClose={() => setShowNetBirdPicker(false)}
                    onSelect={(peer: NetBirdPeer) => {
                      setFormData(prev => ({ ...prev, selected_device_name: peer.name, selected_device_address: peer.ip }))
                      setShowNetBirdPicker(false)
                    }}
                    selectedId={formData.selected_device_name}
                  />
                </div>
              )}

              {formData.orthrus_ip_mode === 'zerotier' && (
                <div className="space-y-2">
                  {formData.selected_device_name ? (
                    <div className="flex items-center gap-2 text-sm">
                      <span className="text-gray-300">Selected:</span>
                      <span className="font-medium text-white">{formData.selected_device_name}</span>
                      <span className="text-gray-500">{formData.selected_device_address}</span>
                      <button type="button" onClick={() => setShowZeroTierPicker(true)} className="text-blue-400 hover:text-blue-300 underline text-xs focus:outline-none focus:ring-2 focus:ring-blue-500 rounded">
                        Change
                      </button>
                    </div>
                  ) : (
                    <button type="button" onClick={() => setShowZeroTierPicker(true)} className="text-sm text-blue-400 hover:text-blue-300 underline focus:outline-none focus:ring-2 focus:ring-blue-500 rounded">
                      {t('hecate.form.mode.selectMember')}
                    </button>
                  )}
                  <ZeroTierMemberPicker
                    open={showZeroTierPicker}
                    onClose={() => setShowZeroTierPicker(false)}
                    onSelect={(member: ZeroTierMember, _network: ZeroTierNetwork) => {
                      setFormData(prev => ({ ...prev, selected_device_name: member.name, selected_device_address: member.ip_assignments[0] ?? member.node_id }))
                      setShowZeroTierPicker(false)
                    }}
                    selectedId={formData.selected_device_name}
                  />
                </div>
              )}
            </div>
          )}

          {formData.connection_mode === 'agent' && formData.connection_type === 'cloudflare' && selectedTunnel && (
            <div className="text-sm text-gray-300 space-y-1 p-3 bg-gray-800 rounded-lg">
              <div className="flex items-center gap-2">
                <span>{selectedTunnel.name}</span>
                {tunnelStatus && <TunnelStatusBadge state={tunnelStatus.state} showLabel />}
              </div>
            </div>
          )}

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
            <button
              type="submit"
              disabled={loading}
              className="px-6 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              {loading ? 'Saving...' : (server ? 'Update' : 'Create')}
            </button>
          </div>
        </form>
        </div>
      </div>
    </>
  )
}
