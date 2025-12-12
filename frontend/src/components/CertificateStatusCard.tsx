import { Link } from 'react-router-dom'
import { Loader2 } from 'lucide-react'
import type { Certificate } from '../api/certificates'
import type { ProxyHost } from '../api/proxyHosts'

interface CertificateStatusCardProps {
  certificates: Certificate[]
  hosts: ProxyHost[]
}

export default function CertificateStatusCard({ certificates, hosts }: CertificateStatusCardProps) {
  const validCount = certificates.filter(c => c.status === 'valid').length
  const expiringCount = certificates.filter(c => c.status === 'expiring').length
  const untrustedCount = certificates.filter(c => c.status === 'untrusted').length

  // Pending = hosts with ssl_forced and enabled but no certificate_id
  const sslHosts = hosts.filter(h => h.ssl_forced && h.enabled)
  const hostsWithCerts = sslHosts.filter(h => h.certificate_id != null)
  const pendingCount = sslHosts.length - hostsWithCerts.length

  const hasProvisioning = pendingCount > 0
  const progressPercent = sslHosts.length > 0
    ? Math.round((hostsWithCerts.length / sslHosts.length) * 100)
    : 100

  return (
    <Link
      to="/certificates"
      className="bg-dark-card p-6 rounded-lg border border-gray-800 hover:border-gray-700 transition-colors block"
    >
      <div className="text-sm text-gray-400 mb-2">SSL Certificates</div>
      <div className="text-3xl font-bold text-white mb-1">{certificates.length}</div>

      {/* Status breakdown */}
      <div className="flex flex-wrap gap-x-3 gap-y-1 mt-2 text-xs">
        <span className="text-green-400">{validCount} valid</span>
        {expiringCount > 0 && <span className="text-yellow-400">{expiringCount} expiring</span>}
        {untrustedCount > 0 && <span className="text-orange-400">{untrustedCount} staging</span>}
      </div>

      {/* Pending indicator */}
      {hasProvisioning && (
        <div className="mt-3 pt-3 border-t border-gray-700">
          <div className="flex items-center gap-2 text-blue-400 text-xs">
            <Loader2 className="h-3 w-3 animate-spin" />
            <span>{pendingCount} host{pendingCount !== 1 ? 's' : ''} awaiting certificate</span>
          </div>
          <div className="mt-2 h-1.5 bg-gray-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-blue-500 transition-all duration-500 rounded-full"
              style={{ width: `${progressPercent}%` }}
            />
          </div>
          <div className="text-xs text-gray-500 mt-1">{progressPercent}% provisioned</div>
        </div>
      )}
    </Link>
  )
}
