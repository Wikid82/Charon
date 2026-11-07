import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  listTailscaleDevices,
  type NetBirdPeer,
  type TailscaleDevice,
  type TunnelConfig,
  type TunnelProvider,
  type ZeroTierMember,
  type ZeroTierNetwork,
} from '../../api/hecate'
import { NetBirdPeerPicker } from './NetBirdPeerPicker'
import { TailscaleDevicePicker } from './TailscaleDevicePicker'
import { ZeroTierMemberPicker } from './ZeroTierMemberPicker'

export interface ProviderDevicePickerProps {
  selectedTunnelUUID: string | null
  selectedDeviceId: string
  tunnels: TunnelConfig[]
  onTunnelSelect: (tunnelUUID: string, provider: TunnelProvider) => void
  onDeviceSelect: (deviceId: string, address: string) => void
  disabled?: boolean
}

export function ProviderDevicePicker({
  selectedTunnelUUID,
  selectedDeviceId,
  tunnels,
  onTunnelSelect,
  onDeviceSelect,
  disabled,
}: ProviderDevicePickerProps) {
  const { t } = useTranslation()
  const [pickerOpen, setPickerOpen] = useState(false)
  const [cfHostname, setCfHostname] = useState('')

  const selectedTunnel = tunnels.find(tn => tn.uuid === selectedTunnelUUID)
  const isCloudflare = selectedTunnel?.provider === 'cloudflare'
  const isTailscale = selectedTunnel?.provider === 'tailscale'
  const isNetBird = selectedTunnel?.provider === 'netbird'
  const isZeroTier = selectedTunnel?.provider === 'zerotier'

  const { data: tailscaleDevices = [] } = useQuery({
    queryKey: ['hecate', 'tailscale', 'devices'],
    queryFn: listTailscaleDevices,
    enabled: isTailscale,
    staleTime: 60_000,
  })

  const handleTunnelChange = (uuid: string) => {
    setCfHostname('')
    setPickerOpen(false)
    if (!uuid) {
      onTunnelSelect('', '' as TunnelProvider)
      return
    }
    const tunnel = tunnels.find(tn => tn.uuid === uuid)
    if (tunnel) {
      onTunnelSelect(tunnel.uuid, tunnel.provider)
    }
  }

  const handleCfHostnameChange = (hostname: string) => {
    setCfHostname(hostname)
    onDeviceSelect('', hostname)
  }

  return (
    <div className="space-y-3">
      <div>
        <label
          htmlFor="pdp-tunnel"
          className="block text-sm font-medium text-content-primary mb-1"
        >
          {t('hecate.form.mode.selectProvider')}
        </label>
        {tunnels.length > 0 ? (
          <select
            id="pdp-tunnel"
            value={selectedTunnelUUID ?? ''}
            onChange={e => handleTunnelChange(e.target.value)}
            disabled={disabled}
            className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-60"
          >
            <option value="">{t('hecate.form.mode.selectProvider')}</option>
            {(['cloudflare', 'tailscale', 'netbird', 'zerotier'] as TunnelProvider[]).map(provider => {
              const pts = tunnels.filter(tn => tn.provider === provider)
              if (pts.length === 0) return null
              const label = provider.charAt(0).toUpperCase() + provider.slice(1)
              return (
                <optgroup key={provider} label={label}>
                  {pts.map(tn => (
                    <option key={tn.uuid} value={tn.uuid}>{tn.name}</option>
                  ))}
                </optgroup>
              )
            })}
          </select>
        ) : (
          <p className="text-sm text-content-muted">{t('hecate.form.mode.noProviders')}</p>
        )}
      </div>

      {isCloudflare && selectedTunnelUUID && (
        <div>
          <label
            htmlFor="cf-hostname"
            className="block text-sm font-medium text-content-primary mb-1"
          >
            {t('hecate.form.provider.cloudflareTunnelHostname')}
          </label>
          <input
            id="cf-hostname"
            type="text"
            value={cfHostname}
            onChange={e => handleCfHostnameChange(e.target.value)}
            disabled={disabled}
            aria-describedby="cf-hostname-hint"
            placeholder="app.example.com"
            className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-60"
          />
          <p id="cf-hostname-hint" className="text-xs text-content-muted mt-1">
            {t('hecate.form.provider.cloudflareTunnelHint')}
          </p>
        </div>
      )}

      {(isTailscale || isNetBird || isZeroTier) && selectedTunnelUUID && (
        <div>
          {selectedDeviceId ? (
            <div className="flex items-center gap-2 text-sm">
              <span className="text-content-muted font-medium">{selectedDeviceId}</span>
              <button
                type="button"
                onClick={() => setPickerOpen(true)}
                disabled={disabled}
                className="text-blue-400 hover:text-blue-300 underline text-xs focus:outline-none focus:ring-2 focus:ring-blue-500 rounded"
              >
                {t('hecate.form.mode.changeDevice')}
              </button>
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setPickerOpen(true)}
              disabled={disabled}
              className="text-sm text-blue-400 hover:text-blue-300 underline focus:outline-none focus:ring-2 focus:ring-blue-500 rounded"
            >
              {isTailscale
                ? t('hecate.form.mode.selectDevice')
                : isNetBird
                  ? t('hecate.form.mode.selectPeer')
                  : t('hecate.form.mode.selectMember')}
            </button>
          )}

          {isTailscale && (
            <TailscaleDevicePicker
              devices={tailscaleDevices}
              open={pickerOpen}
              onClose={() => setPickerOpen(false)}
              onSelect={(device: TailscaleDevice) => {
                onDeviceSelect(device.id, device.addresses[0] ?? '')
                setPickerOpen(false)
              }}
              selectedId={selectedDeviceId}
            />
          )}

          {isNetBird && (
            <NetBirdPeerPicker
              open={pickerOpen}
              onClose={() => setPickerOpen(false)}
              onSelect={(peer: NetBirdPeer) => {
                onDeviceSelect(peer.id, peer.ip)
                setPickerOpen(false)
              }}
              selectedId={selectedDeviceId}
            />
          )}

          {isZeroTier && (
            <ZeroTierMemberPicker
              open={pickerOpen}
              onClose={() => setPickerOpen(false)}
              onSelect={(member: ZeroTierMember, _network: ZeroTierNetwork) => {
                onDeviceSelect(member.node_id, member.ip_assignments[0] ?? '')
                setPickerOpen(false)
              }}
              selectedId={selectedDeviceId}
            />
          )}
        </div>
      )}
    </div>
  )
}
