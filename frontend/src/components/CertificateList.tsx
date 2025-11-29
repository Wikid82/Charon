import { useState, useMemo } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2, ChevronUp, ChevronDown } from 'lucide-react'
import { useCertificates } from '../hooks/useCertificates'
import { deleteCertificate } from '../api/certificates'
import { LoadingSpinner } from './LoadingStates'
import { toast } from '../utils/toast'

type SortColumn = 'name' | 'expires'
type SortDirection = 'asc' | 'desc'

export default function CertificateList() {
  const { certificates, isLoading, error } = useCertificates()
  const queryClient = useQueryClient()
  const [sortColumn, setSortColumn] = useState<SortColumn>('name')
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc')

  const deleteMutation = useMutation({
    mutationFn: deleteCertificate,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['certificates'] })
      toast.success('Certificate deleted')
    },
    onError: (error: Error) => {
      toast.error(`Failed to delete certificate: ${error.message}`)
    },
  })

  const sortedCertificates = useMemo(() => {
    return [...certificates].sort((a, b) => {
      let comparison = 0

      switch (sortColumn) {
        case 'name': {
          const aName = (a.name || a.domain || '').toLowerCase()
          const bName = (b.name || b.domain || '').toLowerCase()
          comparison = aName.localeCompare(bName)
          break
        }
        case 'expires': {
          const aDate = new Date(a.expires_at).getTime()
          const bDate = new Date(b.expires_at).getTime()
          comparison = aDate - bDate
          break
        }
      }

      return sortDirection === 'asc' ? comparison : -comparison
    })
  }, [certificates, sortColumn, sortDirection])

  const handleSort = (column: SortColumn) => {
    if (sortColumn === column) {
      setSortDirection(prev => prev === 'asc' ? 'desc' : 'asc')
    } else {
      setSortColumn(column)
      setSortDirection('asc')
    }
  }

  const SortIcon = ({ column }: { column: SortColumn }) => {
    if (sortColumn !== column) return null
    return sortDirection === 'asc' ? <ChevronUp size={14} /> : <ChevronDown size={14} />
  }

  if (isLoading) return <LoadingSpinner />
  if (error) return <div className="text-red-500">Failed to load certificates</div>

  return (
    <div className="bg-dark-card rounded-lg border border-gray-800 overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm text-gray-400">
          <thead className="bg-gray-900 text-gray-200 uppercase font-medium">
            <tr>
              <th
                onClick={() => handleSort('name')}
                className="px-6 py-3 cursor-pointer hover:text-white transition-colors"
              >
                <div className="flex items-center gap-1">
                  Name
                  <SortIcon column="name" />
                </div>
              </th>
              <th className="px-6 py-3">Domain</th>
              <th className="px-6 py-3">Issuer</th>
              <th
                onClick={() => handleSort('expires')}
                className="px-6 py-3 cursor-pointer hover:text-white transition-colors"
              >
                <div className="flex items-center gap-1">
                  Expires
                  <SortIcon column="expires" />
                </div>
              </th>
              <th className="px-6 py-3">Status</th>
              <th className="px-6 py-3">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {certificates.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-6 py-8 text-center text-gray-500">
                  No certificates found.
                </td>
              </tr>
            ) : (
              sortedCertificates.map((cert) => (
                <tr key={cert.id || cert.domain} className="hover:bg-gray-800/50 transition-colors">
                  <td className="px-6 py-4 font-medium text-white">{cert.name || '-'}</td>
                  <td className="px-6 py-4 font-medium text-white">{cert.domain}</td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <span>{cert.issuer}</span>
                      {cert.issuer?.toLowerCase().includes('staging') && (
                        <span className="px-2 py-0.5 text-xs font-medium bg-yellow-500/10 text-yellow-400 border border-yellow-500/20 rounded">
                          STAGING
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    {new Date(cert.expires_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4">
                    <StatusBadge status={cert.status} />
                  </td>
                  <td className="px-6 py-4">
                    {cert.id && (cert.provider === 'custom' || cert.issuer?.includes('staging')) && (
                      <button
                        onClick={() => {
                          const message = cert.provider === 'custom'
                            ? 'Are you sure you want to delete this certificate?'
                            : 'Delete this staging certificate? It will be regenerated on next request.'
                          if (confirm(message)) {
                            deleteMutation.mutate(cert.id!)
                          }
                        }}
                        className="text-red-400 hover:text-red-300 transition-colors"
                        title={cert.provider === 'custom' ? 'Delete Certificate' : 'Delete Staging Certificate'}
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
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
    untrusted: 'bg-orange-900/30 text-orange-400 border-orange-800',
  }

  const labels = {
    valid: 'Valid',
    expiring: 'Expiring Soon',
    expired: 'Expired',
    untrusted: 'Untrusted (Staging)',
  }

  const style = styles[status as keyof typeof styles] || styles.valid
  const label = labels[status as keyof typeof labels] || status

  return (
    <span className={`px-2.5 py-0.5 rounded-full text-xs font-medium border ${style}`}>
      {label}
    </span>
  )
}
