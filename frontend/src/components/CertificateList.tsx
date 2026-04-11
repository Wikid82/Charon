import { Download, Eye, Trash2, ChevronUp, ChevronDown } from 'lucide-react'
import { useState, useMemo, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import BulkDeleteCertificateDialog from './dialogs/BulkDeleteCertificateDialog'
import CertificateDetailDialog from './dialogs/CertificateDetailDialog'
import CertificateExportDialog from './dialogs/CertificateExportDialog'
import DeleteCertificateDialog from './dialogs/DeleteCertificateDialog'
import { LoadingSpinner, ConfigReloadOverlay } from './LoadingStates'
import { Button } from './ui/Button'
import { Checkbox } from './ui/Checkbox'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from './ui/Tooltip'
import { type Certificate } from '../api/certificates'
import { useCertificates, useDeleteCertificate, useBulkDeleteCertificates } from '../hooks/useCertificates'
import { toast } from '../utils/toast'

type SortColumn = 'name' | 'expires'
type SortDirection = 'asc' | 'desc'

export function isInUse(cert: Certificate): boolean {
  return cert.in_use
}

export function isDeletable(cert: Certificate): boolean {
  if (cert.in_use) return false
  return (
    cert.provider === 'custom' ||
    cert.provider === 'letsencrypt-staging' ||
    cert.status === 'expired' ||
    cert.status === 'expiring'
  )
}

function daysUntilExpiry(expiresAt: string): number {
  const now = new Date()
  const expiry = new Date(expiresAt)
  return Math.ceil((expiry.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))
}

export default function CertificateList() {
  const { certificates, isLoading, error } = useCertificates()
  const { t } = useTranslation()
  const [sortColumn, setSortColumn] = useState<SortColumn>('name')
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc')
  const [certToDelete, setCertToDelete] = useState<Certificate | null>(null)
  const [certToView, setCertToView] = useState<Certificate | null>(null)
  const [certToExport, setCertToExport] = useState<Certificate | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [showBulkDeleteDialog, setShowBulkDeleteDialog] = useState(false)

  const deleteMutation = useDeleteCertificate()

  useEffect(() => {
    setSelectedIds(prev => {
      const validIds = new Set(certificates.map(c => c.uuid).filter(Boolean))
      const reconciled = new Set([...prev].filter(id => validIds.has(id)))
      if (reconciled.size === prev.size) return prev
      return reconciled
    })
  }, [certificates])

  const handleDelete = (cert: Certificate) => {
    deleteMutation.mutate(cert.uuid, {
      onSuccess: () => {
        toast.success(t('certificates.deleteSuccess'))
        setCertToDelete(null)
      },
      onError: (error: Error) => {
        toast.error(`${t('certificates.deleteFailed')}: ${error.message}`)
        setCertToDelete(null)
      },
    })
  }

  const bulkDeleteMutation = useBulkDeleteCertificates()

  const sortedCertificates = useMemo(() => {
    return [...certificates].sort((a, b) => {
      let comparison = 0

      switch (sortColumn) {
        case 'name': {
          const aName = (a.name || a.domains || '').toLowerCase()
          const bName = (b.name || b.domains || '').toLowerCase()
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

  const selectableCertIds = useMemo<Set<string>>(() => {
    const ids = new Set<string>()
    for (const cert of sortedCertificates) {
      if (isDeletable(cert) && cert.uuid) {
        ids.add(cert.uuid)
      }
    }
    return ids
  }, [sortedCertificates])

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

  const handleSelectRow = (uuid: string) => {
    const next = new Set(selectedIds)
    if (next.has(uuid)) {
      next.delete(uuid)
    } else {
      next.add(uuid)
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
                const inUse = isInUse(cert)
                const deletable = isDeletable(cert)
                const isInUseDeletableCategory = inUse && (cert.provider === 'custom' || cert.provider === 'letsencrypt-staging' || cert.status === 'expired' || cert.status === 'expiring')
                const days = daysUntilExpiry(cert.expires_at)

                return (
                <tr key={cert.uuid} className="hover:bg-gray-800/50 transition-colors">
                  {deletable && !inUse ? (
                    <td className="w-12 px-4 py-4">
                      <Checkbox
                        checked={selectedIds.has(cert.uuid)}
                        onCheckedChange={() => handleSelectRow(cert.uuid)}
                        aria-label={t('certificates.selectCert', { name: cert.name || cert.domains })}
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
                                aria-label={t('certificates.selectCert', { name: cert.name || cert.domains })}
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
                  <td className="px-6 py-4 font-medium text-white">{cert.domains}</td>
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
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span className={days <= 0 ? 'text-red-400' : days <= 30 ? 'text-yellow-400' : ''}>
                            {new Date(cert.expires_at).toLocaleDateString()}
                          </span>
                        </TooltipTrigger>
                        <TooltipContent>
                          {days > 0
                            ? t('certificates.expiresInDays', { days })
                            : t('certificates.expiredAgo', { days: Math.abs(days) })}
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </td>
                  <td className="px-6 py-4">
                    <StatusBadge status={cert.status} />
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => setCertToView(cert)}
                        className="text-gray-400 hover:text-white transition-colors"
                        aria-label={t('certificates.viewDetails')}
                        data-testid={`view-cert-${cert.uuid}`}
                      >
                        <Eye className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => setCertToExport(cert)}
                        className="text-gray-400 hover:text-white transition-colors"
                        aria-label={t('certificates.export')}
                        data-testid={`export-cert-${cert.uuid}`}
                      >
                        <Download className="w-4 h-4" />
                      </button>
                      {(() => {
                        if (inUse && (cert.provider === 'custom' || cert.provider === 'letsencrypt-staging' || cert.status === 'expired')) {
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
                    </div>
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
          if (certToDelete?.uuid) {
            handleDelete(certToDelete)
          }
        }}
        onCancel={() => setCertToDelete(null)}
        isDeleting={deleteMutation.isPending}
      />
      <BulkDeleteCertificateDialog
        certificates={sortedCertificates.filter(c => selectedIds.has(c.uuid))}
        open={showBulkDeleteDialog}
        onConfirm={() => bulkDeleteMutation.mutate(Array.from(selectedIds), {
          onSuccess: ({ succeeded, failed }) => {
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
        })}
        onCancel={() => setShowBulkDeleteDialog(false)}
        isDeleting={bulkDeleteMutation.isPending}
      />
      <CertificateDetailDialog
        certificate={certToView}
        open={certToView !== null}
        onOpenChange={(open) => { if (!open) setCertToView(null) }}
      />
      <CertificateExportDialog
        certificate={certToExport}
        open={certToExport !== null}
        onOpenChange={(open) => { if (!open) setCertToExport(null) }}
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
