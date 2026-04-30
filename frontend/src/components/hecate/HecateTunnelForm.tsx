import { Eye, EyeOff } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  type CreateTunnelRequest,
  type TunnelConfig,
  type TunnelProvider,
  type UpdateTunnelRequest,
} from '../../api/hecate'
import { useHecate } from '../../hooks/useHecate'
import { Button } from '../ui/Button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/Dialog'
import { Label } from '../ui/Label'

interface Props {
  tunnel?: TunnelConfig
  open: boolean
  onClose: () => void
}

type CloudflareFields = { apiToken: string; accountId: string; tunnelToken: string }
type TailscaleFields = { apiKey: string; tailnet: string }
type NetBirdFields = { accessToken: string; managementUrl: string }
type ZeroTierFields = { apiToken: string; controllerUrl: string }

type CredentialState = {
  cloudflare: CloudflareFields
  tailscale: TailscaleFields
  netbird: NetBirdFields
  zerotier: ZeroTierFields
}

const defaultCreds: CredentialState = {
  cloudflare: { apiToken: '', accountId: '', tunnelToken: '' },
  tailscale: { apiKey: '', tailnet: '' },
  netbird: { accessToken: '', managementUrl: '' },
  zerotier: { apiToken: '', controllerUrl: '' },
}

function buildCredentialsJSON(
  _provider: TunnelProvider,
  creds: CredentialState[TunnelProvider],
): string {
  const filtered = Object.fromEntries(
    Object.entries(creds as Record<string, string>).filter(([, v]) => v !== ''),
  )
  return JSON.stringify(filtered)
}

export function HecateTunnelForm({ tunnel, open, onClose }: Props) {
  const { t } = useTranslation()
  const { createTunnel, updateTunnel, isCreating, isUpdating } = useHecate()

  const isEdit = !!tunnel

  const [name, setName] = useState(tunnel?.name ?? '')
  const [provider, setProvider] = useState<TunnelProvider>(tunnel?.provider ?? 'cloudflare')
  const [isActive, setIsActive] = useState(tunnel?.is_active ?? true)
  const [creds, setCreds] = useState<CredentialState>({ ...defaultCreds })
  const [showFields, setShowFields] = useState<Record<string, boolean>>({})
  const [error, setError] = useState<string | null>(null)

  const toggleShow = (field: string) =>
    setShowFields(prev => ({ ...prev, [field]: !prev[field] }))

  const isSubmitting = isCreating || isUpdating

  const handleClose = () => {
    setError(null)
    onClose()
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      if (isEdit) {
        const credJson = buildCredentialsJSON(provider, creds[provider])
        const hasCredentials = credJson !== '{}'
        const req: UpdateTunnelRequest = {
          name,
          provider,
          is_active: isActive,
          ...(hasCredentials ? { credentials: credJson } : {}),
        }
        await updateTunnel({ uuid: tunnel.uuid, req })
      } else {
        const req: CreateTunnelRequest = {
          name,
          provider,
          credentials: buildCredentialsJSON(provider, creds[provider]),
          is_active: isActive,
        }
        await createTunnel(req)
      }
      handleClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save provider')
    }
  }

  const renderField = (
    field: string,
    label: string,
    help: string,
    required: boolean,
    value: string,
    onChange: (v: string) => void,
  ) => {
    const isVisible = !!showFields[field]
    const fieldId = `htf-${field}`
    const helpId = `${fieldId}-help`
    return (
      <div key={field}>
        <Label htmlFor={fieldId}>
          {label}
          {required && !isEdit && <span aria-hidden="true"> *</span>}
        </Label>
        <div className="relative mt-1">
          <input
            id={fieldId}
            type={isVisible ? 'text' : 'password'}
            value={value}
            onChange={e => onChange(e.target.value)}
            required={required && !isEdit}
            aria-required={required && !isEdit}
            aria-describedby={helpId}
            placeholder={isEdit ? t('hecate.page.credentials.editHint') : undefined}
            className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 pr-10 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button
            type="button"
            onClick={() => toggleShow(field)}
            className="absolute right-3 top-1/2 -translate-y-1/2 text-content-muted hover:text-content-primary"
            aria-label={
              isVisible
                ? t('hecate.page.credentials.hideField', { field: label })
                : t('hecate.page.credentials.showField', { field: label })
            }
          >
            {isVisible ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
          </button>
        </div>
        <p id={helpId} className="mt-1 text-xs text-content-muted">
          {help}
        </p>
      </div>
    )
  }

  const renderCredentialFields = () => {
    const c = t('hecate.page.credentials')
    switch (provider) {
      case 'cloudflare':
        return (
          <>
            {renderField(
              'cf-apiToken',
              t('hecate.page.credentials.cloudflare.apiToken'),
              t('hecate.page.credentials.cloudflare.apiTokenHelp'),
              true,
              creds.cloudflare.apiToken,
              v => setCreds(p => ({ ...p, cloudflare: { ...p.cloudflare, apiToken: v } })),
            )}
            {renderField(
              'cf-accountId',
              t('hecate.page.credentials.cloudflare.accountId'),
              t('hecate.page.credentials.cloudflare.accountIdHelp'),
              true,
              creds.cloudflare.accountId,
              v => setCreds(p => ({ ...p, cloudflare: { ...p.cloudflare, accountId: v } })),
            )}
            {renderField(
              'cf-tunnelToken',
              t('hecate.page.credentials.cloudflare.tunnelToken'),
              t('hecate.page.credentials.cloudflare.tunnelTokenHelp'),
              false,
              creds.cloudflare.tunnelToken,
              v => setCreds(p => ({ ...p, cloudflare: { ...p.cloudflare, tunnelToken: v } })),
            )}
          </>
        )
      case 'tailscale':
        return (
          <>
            {renderField(
              'ts-apiKey',
              t('hecate.page.credentials.tailscale.apiKey'),
              t('hecate.page.credentials.tailscale.apiKeyHelp'),
              true,
              creds.tailscale.apiKey,
              v => setCreds(p => ({ ...p, tailscale: { ...p.tailscale, apiKey: v } })),
            )}
            {renderField(
              'ts-tailnet',
              t('hecate.page.credentials.tailscale.tailnet'),
              t('hecate.page.credentials.tailscale.tailnetHelp'),
              true,
              creds.tailscale.tailnet,
              v => setCreds(p => ({ ...p, tailscale: { ...p.tailscale, tailnet: v } })),
            )}
          </>
        )
      case 'netbird':
        return (
          <>
            {renderField(
              'nb-accessToken',
              t('hecate.page.credentials.netbird.accessToken'),
              t('hecate.page.credentials.netbird.accessTokenHelp'),
              true,
              creds.netbird.accessToken,
              v => setCreds(p => ({ ...p, netbird: { ...p.netbird, accessToken: v } })),
            )}
            {renderField(
              'nb-managementUrl',
              t('hecate.page.credentials.netbird.managementUrl'),
              t('hecate.page.credentials.netbird.managementUrlHelp'),
              false,
              creds.netbird.managementUrl,
              v => setCreds(p => ({ ...p, netbird: { ...p.netbird, managementUrl: v } })),
            )}
          </>
        )
      case 'zerotier':
        return (
          <>
            {renderField(
              'zt-apiToken',
              t('hecate.page.credentials.zerotier.apiToken'),
              t('hecate.page.credentials.zerotier.apiTokenHelp'),
              true,
              creds.zerotier.apiToken,
              v => setCreds(p => ({ ...p, zerotier: { ...p.zerotier, apiToken: v } })),
            )}
            {renderField(
              'zt-controllerUrl',
              t('hecate.page.credentials.zerotier.controllerUrl'),
              t('hecate.page.credentials.zerotier.controllerUrlHelp'),
              false,
              creds.zerotier.controllerUrl,
              v => setCreds(p => ({ ...p, zerotier: { ...p.zerotier, controllerUrl: v } })),
            )}
          </>
        )
    }
    // Unreachable but satisfies exhaustiveness
    void c
  }

  return (
    <Dialog open={open} onOpenChange={isOpen => !isOpen && handleClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t('hecate.page.editProvider') : t('hecate.page.addProvider')}
          </DialogTitle>
        </DialogHeader>

        <form id="hecate-tunnel-form" onSubmit={handleSubmit} className="space-y-4 py-2">
          <div>
            <Label htmlFor="htf-name">
              {t('common.name')}
              <span aria-hidden="true"> *</span>
            </Label>
            <input
              id="htf-name"
              type="text"
              required
              aria-required
              value={name}
              onChange={e => setName(e.target.value)}
              className="w-full mt-1 bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div>
            <Label htmlFor="htf-provider">{t('hecate.page.columns.provider')}</Label>
            <select
              id="htf-provider"
              value={provider}
              onChange={e => setProvider(e.target.value as TunnelProvider)}
              disabled={isEdit}
              aria-disabled={isEdit}
              className="w-full mt-1 bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-60"
            >
              <option value="cloudflare">Cloudflare</option>
              <option value="tailscale">Tailscale</option>
              <option value="netbird">NetBird</option>
              <option value="zerotier">ZeroTier</option>
            </select>
          </div>

          {isEdit && (
            <p className="text-xs text-content-muted">{t('hecate.page.credentials.editHint')}</p>
          )}

          {renderCredentialFields()}

          <label className="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              checked={isActive}
              onChange={e => setIsActive(e.target.checked)}
              className="w-4 h-4 rounded"
            />
            <span className="text-sm text-content-primary">
              {t('hecate.page.columns.active')}
            </span>
          </label>

          {error && (
            <p role="alert" className="text-sm text-error">
              {error}
            </p>
          )}
        </form>

        <DialogFooter>
          <Button variant="secondary" onClick={handleClose} disabled={isSubmitting}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" form="hecate-tunnel-form" disabled={isSubmitting}>
            {isSubmitting
              ? t('common.loading')
              : isEdit
                ? t('common.update')
                : t('common.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
