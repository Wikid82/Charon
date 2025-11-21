import { useCertificates } from '../hooks/useCertificates'
import { LoadingSpinner } from './LoadingStates'

export default function CertificateList() {
  const { certificates, isLoading, error } = useCertificates()

  if (isLoading) return <LoadingSpinner />
  if (error) return <div className="text-red-500">Failed to load certificates</div>

  return (
    <div className="bg-dark-card rounded-lg border border-gray-800 overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm text-gray-400">
          <thead className="bg-gray-900 text-gray-200 uppercase font-medium">
            <tr>
              <th className="px-6 py-3">Domain</th>
              <th className="px-6 py-3">Issuer</th>
              <th className="px-6 py-3">Expires</th>
              <th className="px-6 py-3">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {certificates.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-6 py-8 text-center text-gray-500">
                  No certificates found.
                </td>
              </tr>
            ) : (
              certificates.map((cert) => (
                <tr key={cert.domain} className="hover:bg-gray-800/50 transition-colors">
                  <td className="px-6 py-4 font-medium text-white">{cert.domain}</td>
                  <td className="px-6 py-4">{cert.issuer}</td>
                  <td className="px-6 py-4">
                    {new Date(cert.expires_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4">
                    <StatusBadge status={cert.status} />
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  const styles = {
    valid: 'bg-green-900/30 text-green-400 border-green-800',
    expiring: 'bg-yellow-900/30 text-yellow-400 border-yellow-800',
    expired: 'bg-red-900/30 text-red-400 border-red-800',
  }

  const labels = {
    valid: 'Valid',
    expiring: 'Expiring Soon',
    expired: 'Expired',
  }

  const style = styles[status as keyof typeof styles] || styles.valid
  const label = labels[status as keyof typeof labels] || status

  return (
    <span className={`px-2.5 py-0.5 rounded-full text-xs font-medium border ${style}`}>
      {label}
    </span>
  )
}
