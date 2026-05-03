import { Plus, Settings } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { type TunnelConfig, type TunnelProvider } from '../api/hecate'
import { HecateTunnelForm } from '../components/hecate/HecateTunnelForm'
import { TunnelStatusBadge } from '../components/hecate/TunnelStatusBadge'
import { PageShell } from '../components/layout/PageShell'
import { Button } from '../components/ui'
import { useHecate } from '../hooks/useHecate'

const PROVIDERS: { id: TunnelProvider; label: string; icon: string }[] = [
  { id: 'cloudflare', label: 'Cloudflare',  icon: '☁️' },
  { id: 'tailscale', label: 'Tailscale',   icon: '🔐' },
  { id: 'netbird',   label: 'NetBird',     icon: '🐦' },
  { id: 'zerotier',  label: 'ZeroTier',    icon: '🔵' },
]

export default function HecateProviders() {
  const { t } = useTranslation()
  const { tunnels, getStatus } = useHecate()

  const [formOpen, setFormOpen] = useState(false)
  const [selectedProvider, setSelectedProvider] = useState<TunnelProvider>('cloudflare')
  const [editTunnel, setEditTunnel] = useState<TunnelConfig | null>(null)
  const [editFormOpen, setEditFormOpen] = useState(false)

  const openEdit = (tunnel: TunnelConfig) => {
    setEditTunnel(tunnel)
    setEditFormOpen(true)
  }

  const openCreate = (provider: TunnelProvider) => {
    setEditTunnel(null)
    setSelectedProvider(provider)
    setFormOpen(true)
  }

  return (
    <PageShell
      title={t('hecate.providers.title')}
      description={t('hecate.providers.description')}
    >
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
        {PROVIDERS.map(p => {
          const providerTunnels = tunnels.filter(tun => tun.provider === p.id)
          const count = providerTunnels.length
          return (
            <div
              key={p.id}
              className="rounded-xl border border-border bg-surface p-6 flex flex-col gap-4"
            >
              <div className="flex items-center gap-3">
                <span className="text-3xl" aria-hidden="true">{p.icon}</span>
                <div>
                  <h2 className="text-base font-semibold text-content-primary">{p.label}</h2>
                  <p className="text-sm text-content-secondary">
                    {count === 1
                      ? t('hecate.providers.tunnelCount_one', { count, defaultValue: '{{count}} tunnel' })
                      : t('hecate.providers.tunnelCount_other', { count, defaultValue: '{{count}} tunnels' })}
                  </p>
                </div>
              </div>

              <ul aria-label={t('hecate.providers.tunnelList')}>
                {providerTunnels.map(tun => (
                  <li
                    key={tun.uuid}
                    className="flex items-center justify-between text-sm py-1 px-2 rounded bg-surface-subtle"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="font-medium text-content-primary truncate">{tun.name}</span>
                      <TunnelStatusBadge state={getStatus(tun.uuid)?.state ?? 'stopped'} />
                    </div>
                    <button
                      type="button"
                      className="p-1 rounded text-content-tertiary hover:text-brand-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
                      aria-label={t('hecate.providers.editTunnel', { name: tun.name })}
                      onClick={() => openEdit(tun)}
                    >
                      <Settings className="w-4 h-4" aria-hidden="true" />
                    </button>
                  </li>
                ))}
                {providerTunnels.length === 0 && (
                  <li className="text-xs text-content-muted py-1" aria-live="polite">
                    {t('hecate.providers.noTunnels')}
                  </li>
                )}
              </ul>

              <Button
                variant="secondary"
                size="sm"
                onClick={() => openCreate(p.id)}
                aria-label={t('hecate.providers.addTunnel', { provider: p.label })}
              >
                <Plus className="w-4 h-4 mr-2" aria-hidden="true" />
                {t('hecate.page.addProvider')}
              </Button>
            </div>
          )
        })}
      </div>

      <HecateTunnelForm
        initialProvider={selectedProvider}
        open={formOpen}
        onClose={() => setFormOpen(false)}
      />

      <HecateTunnelForm
        tunnel={editTunnel ?? undefined}
        open={editFormOpen}
        onClose={() => {
          setEditFormOpen(false)
          setEditTunnel(null)
        }}
      />
    </PageShell>
  )
}
