import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import { useHecate } from '../../hooks/useHecate'
import { useAgentList } from '../../hooks/useOrthrus'

export type ConnectionMode = 'direct' | 'agent'
export type ConnectionType = 'direct' | 'orthrus' | 'cloudflare' | 'tailscale' | 'netbird' | 'zerotier'
export type HecateProvider = Exclude<ConnectionType, 'direct'>

export interface ConnectionTypeSelectorProps {
  mode: ConnectionMode
  onModeChange: (mode: ConnectionMode) => void
  selectedTunnelUUID: string | null
  selectedAgentUUID: string | null
  onTunnelSelect: (tunnelUUID: string, provider: HecateProvider) => void
  onAgentSelect: (agentUUID: string) => void
  disabled?: boolean
}

export function ConnectionTypeSelector({
  mode,
  onModeChange,
  selectedTunnelUUID,
  selectedAgentUUID,
  onTunnelSelect,
  onAgentSelect,
  disabled,
}: ConnectionTypeSelectorProps) {
  const { t } = useTranslation()
  const { tunnels } = useHecate()
  const { data: agents = [] } = useAgentList()

  const hasProviders = tunnels.length > 0 || agents.length > 0

  const handleModeChange = (newMode: ConnectionMode) => {
    if (!disabled) onModeChange(newMode)
  }

  const currentSelectValue = () => {
    if (mode !== 'agent') return ''
    if (selectedAgentUUID) return `orthrus:${selectedAgentUUID}`
    if (selectedTunnelUUID) return selectedTunnelUUID
    return ''
  }

  const handleProviderChange = (value: string) => {
    if (!value) return
    if (value.startsWith('orthrus:')) {
      onAgentSelect(value.slice('orthrus:'.length))
    } else {
      const tunnel = tunnels.find(tn => tn.uuid === value)
      if (tunnel) {
        onTunnelSelect(tunnel.uuid, tunnel.provider as HecateProvider)
      }
    }
  }

  return (
    <div className="space-y-3">
      {/* Tier 1: Mode selection */}
      <fieldset disabled={disabled}>
        <legend className="text-sm font-medium text-content-primary mb-2">
          {t('hecate.form.mode.label')}
        </legend>
        <div className="flex gap-4">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="radio"
              name="connection-mode"
              value="direct"
              checked={mode === 'direct'}
              onChange={() => handleModeChange('direct')}
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
              onChange={() => handleModeChange('agent')}
              className="w-4 h-4"
            />
            <span className="text-sm text-content-primary">{t('hecate.form.mode.agent')}</span>
            <span className="text-xs text-content-muted">
              {' — '}
              {t('hecate.form.mode.agentDescription')}
            </span>
          </label>
        </div>
      </fieldset>

      {/* Tier 2: Provider selection (only when agent mode) */}
      {mode === 'agent' && (
        <div>
          <label
            htmlFor="cts-provider"
            className="block text-sm font-medium text-content-primary mb-1"
          >
            {t('hecate.form.mode.provider')}
          </label>

          {hasProviders ? (
            <select
              id="cts-provider"
              value={currentSelectValue()}
              onChange={e => handleProviderChange(e.target.value)}
              disabled={disabled}
              aria-label={t('hecate.form.mode.provider')}
              className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-60"
            >
              <option value="">{t('hecate.form.mode.selectProvider')}</option>

              {['cloudflare', 'tailscale', 'netbird', 'zerotier'].map(provider => {
                const providerTunnels = tunnels.filter(t => t.provider === provider)
                if (providerTunnels.length === 0) return null
                const label = provider.charAt(0).toUpperCase() + provider.slice(1)
                return (
                  <optgroup key={provider} label={label}>
                    {providerTunnels.map(t => (
                      <option key={t.uuid} value={t.uuid}>{t.name}</option>
                    ))}
                  </optgroup>
                )
              })}

              {agents.length > 0 && (
                <optgroup label="Orthrus Agents">
                  {agents.map(agent => (
                    <option key={agent.uuid} value={`orthrus:${agent.uuid}`}>
                      {agent.name}
                    </option>
                  ))}
                </optgroup>
              )}
            </select>
          ) : (
            <p aria-live="polite" className="text-sm text-content-muted">
              {t('hecate.form.mode.noProviders')}{' '}
              <Link to="/hecate" className="underline text-blue-500 hover:text-blue-400">
                {t('hecate.form.mode.goToHecate')}
              </Link>
            </p>
          )}
        </div>
      )}
    </div>
  )
}

