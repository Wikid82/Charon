import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import { useHecate } from '../../hooks/useHecate'
import { useAgentList } from '../../hooks/useOrthrus'
import { ProviderDevicePicker } from './ProviderDevicePicker'

export type ConnectionMode = 'direct' | 'agent' | 'provider'
export type ConnectionType = 'direct' | 'orthrus' | 'cloudflare' | 'tailscale' | 'netbird' | 'zerotier'
export type HecateProvider = 'cloudflare' | 'tailscale' | 'netbird' | 'zerotier'

export interface ConnectionTypeSelectorProps {
  mode: ConnectionMode
  onModeChange: (mode: ConnectionMode) => void
  selectedTunnelUUID: string | null
  selectedAgentUUID: string | null
  selectedDeviceId: string
  onTunnelSelect: (tunnelUUID: string, provider: HecateProvider) => void
  onAgentSelect: (agentUUID: string) => void
  onDeviceSelect: (deviceId: string, address: string) => void
  disabled?: boolean
}

export function ConnectionTypeSelector({
  mode,
  onModeChange,
  selectedTunnelUUID,
  selectedAgentUUID,
  selectedDeviceId,
  onTunnelSelect,
  onAgentSelect,
  onDeviceSelect,
  disabled,
}: ConnectionTypeSelectorProps) {
  const { t } = useTranslation()
  const { tunnels } = useHecate()
  const { data: agents = [] } = useAgentList()
  const warningId = useId()

  const selectedAgent = agents.find(a => a.uuid === selectedAgentUUID)
  const agentHasNoProvider = Boolean(selectedAgentUUID && !selectedAgent?.resolved_address)

  return (
    <div className="space-y-3">
      <fieldset disabled={disabled}>
        <legend className="text-sm font-medium text-content-primary mb-2">
          {t('hecate.form.mode.label')}
        </legend>
        <div className="flex flex-wrap gap-4">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="radio"
              name="connection-mode"
              value="direct"
              checked={mode === 'direct'}
              onChange={() => onModeChange('direct')}
              className="w-4 h-4"
            />
            <span className="text-sm text-content-primary">{t('hecate.form.mode.direct')}</span>
            <span className="text-xs text-content-muted">
              {' — '}
              {t('hecate.form.mode.directDescription')}
            </span>
          </label>

          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="radio"
              name="connection-mode"
              value="agent"
              checked={mode === 'agent'}
              onChange={() => onModeChange('agent')}
              className="w-4 h-4"
            />
            <span className="text-sm text-content-primary">{t('hecate.form.mode.agent.label')}</span>
            <span className="text-xs text-content-muted">
              {' — '}
              {t('hecate.form.mode.agentDescription')}
            </span>
          </label>

          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="radio"
              name="connection-mode"
              value="provider"
              checked={mode === 'provider'}
              onChange={() => onModeChange('provider')}
              className="w-4 h-4"
            />
            <span className="text-sm text-content-primary">{t('hecate.form.mode.provider')}</span>
            <span className="text-xs text-content-muted">
              {' — '}
              {t('hecate.form.mode.providerDescription')}
            </span>
          </label>
        </div>
      </fieldset>

      {mode === 'agent' && (
        <div className="space-y-2">
          <label
            htmlFor="cts-agent"
            className="block text-sm font-medium text-content-primary"
          >
            {t('hecate.form.mode.selectAgent')}
          </label>
          <select
            id="cts-agent"
            value={selectedAgentUUID ?? ''}
            onChange={e => {
              const uuid = e.target.value
              if (uuid) onAgentSelect(uuid)
            }}
            disabled={disabled}
            aria-describedby={agentHasNoProvider ? warningId : undefined}
            className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-60"
          >
            <option value="">{t('hecate.form.mode.selectAgent')}</option>
            {agents.map(agent => (
              <option key={agent.uuid} value={agent.uuid}>
                {agent.name}
                {!agent.resolved_address ? ` (${t('hecate.agentManager.noProviderAssigned')})` : ''}
              </option>
            ))}
          </select>
          {agentHasNoProvider && (
            <p id={warningId} role="alert" className="text-sm text-amber-400">
              {t('hecate.form.mode.agent.noProviderWarning')}{' '}
              <Link to="/hecate/agent" className="underline hover:text-amber-300">
                {t('hecate.form.mode.agent.noProviderLink')}
              </Link>
            </p>
          )}
        </div>
      )}

      {mode === 'provider' && (
        <ProviderDevicePicker
          selectedTunnelUUID={selectedTunnelUUID}
          selectedDeviceId={selectedDeviceId}
          tunnels={tunnels}
          onTunnelSelect={(uuid, provider) => onTunnelSelect(uuid, provider as HecateProvider)}
          onDeviceSelect={onDeviceSelect}
          disabled={disabled}
        />
      )}
    </div>
  )
}

