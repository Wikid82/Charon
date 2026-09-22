import { useEffect, useState } from 'react'

import DNSProviderSelector from './DNSProviderSelector'
import {
  Alert,
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
} from './ui'
import type { Certificate } from '../api/certificates'
import {
  REDIRECT_STATUS_CODES,
  type RedirectionHost,
  type RedirectStatusCode,
} from '../api/redirectionHosts'
import { useCertificates } from '../hooks/useCertificates'

interface RedirectionHostFormProps {
  host?: RedirectionHost
  onSubmit: (data: Partial<RedirectionHost>) => Promise<void>
  onCancel: () => void
}

interface RedirectionHostFormState {
  name: string
  domain_names: string
  target_url: string
  status_code: RedirectStatusCode
  preserve_path: boolean
  ssl_forced: boolean
  http2_support: boolean
  hsts_enabled: boolean
  hsts_subdomains: boolean
  certificate_id: number | string | null
  dns_provider_id: string | null
  use_dns_challenge: boolean
}

function buildInitialFormData(host?: RedirectionHost): RedirectionHostFormState {
  return {
    name: host?.name ?? '',
    domain_names: host?.domain_names ?? '',
    target_url: host?.target_url ?? '',
    status_code: host?.status_code ?? 301,
    preserve_path: host?.preserve_path ?? true,
    ssl_forced: host?.ssl_forced ?? true,
    http2_support: host?.http2_support ?? true,
    hsts_enabled: host?.hsts_enabled ?? false,
    hsts_subdomains: host?.hsts_subdomains ?? false,
    certificate_id: host?.certificate_id ?? null,
    dns_provider_id:
      typeof host?.dns_provider_id === 'string' ? host.dns_provider_id : (host?.dns_provider?.uuid ?? null),
    use_dns_challenge: host?.use_dns_challenge ?? false,
  }
}

// Resolves a nullable certificate reference (numeric id or uuid) into a
// Radix Select-safe string token, and back again on change. Mirrors
// ProxyHostForm's resolveSelectToken/resolveTokenToFormValue/getEntityToken
// helpers. Duplicated locally rather than extracted to a shared util so this
// commit doesn't touch ProxyHostForm.tsx mid-feature (same reasoning the
// spec applies to the DomainNamesInput extraction candidate — see
// docs/plans/current_spec.md §4.5).
function resolveSelectToken(value: number | string | null | undefined): string {
  if (value === null || value === undefined) return 'none'
  if (typeof value === 'number') return `id:${value}`

  const trimmed = value.trim()
  if (trimmed === '') return 'none'
  if (trimmed.startsWith('id:') || trimmed.startsWith('uuid:')) return trimmed
  if (/^\d+$/.test(trimmed)) return `id:${Number.parseInt(trimmed, 10)}`
  return `uuid:${trimmed}`
}

function resolveTokenToFormValue(value: string): number | string | null {
  if (value === 'none') return null
  if (value.startsWith('id:')) {
    const parsed = Number.parseInt(value.slice(3), 10)
    return Number.isNaN(parsed) ? null : parsed
  }
  if (value.startsWith('uuid:')) return value.slice(5)
  if (/^\d+$/.test(value)) {
    const parsed = Number.parseInt(value, 10)
    return Number.isNaN(parsed) ? value : parsed
  }
  return value
}

function getEntityToken(entity: { id?: number; uuid?: string }): string | null {
  if (typeof entity.id === 'number' && Number.isFinite(entity.id)) return `id:${entity.id}`
  if (entity.uuid) return `uuid:${entity.uuid}`
  return null
}

export default function RedirectionHostForm({ host, onSubmit, onCancel }: RedirectionHostFormProps) {
  const [formData, setFormData] = useState<RedirectionHostFormState>(() => buildInitialFormData(host))
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const { certificates } = useCertificates()

  useEffect(() => {
    setFormData(buildInitialFormData(host))
    setError(null)
  }, [host])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const domainNames = formData.domain_names.trim()
    const targetUrl = formData.target_url.trim()

    if (!domainNames) {
      setError('Domain names is required')
      return
    }

    if (!targetUrl) {
      setError('Target URL is required')
      return
    }

    if (formData.use_dns_challenge && !formData.dns_provider_id) {
      setError('DNS provider is required when DNS challenge is enabled')
      return
    }

    setLoading(true)
    try {
      await onSubmit({
        name: formData.name.trim(),
        domain_names: domainNames,
        target_url: targetUrl,
        status_code: formData.status_code,
        preserve_path: formData.preserve_path,
        ssl_forced: formData.ssl_forced,
        http2_support: formData.http2_support,
        hsts_enabled: formData.hsts_enabled,
        hsts_subdomains: formData.hsts_subdomains,
        certificate_id: formData.certificate_id,
        dns_provider_id: formData.dns_provider_id,
        use_dns_challenge: formData.use_dns_challenge,
      })
      onCancel()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save redirection host')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open onOpenChange={(isOpen) => !isOpen && onCancel()}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{host ? 'Edit Redirection Host' : 'Add Redirection Host'}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-5">
          {error && <Alert variant="error">{error}</Alert>}

          <Input
            id="redirection-host-name"
            label="Name"
            value={formData.name}
            onChange={(e) => setFormData((prev) => ({ ...prev, name: e.target.value }))}
            placeholder="Old blog redirect"
            helperText="A friendly name to identify this redirection host"
          />

          <Input
            id="redirection-host-domain-names"
            label="Domain Names"
            aria-required="true"
            value={formData.domain_names}
            onChange={(e) => setFormData((prev) => ({ ...prev, domain_names: e.target.value }))}
            placeholder="example.com, www.example.com"
            helperText="Comma-separated list of domains that should redirect"
          />

          <Input
            id="redirection-host-target-url"
            label="Target URL"
            aria-required="true"
            value={formData.target_url}
            onChange={(e) => setFormData((prev) => ({ ...prev, target_url: e.target.value }))}
            placeholder="https://newsite.example.com"
            helperText="Where visitors will be sent"
          />

          <div>
            <Label htmlFor="redirection-host-status-code" className="block mb-1.5">
              Status Code
            </Label>
            <Select
              value={String(formData.status_code)}
              onValueChange={(value) =>
                setFormData((prev) => ({ ...prev, status_code: Number(value) as RedirectStatusCode }))
              }
            >
              <SelectTrigger id="redirection-host-status-code" aria-label="Status Code">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {REDIRECT_STATUS_CODES.map((option) => (
                  <SelectItem key={option.value} value={String(option.value)}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1">
            <div className="flex items-center gap-3">
              <Switch
                id="redirection-host-preserve-path"
                checked={formData.preserve_path}
                onCheckedChange={(checked) => setFormData((prev) => ({ ...prev, preserve_path: checked }))}
              />
              <Label htmlFor="redirection-host-preserve-path">Preserve Path</Label>
            </div>
            <p className="text-xs text-content-muted">
              When enabled, the original path and query string are appended to the target URL.
            </p>
          </div>

          <div className="space-y-3">
            <div className="flex items-center gap-3">
              <Switch
                id="redirection-host-ssl-forced"
                checked={formData.ssl_forced}
                onCheckedChange={(checked) => setFormData((prev) => ({ ...prev, ssl_forced: checked }))}
              />
              <Label htmlFor="redirection-host-ssl-forced">Force SSL</Label>
            </div>
            <div className="flex items-center gap-3">
              <Switch
                id="redirection-host-http2"
                checked={formData.http2_support}
                onCheckedChange={(checked) => setFormData((prev) => ({ ...prev, http2_support: checked }))}
              />
              <Label htmlFor="redirection-host-http2">HTTP/2 Support</Label>
            </div>
            <div className="flex items-center gap-3">
              <Switch
                id="redirection-host-hsts"
                checked={formData.hsts_enabled}
                onCheckedChange={(checked) => setFormData((prev) => ({ ...prev, hsts_enabled: checked }))}
              />
              <Label htmlFor="redirection-host-hsts">HSTS Enabled</Label>
            </div>
            <div className="flex items-center gap-3">
              <Switch
                id="redirection-host-hsts-subdomains"
                checked={formData.hsts_subdomains}
                onCheckedChange={(checked) => setFormData((prev) => ({ ...prev, hsts_subdomains: checked }))}
              />
              <Label htmlFor="redirection-host-hsts-subdomains">HSTS Subdomains</Label>
            </div>
          </div>

          <div>
            <Label htmlFor="redirection-host-certificate" className="block mb-1.5">
              Certificate
            </Label>
            <Select
              value={resolveSelectToken(formData.certificate_id)}
              onValueChange={(token) =>
                setFormData((prev) => ({ ...prev, certificate_id: resolveTokenToFormValue(token) }))
              }
            >
              <SelectTrigger id="redirection-host-certificate" aria-label="Certificate">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">Auto-manage with Let&apos;s Encrypt (recommended)</SelectItem>
                {certificates.map((cert: Certificate) => {
                  const token = getEntityToken(cert)
                  if (!token) return null
                  return (
                    <SelectItem key={token} value={token}>
                      {cert.name || cert.domains}
                      {cert.provider ? ` (${cert.provider})` : ''}
                    </SelectItem>
                  )
                })}
              </SelectContent>
            </Select>
          </div>

          <div className="flex items-center gap-3">
            <Switch
              id="redirection-host-use-dns-challenge"
              checked={formData.use_dns_challenge}
              onCheckedChange={(checked) =>
                setFormData((prev) => ({
                  ...prev,
                  use_dns_challenge: checked,
                  dns_provider_id: checked ? prev.dns_provider_id : null,
                }))
              }
            />
            <Label htmlFor="redirection-host-use-dns-challenge">Use DNS Challenge</Label>
          </div>

          {formData.use_dns_challenge && (
            <DNSProviderSelector
              label="DNS Provider"
              required
              value={formData.dns_provider_id ?? undefined}
              onChange={(id) => setFormData((prev) => ({ ...prev, dns_provider_id: id ?? null }))}
            />
          )}

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onCancel} disabled={loading}>
              Cancel
            </Button>
            <Button type="submit" isLoading={loading}>
              Save
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
