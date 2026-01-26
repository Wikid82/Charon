import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { FileKey, Loader2 } from 'lucide-react'
import { Card, CardHeader, CardContent, Badge, Skeleton, Progress } from './ui'
import type { Certificate } from '../api/certificates'
import type { ProxyHost } from '../api/proxyHosts'

interface CertificateStatusCardProps {
  certificates: Certificate[]
  hosts: ProxyHost[]
  isLoading?: boolean
}

export default function CertificateStatusCard({ certificates, hosts, isLoading }: CertificateStatusCardProps) {
  const validCount = certificates.filter(c => c.status === 'valid').length
  const expiringCount = certificates.filter(c => c.status === 'expiring').length
  const untrustedCount = certificates.filter(c => c.status === 'untrusted').length

  // Build a set of all domains that have certificates (case-insensitive)
  // ACME certificates (Let's Encrypt) are auto-managed and don't set certificate_id,
  // so we match by domain name instead
  const certifiedDomains = useMemo(() => {
    const domains = new Set<string>()
    certificates.forEach(cert => {
      // Handle missing or undefined domain field
      if (!cert.domain) return
      // Certificate domain field can be comma-separated
      cert.domain.split(',').forEach(d => {
        const trimmed = d.trim().toLowerCase()
        if (trimmed) domains.add(trimmed)
      })
    })
    return domains
  }, [certificates])

  // Calculate pending hosts: SSL-enabled hosts without any domain covered by a certificate
  const { pendingCount, totalSSLHosts, hostsWithCerts } = useMemo(() => {
    const sslHosts = hosts.filter(h => h.ssl_forced && h.enabled)

    let withCerts = 0
    sslHosts.forEach(host => {
      // Check if any of the host's domains have a certificate
      const hostDomains = host.domain_names.split(',').map(d => d.trim().toLowerCase())
      if (hostDomains.some(domain => certifiedDomains.has(domain))) {
        withCerts++
      }
    })

    return {
      pendingCount: sslHosts.length - withCerts,
      totalSSLHosts: sslHosts.length,
      hostsWithCerts: withCerts,
    }
  }, [hosts, certifiedDomains])

  const hasProvisioning = pendingCount > 0
  const progressPercent = totalSSLHosts > 0
    ? Math.round((hostsWithCerts / totalSSLHosts) * 100)
    : 100

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="pb-2">
          <div className="flex items-center gap-2">
            <Skeleton className="h-5 w-5 rounded" />
            <Skeleton className="h-4 w-28" />
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-8 w-16" />
          <div className="flex gap-2">
            <Skeleton className="h-5 w-16 rounded-md" />
            <Skeleton className="h-5 w-20 rounded-md" />
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Link to="/certificates" className="block group">
      <Card variant="interactive" className="h-full">
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="rounded-lg bg-brand-500/10 p-2 text-brand-500">
                <FileKey className="h-5 w-5" />
              </div>
              <span className="text-sm font-medium text-content-secondary">SSL Certificates</span>
            </div>
            {hasProvisioning && (
              <Badge variant="primary" size="sm" className="animate-pulse">
                Provisioning
              </Badge>
            )}
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="text-3xl font-bold text-content-primary tabular-nums">
            {certificates.length}
          </div>

          {/* Status breakdown */}
          <div className="flex flex-wrap gap-2">
            {validCount > 0 && (
              <Badge variant="success" size="sm">
                {validCount} valid
              </Badge>
            )}
            {expiringCount > 0 && (
              <Badge variant="warning" size="sm">
                {expiringCount} expiring
              </Badge>
            )}
            {untrustedCount > 0 && (
              <Badge variant="outline" size="sm">
                {untrustedCount} staging
              </Badge>
            )}
            {certificates.length === 0 && (
              <Badge variant="outline" size="sm">
                No certificates
              </Badge>
            )}
          </div>

          {/* Pending indicator */}
          {hasProvisioning && (
            <div className="pt-3 border-t border-border space-y-2">
              <div className="flex items-center gap-2 text-brand-400 text-sm">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span>{pendingCount} host{pendingCount !== 1 ? 's' : ''} awaiting certificate</span>
              </div>
              <Progress value={progressPercent} variant="default" />
              <div className="text-xs text-content-muted">{progressPercent}% provisioned</div>
            </div>
          )}
        </CardContent>
      </Card>
    </Link>
  )
}
