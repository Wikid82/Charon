import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { type TunnelProvider } from '../api/hecate'
import { HecateTunnelForm } from '../components/hecate/HecateTunnelForm'
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
  const { tunnels } = useHecate()

  const [formOpen, setFormOpen] = useState(false)
  const [selectedProvider, setSelectedProvider] = useState<TunnelProvider>('cloudflare')

  const openFormForProvider = (provider: TunnelProvider) => {
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
          const count = tunnels.filter(tun => tun.provider === p.id).length
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
              <Button
                variant="secondary"
                size="sm"
                onClick={() => openFormForProvider(p.id)}
                aria-label={`New ${p.label} tunnel`}
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
    </PageShell>
  )
}
