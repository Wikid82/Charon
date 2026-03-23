import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2, ChevronUp, ChevronDown } from 'lucide-react'
import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import BulkDeleteCertificateDialog from './dialogs/BulkDeleteCertificateDialog'
import DeleteCertificateDialog from './dialogs/DeleteCertificateDialog'
import { LoadingSpinner, ConfigReloadOverlay } from './LoadingStates'
import { Button } from './ui/Button'
import { Checkbox } from './ui/Checkbox'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from './ui/Tooltip'
import { deleteCertificate, type Certificate } from '../api/certificates'
import { useCertificates } from '../hooks/useCertificates'
import { useProxyHosts } from '../hooks/useProxyHosts'
import { toast } from '../utils/toast'

import type { ProxyHost } from '../api/proxyHosts'

type SortColumn = 'name' | 'expires'
type SortDirection = 'asc' | 'desc'

export function isInUse(cert: Certificate, hosts: ProxyHost[]): boolean {
  if (!cert.id) return false
  return hosts.some(h => (h.certificate_id ?? h.certificate?.id) === cert.id)
}

export function isDeletable(cert: Certificate, hosts: ProxyHost[]): boolean {
  if (!cert.id) return false
  if (isInUse(cert, hosts)) return false
  return (
    cert.provider === 'custom' ||
    cert.provider === 'letsencrypt-staging' ||
    cert.status === 'expired' ||
    cert.status === 'expiring'
  )
}

export default function CertificateList() {
  const { certificates, isLoading, error } = useCertificates()
  const { hosts } = useProxyHosts()
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  const [sortColumn, setSortColumn] = useState<SortColumn>('name')
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc')
  const [certToDelete, setCertToDelete] = useState<Certificate | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [showBulkDeleteDialog, setShowBulkDeleteDialog] = useState(false)

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await deleteCertificate(id)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['certificates'] })
      queryClient.invalidateQueries({ queryKey: ['proxyHosts'] })
      toast.success(t('certificates.deleteSuccess'))
      setCertToDelete(null)
    },
    onError: (error: Error) => {
      toast.error(`${t('certificates.deleteFailed')}: ${error.message}`)
      setCertToDelete(null)
    },
  })

  const bulkDeleteMutation = useMutation({
    mutationFn: async (ids: number[]) => {
      const results = await Promise.allSettled(ids.map(id => deleteCertificate(id)))
      const failed = results.filter(r => r.status === 'rejected').length
      const succeeded = results.filter(r => r.status === 'fulfilled').length
      return { succeeded, failed }
    },
    onSuccess: ({ succeeded, failed }) => {
      queryClient.invalidateQueries({ queryKey: ['certificates'] })
      queryClient.invalidateQueries({ queryKey: ['proxyHosts'] })
      setSelectedIds(new Set())
      setShowBulkDeleteDialog(false)
      if (failed > 0) {
        toast.error(t('certificates.bulkDeletePartial', { deleted: succeeded, failed }))
      } else {
        toast.success(t('certificates.bulkDeleteSuccess', { count: succeeded }))
      }
    },
    onError: () => {
      toast.error(t('certificates.bulkDeleteFailed'))
      setShowBulkDeleteDialog(false)
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

  const selectableCertIds = useMemo<Set<number>>(() => {
    const ids = new Set<number>()
    for (const cert of sortedCertificates) {
      if (isDeletable(cert, hosts) && cert.id) {
        ids.add(cert.id)
      }
    }
    return ids
  }, [sortedCertificates, hosts])

  const allSelectableSelected =
    selectableCertIds.size > 0 && selectedIds.size === selectableCertIds.size
  const someSelected =
    selectedIds.size > 0 && selectedIds.size < selectableCertIds.size

  const handleSelectAll = () => {
    if (selectedIds.size === selectableCertIds.size) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(selectableCertIds))
    }
  }

  const handleSelectRow = (id: number) => {
    const next = new Set(selectedIds)
    if (next.has(id)) {
      next.delete(id)
    } else {
      next.add(id)
    }
    setSelectedIds(next)
  }

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
    <>
      {(deleteMutation.isPending || bulkDeleteMutation.isPending) && (
        <ConfigReloadOverlay
          message="Returning to shore..."
          submessage="Certificate departure in progress"
          type="charon"
        />
      )}
      {selectedIds.size > 0 && (
        <div
          role="status"
          aria-live="polite"
          className="flex items-center justify-between rounded-lg border border-brand-500/30 bg-brand-500/10 px-4 py-2 mb-3"
        >
          <span className="text-sm text-gray-300">
            {t('certificates.bulkSelectedCount', { count: selectedIds.size })}
          </span>
          <Button
            variant="danger"
            size="sm"
            leftIcon={Trash2}
            onClick={() => setShowBulkDeleteDialog(true)}
          >
            {t('certificates.bulkDeleteButton', { count: selectedIds.size })}
          </Button>
        </div>
      )}
      <div className="bg-dark-card rounded-lg border border-gray-800 overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm text-gray-400">
          <thead className="bg-gray-900 text-gray-200 uppercase font-medium">
            <tr>
              <th className="w-12 px-4 py-3">
                <Checkbox
                  checked={allSelectableSelected}
                  indeterminate={someSelected}
                  onCheckedChange={handleSelectAll}
                  aria-label={t('certificates.bulkSelectAll')}
                  disabled={selectableCertIds.size === 0}
                />
              </th>
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
                <td colSpan={7} className="px-6 py-8 text-center text-gray-500">
                  No certificates found.
                </td>
              </tr>
            ) : (
              sortedCertificates.map((cert) => {
                const inUse = isInUse(cert, hosts)
                const deletable = isDeletable(cert, hosts)
                const isInUseDeletableCategory = inUse && (cert.provider === 'custom' || cert.provider === 'letsencrypt-staging' || cert.status === 'expired' || cert.status === 'expiring')

                return (
                <tr key={cert.id || cert.domain} className="hover:bg-gray-800/50 transition-colors">
                  {deletable && !inUse ? (
                    <td className="w-12 px-4 py-4">
                      <Checkbox
                        checked={selectedIds.has(cert.id!)}
                        onCheckedChange={() => handleSelectRow(cert.id!)}
                        aria-label={t('certificates.selectCert', { name: cert.name || cert.domain })}
                      />
                    </td>
                  ) : isInUseDeletableCategory ? (
                    <td className="w-12 px-4 py-4">
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className="inline-flex">
                              <Checkbox
                                checked={false}
                                disabled
                                aria-disabled="true"
                                aria-label={t('certificates.selectCert', { name: cert.name || cert.domain })}
                              />
                            </span>
                          </TooltipTrigger>
                          <TooltipContent>{t('certificates.deleteInUse')}</TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </td>
                  ) : (
                    <td className="w-12 px-4 py-4" aria-hidden="true" />
                  )}
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
                    {(() => {
                      if (cert.id && inUse && (cert.provider === 'custom' || cert.provider === 'letsencrypt-staging' || cert.status === 'expired')) {
                        return (
                          <TooltipProvider>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <button
                                  aria-disabled="true"
                                  aria-label={t('certificates.deleteTitle')}
                                  className="text-red-400/40 cursor-not-allowed transition-colors"
                                  onClick={(e) => e.preventDefault()}
                                >
                                  <Trash2 className="w-4 h-4" />
                                </button>
                              </TooltipTrigger>
                              <TooltipContent>
                                {t('certificates.deleteInUse')}
                              </TooltipContent>
                            </Tooltip>
                          </TooltipProvider>
                        )
                      }

                      if (deletable) {
                        return (
                          <button
                            onClick={() => setCertToDelete(cert)}
                            className="text-red-400 hover:text-red-300 transition-colors"
                            aria-label={t('certificates.deleteTitle')}
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        )
                      }

                      return null
                    })()}
                  </td>
                </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
      </div>
      <DeleteCertificateDialog
        certificate={certToDelete}
        open={certToDelete !== null}
        onConfirm={() => {
          if (certToDelete?.id) {
            deleteMutation.mutate(certToDelete.id)
          }
        }}
        onCancel={() => setCertToDelete(null)}
        isDeleting={deleteMutation.isPending}
      />
      <BulkDeleteCertificateDialog
        certificates={sortedCertificates.filter(c => c.id && selectedIds.has(c.id))}
        open={showBulkDeleteDialog}
        onConfirm={() => bulkDeleteMutation.mutate(Array.from(selectedIds))}
        onCancel={() => setShowBulkDeleteDialog(false)}
        isDeleting={bulkDeleteMutation.isPending}
      />
    </>
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
