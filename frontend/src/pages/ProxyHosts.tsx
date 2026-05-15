import { useQuery } from '@tanstack/react-query'
import { Loader2, ExternalLink, AlertTriangle, Trash2, Globe, Settings, FolderOpen } from 'lucide-react'
import { useState, useMemo } from 'react'
import { toast } from 'react-hot-toast'
import { useTranslation } from 'react-i18next'

import { createBackup } from '../api/backups'
import { deleteCertificate } from '../api/certificates'
import { getSettings } from '../api/settings'
import { getMonitors, type UptimeMonitor } from '../api/uptime'
import CertificateCleanupDialog from '../components/dialogs/CertificateCleanupDialog'
import { PageShell } from '../components/layout/PageShell'
import { ConfigReloadOverlay } from '../components/LoadingStates'
import ProxyHostForm from '../components/ProxyHostForm'
import {
  Badge,
  Alert,
  Button,
  Switch,
  DataTable,
  EmptyState,
  SkeletonTable,
  Checkbox,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
  type Column,
} from '../components/ui'
import { useAccessLists } from '../hooks/useAccessLists'
import { useCertificates } from '../hooks/useCertificates'
import { useProxyHosts } from '../hooks/useProxyHosts'
import { useDeleteProxyGroup, useProxyGroups } from '../hooks/useProxyGroups'
import { useSecurityHeaderProfiles } from '../hooks/useSecurityHeaders'
import { ProxyGroupBadge } from '../components/ProxyGroupBadge'
import { ProxyGroupForm } from '../components/ProxyGroupForm'
import compareHosts from '../utils/compareHosts'
import { formatSettingLabel, settingHelpText, applyBulkSettingsToHosts } from '../utils/proxyHostsHelpers'

import type { AccessList } from '../api/accessLists'
import type { ProxyGroup } from '../api/proxyGroups'
import type { ProxyHost } from '../api/proxyHosts'


export default function ProxyHosts() {
  const { t } = useTranslation()
  const { hosts, loading, isFetching, error, createHost, updateHost, deleteHost, bulkUpdateACL, bulkUpdateSecurityHeaders, isBulkUpdating, isCreating, isUpdating, isDeleting } = useProxyHosts()
  const { certificates } = useCertificates()
  const { data: accessLists } = useAccessLists()
  const { data: securityProfiles } = useSecurityHeaderProfiles()
  const [showForm, setShowForm] = useState(false)
  const [editingHost, setEditingHost] = useState<ProxyHost | undefined>()
  const [selectedHosts, setSelectedHosts] = useState<Set<string>>(new Set())
  const [showBulkACLModal, setShowBulkACLModal] = useState(false)
  const [showBulkApplyModal, setShowBulkApplyModal] = useState(false)
  const [showBulkDeleteModal, setShowBulkDeleteModal] = useState(false)
  const [isCreatingBackup, setIsCreatingBackup] = useState(false)
  const [showCertCleanupDialog, setShowCertCleanupDialog] = useState(false)
  const [certCleanupData, setCertCleanupData] = useState<{
    hostUUIDs: string[]
    hostNames: string[]
    certificates: Array<{ uuid: string; name: string; domain: string }>
    isBulk: boolean
  } | null>(null)
  const [selectedACLs, setSelectedACLs] = useState<Set<number>>(new Set())
  const [bulkACLAction, setBulkACLAction] = useState<'apply' | 'remove'>('apply')
  const [applyProgress, setApplyProgress] = useState<{ current: number; total: number } | null>(null)
  const [bulkApplySettings, setBulkApplySettings] = useState<Record<string, { apply: boolean; value: boolean }>>({
    ssl_forced: { apply: false, value: true },
    http2_support: { apply: false, value: true },
    hsts_enabled: { apply: false, value: true },
    hsts_subdomains: { apply: false, value: true },
    block_exploits: { apply: false, value: true },
    websocket_support: { apply: false, value: true },
    enable_standard_headers: { apply: false, value: true },
  })
  const [bulkSecurityHeaderProfile, setBulkSecurityHeaderProfile] = useState<{
    apply: boolean;
    profileId: number | null;
  }>({ apply: false, profileId: null })
  const [hostToDelete, setHostToDelete] = useState<ProxyHost | null>(null)
  const [showGroupForm, setShowGroupForm] = useState(false)
  const [editingGroup, setEditingGroup] = useState<ProxyGroup | undefined>()
  const [groupToDelete, setGroupToDelete] = useState<ProxyGroup | null>(null)
  const [showAssignGroupModal, setShowAssignGroupModal] = useState(false)
  const [assignTargetGroupUUID, setAssignTargetGroupUUID] = useState<string | null>(null)
  const { data: groups = [] } = useProxyGroups()
  const deleteGroup = useDeleteProxyGroup()

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
  })

  const linkBehavior = settings?.['ui.domain_link_behavior'] || 'new_tab'

  // Determine if any mutation is in progress
  const isApplyingConfig = isCreating || isUpdating || isDeleting || isBulkUpdating

  // Determine contextual message based on operation
  const getMessage = () => {
    if (isCreating) return { message: t('proxyHosts.ferryingNewHost'), submessage: t('proxyHosts.ferryingNewHostSub') }
    if (isUpdating) return { message: t('proxyHosts.guidingChanges'), submessage: t('proxyHosts.guidingChangesSub') }
    if (isDeleting) return { message: t('proxyHosts.returningToShore'), submessage: t('proxyHosts.returningToShoreSub') }
    if (isBulkUpdating) return { message: t('proxyHosts.ferryingSouls', { count: selectedHosts.size }), submessage: t('proxyHosts.ferryingSoulsSub') }
    return { message: t('proxyHosts.ferryingConfig'), submessage: t('proxyHosts.ferryingNewHostSub') }
  }

  const { message, submessage } = getMessage()

  // Create a map of domain -> certificate status for quick lookup
  const certStatusByDomain = useMemo(() => {
    const map: Record<string, { status: string; provider: string }> = {}
    for (const cert of certificates) {
      const domains = (cert.domains || '').split(',').map(d => d.trim().toLowerCase()).filter(Boolean)
      for (const domain of domains) {
        if (!map[domain]) {
          map[domain] = { status: cert.status, provider: cert.provider }
        }
      }
    }
    return map
  }, [certificates])

  // Sort hosts alphabetically by name for display
  const sortedHosts = useMemo(() => [...hosts].sort((a, b) => compareHosts(a, b, 'name', 'asc')), [hosts])

  const groupedHosts = useMemo(() => {
    const byGroup: Record<string, ProxyHost[]> = {}
    const ungrouped: ProxyHost[] = []
    for (const host of sortedHosts) {
      const gid = host.proxy_group?.uuid
      if (gid) {
        if (!byGroup[gid]) byGroup[gid] = []
        byGroup[gid].push(host)
      } else {
        ungrouped.push(host)
      }
    }
    return { byGroup, ungrouped }
  }, [sortedHosts])

  const handleDomainClick = (e: React.MouseEvent, url: string) => {
    if (linkBehavior === 'new_window') {
      e.preventDefault()
      window.open(url, '_blank', 'noopener,noreferrer,width=1024,height=768')
    }
  }

  const handleAdd = () => {
    setEditingHost(undefined)
    setShowForm(true)
  }

  const handleEdit = (host: ProxyHost) => {
    setEditingHost(host)
    setShowForm(true)
  }

  const handleSubmit = async (data: Partial<ProxyHost>) => {
    await (editingHost ? updateHost(editingHost.uuid, data) : createHost(data));
    setShowForm(false)
    setEditingHost(undefined)
  }

  const handleDeleteClick = (host: ProxyHost) => {
    setHostToDelete(host)
  }

  const handleDeleteConfirm = async () => {
    if (!hostToDelete) return
    const host = hostToDelete

    // Check for orphaned certificates that would need cleanup
    const orphanedCerts: Array<{ uuid: string; name: string; domain: string }> = []

    if (host.certificate_id && host.certificate) {
      const cert = host.certificate
      const otherHostsUsingCert = hosts.filter(h =>
        h.uuid !== host.uuid && h.certificate_id === host.certificate_id
      ).length

      if (otherHostsUsingCert === 0) {
        const isCustomOrStaging = cert.provider === 'custom' || cert.provider?.toLowerCase().includes('staging')
        if (isCustomOrStaging) {
          orphanedCerts.push({
            uuid: cert.uuid,
            name: cert.name || '',
            domain: cert.domains
          })
        }
      }
    }

    // If there are orphaned certificates, show cleanup dialog
    if (orphanedCerts.length > 0) {
      setCertCleanupData({
        hostUUIDs: [host.uuid],
        hostNames: [host.name || host.domain_names],
        certificates: orphanedCerts,
        isBulk: false
      })
      setShowCertCleanupDialog(true)
      setHostToDelete(null)
      return
    }

    try {
      let associatedMonitors: UptimeMonitor[] = []
      try {
        const monitors = await getMonitors()
        associatedMonitors = monitors.filter(m => m.upstream_host === host.forward_host || (m.proxy_host_id && m.proxy_host_id === (host as unknown as { id?: number }).id))
      } catch {
        // ignore errors fetching uptime data
      }

      if (associatedMonitors.length > 0) {
        const deleteUptime = confirm('This proxy host has uptime monitors associated with it. Delete the monitors as well?')
        await deleteHost(host.uuid, deleteUptime)
      } else {
        await deleteHost(host.uuid)
      }
      setHostToDelete(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete')
    }
  }

  const handleDelete = async (uuid: string) => {
    const host = hosts.find(h => h.uuid === uuid)
    if (host) {
      handleDeleteClick(host)
    }
  }

  const handleCertCleanupConfirm = async (deleteCerts: boolean) => {
    if (!certCleanupData) return

    setShowCertCleanupDialog(false)

    try {
      // Delete hosts first
      if (certCleanupData.isBulk) {
        // Bulk deletion
        let deleted = 0
        let failed = 0

        for (const uuid of certCleanupData.hostUUIDs) {
          try {
            await deleteHost(uuid)
            deleted++
          } catch {
            failed++
          }
        }

        // Delete certificates if user confirmed
        if (deleteCerts && certCleanupData.certificates.length > 0) {
          let certsDeleted = 0
          let certsFailed = 0

          for (const cert of certCleanupData.certificates) {
            try {
              await deleteCertificate(cert.uuid)
              certsDeleted++
            } catch {
              certsFailed++
            }
          }

          if (certsFailed > 0) {
            toast.error(`Deleted ${deleted} host(s) and ${certsDeleted} certificate(s), ${certsFailed} certificate(s) failed`)
          } else {
            toast.success(`Deleted ${deleted} host(s) and ${certsDeleted} certificate(s)`)
          }
        } else {
          if (failed > 0) {
            toast.error(`Deleted ${deleted} host(s), ${failed} failed`)
          } else {
            toast.success(`Successfully deleted ${deleted} host(s)`)
          }
        }
      } else {
        // Single deletion
        const uuid = certCleanupData.hostUUIDs[0]
        const host = hosts.find(h => h.uuid === uuid)

        // Check for uptime monitors
        let associatedMonitors: UptimeMonitor[] = []
        try {
          const monitors = await getMonitors()
          associatedMonitors = monitors.filter(m =>
            host && (m.upstream_host === host.forward_host || (m.proxy_host_id && m.proxy_host_id === (host as unknown as { id?: number }).id))
          )
        } catch {
          // ignore errors
        }

        if (associatedMonitors.length > 0) {
          const deleteUptime = confirm('This proxy host has uptime monitors associated with it. Delete the monitors as well?')
          await deleteHost(uuid, deleteUptime)
        } else {
          await deleteHost(uuid)
        }

        // Delete certificate if user confirmed
        if (deleteCerts && certCleanupData.certificates.length > 0) {
          try {
            await deleteCertificate(certCleanupData.certificates[0].uuid)
            toast.success('Proxy host and certificate deleted')
          } catch (err) {
            toast.error(`Proxy host deleted but failed to delete certificate: ${err instanceof Error ? err.message : 'Unknown error'}`)
          }
        }
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete')
    } finally {
      setCertCleanupData(null)
      setSelectedHosts(new Set())
      setShowBulkDeleteModal(false)
    }
  }

  const handleBulkApplyACL = async (accessListID: number | null) => {
    const hostUUIDs = Array.from(selectedHosts)
    try {
      const result = await bulkUpdateACL(hostUUIDs, accessListID)

      if (result.errors.length > 0) {
        toast.error(`Updated ${result.updated} host(s), ${result.errors.length} failed`)
      } else {
        const action = accessListID ? 'applied to' : 'removed from'
        toast.success(`Access list ${action} ${result.updated} host(s)`)
      }

      setSelectedHosts(new Set())
      setShowBulkACLModal(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update hosts')
    }
  }

  const handleBulkDelete = async () => {
    const hostUUIDs = Array.from(selectedHosts)
    setIsCreatingBackup(true)

    try {
      // Create automatic backup before deletion
      toast.loading('Creating backup before deletion...')
      const backup = await createBackup()
      toast.dismiss()
      toast.success(`Backup created: ${backup.filename}`)

      // Collect certificates to potentially delete
      const certsToConsider: Map<string, { uuid: string; name: string; domain: string }> = new Map()

      for (const uuid of hostUUIDs) {
        const host = hosts.find(h => h.uuid === uuid)
        if (host?.certificate_id && host.certificate) {
          const cert = host.certificate
          // Only consider custom/staging certs
          const isCustomOrStaging = cert.provider === 'custom' || cert.provider?.toLowerCase().includes('staging')
          if (isCustomOrStaging) {
            // Check if this cert is ONLY used by hosts being deleted
            const otherHosts = hosts.filter(h =>
              h.certificate_id === host.certificate_id &&
              !hostUUIDs.includes(h.uuid)
            )
            if (otherHosts.length === 0 && cert.uuid) {
              certsToConsider.set(cert.uuid, {
                uuid: cert.uuid,
                name: cert.name || '',
                domain: cert.domains
              })
            }
          }
        }
      }

      // If there are orphaned certificates, show cleanup dialog
      if (certsToConsider.size > 0) {
        const hostNames = hostUUIDs.map(uuid => {
          const host = hosts.find(h => h.uuid === uuid)
          return host?.name || host?.domain_names || 'Unnamed'
        })

        setCertCleanupData({
          hostUUIDs,
          hostNames,
          certificates: Array.from(certsToConsider.values()),
          isBulk: true
        })
        setShowCertCleanupDialog(true)
        setShowBulkDeleteModal(false) // Close bulk delete modal when showing cert cleanup
        setIsCreatingBackup(false)
        return
      }

      // No orphaned certificates, proceed with deletion
      let deleted = 0
      let failed = 0

      for (const uuid of hostUUIDs) {
        try {
          await deleteHost(uuid)
          deleted++
        } catch {
          failed++
        }
      }

      if (failed > 0) {
        toast.error(`Deleted ${deleted} host(s), ${failed} failed`)
      } else {
        toast.success(`Successfully deleted ${deleted} host(s). Backup available for restore.`)
      }

      setSelectedHosts(new Set())
      setShowBulkDeleteModal(false)
    } catch (err) {
      toast.dismiss()
      toast.error(err instanceof Error ? err.message : 'Failed to create backup')
    } finally {
      setIsCreatingBackup(false)
    }
  }

  // DataTable columns definition
  const columns: Column<ProxyHost>[] = [
    {
      key: 'name',
      header: t('proxyHosts.columnName'),
      sortable: true,
      width: '18%',
      cell: (host) => (
        <div className="text-sm font-medium text-content-primary truncate">
          {host.name || <span className="text-content-muted italic">{t('proxyHosts.unnamed')}</span>}
        </div>
      ),
    },
    {
      key: 'domain',
      header: t('proxyHosts.columnDomain'),
      sortable: true,
      width: '24%',
      cell: (host) => (
        <div className="text-sm font-medium text-content-primary">
          {host.domain_names.split(',').map((domain, i) => {
            const d = domain.trim()
            const url = `${host.ssl_forced ? 'https' : 'http'}://${d}`
            return (
              <div key={i} className="flex items-center gap-1">
                <a
                  href={url}
                  title={url}
                  target={linkBehavior === 'same_tab' ? '_self' : '_blank'}
                  rel="noopener noreferrer"
                  onClick={(e) => handleDomainClick(e, url)}
                  className="hover:text-brand-400 hover:underline flex items-center gap-1 truncate"
                  style={{ maxWidth: '100%' }}
                >
                  <span className="truncate max-w-[30ch]">{d}</span>
                  <ExternalLink size={12} className="opacity-50 shrink-0" />
                </a>
              </div>
            )
          })}
        </div>
      ),
    },
    {
      key: 'forward',
      header: t('proxyHosts.columnForwardTo'),
      sortable: true,
      width: '18%',
      cell: (host) => (
        <div className="text-sm text-content-secondary">
          {host.forward_scheme}://{host.forward_host}:{host.forward_port}
        </div>
      ),
    },
    {
      key: 'ssl',
      header: t('proxyHosts.columnSSL'),
      width: '10%',
      cell: (host) => {
        const primaryDomain = host.domain_names.split(',')[0]?.trim().toLowerCase()
        const certInfo = certStatusByDomain[primaryDomain]
        const isUntrusted = certInfo?.status === 'untrusted'
        const isStaging = certInfo?.provider?.includes('staging')

        if (!host.ssl_forced) return <span className="text-content-muted">—</span>

        if (isUntrusted || isStaging) {
          return (
            <Badge variant="warning" size="sm" className="gap-1">
              <AlertTriangle size={12} />
              {t('proxyHosts.staging')}
            </Badge>
          )
        }

        return <Badge variant="success" size="sm">SSL</Badge>
      },
    },
    {
      key: 'features',
      header: t('proxyHosts.columnFeatures'),
      width: '12%',
      cell: (host) => (
        <div className="flex flex-wrap gap-1">
          {host.websocket_support && (
            <Badge variant="primary" size="sm">{t('proxyHosts.websocket')}</Badge>
          )}
          {host.access_list_id && (
            <Badge variant="outline" size="sm">{t('proxyHosts.acl')}</Badge>
          )}
        </div>
      ),
    },
    {
      key: 'group',
      header: t('proxyGroups.group'),
      width: '12%',
      cell: (host) =>
        host.proxy_group ? (
          <ProxyGroupBadge group={host.proxy_group} />
        ) : (
          <span className="text-content-muted text-sm">—</span>
        ),
    },
    {
      key: 'status',
      header: t('proxyHosts.columnStatus'),
      width: '8%',
      cell: (host) => (
        <Switch
          checked={host.enabled}
          aria-label={`${host.enabled ? 'Disable' : 'Enable'} proxy host ${host.name || host.domain_names}`}
          onCheckedChange={(checked) => updateHost(host.uuid, { enabled: checked })}
        />
      ),
    },
    {
      key: 'actions',
      header: t('proxyHosts.columnActions'),
      width: '10%',
      cell: (host) => (
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            aria-label={`Edit proxy host ${host.name || host.domain_names}`}
            onClick={(e) => {
              e.stopPropagation()
              handleEdit(host)
            }}
          >
            {t('common.edit')}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="text-error hover:text-error hover:bg-error/10"
            aria-label={`Delete proxy host ${host.name || host.domain_names}`}
            onClick={(e) => {
              e.stopPropagation()
              handleDelete(host.uuid)
            }}
          >
            {t('common.delete')}
          </Button>
        </div>
      ),
    },
  ]

  return (
    <>
      {isApplyingConfig && (
        <ConfigReloadOverlay
          message={message}
          submessage={submessage}
          type="charon"
        />
      )}

      <PageShell
        title={t('proxyHosts.title')}
        description={t('proxyHosts.description')}
        actions={
          <div className="flex items-center gap-3">
            {isFetching && !loading && <Loader2 className="animate-spin text-brand-400" size={20} />}
            <Button
              variant="outline"
              leftIcon={FolderOpen}
              onClick={() => {
                setEditingGroup(undefined)
                setShowGroupForm(true)
              }}
            >
              {t('proxyGroups.manageGroups')}
            </Button>
            <Button onClick={handleAdd}>{t('proxyHosts.addHost')}</Button>
          </div>
        }
      >
        {/* Error Alert */}
        {error && (
          <Alert variant="error" dismissible>
            {error}
          </Alert>
        )}

        {/* Bulk Actions Bar */}
        {selectedHosts.size > 0 && (
          <Alert variant="info" className="flex items-center justify-between">
            <span>
              <strong>{selectedHosts.size}</strong> {t('proxyHosts.selectedCount', { count: selectedHosts.size }).replace(`${selectedHosts.size} `, '')}
              {selectedHosts.size === hosts.length && ` ${t('proxyHosts.selectedCountAll')}`}
            </span>
            <div className="flex items-center gap-2">
              <Button
                variant="secondary"
                size="sm"
                leftIcon={Settings}
                onClick={() => setShowBulkApplyModal(true)}
              >
                {t('proxyHosts.bulkApply')}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowBulkACLModal(true)}
                disabled={isBulkUpdating}
                isLoading={isBulkUpdating}
              >
                {t('proxyHosts.manageACL')}
              </Button>
              {groups.length > 0 && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setAssignTargetGroupUUID(groups[0].uuid)
                    setShowAssignGroupModal(true)
                  }}
                >
                  {t('proxyGroups.assignToGroup')}
                </Button>
              )}
              <Button
                variant="danger"
                size="sm"
                leftIcon={Trash2}
                onClick={() => setShowBulkDeleteModal(true)}
              >
                {t('common.delete')}
              </Button>
            </div>
          </Alert>
        )}

        {/* Data Table */}
        {loading ? (
          <SkeletonTable rows={5} columns={7} />
        ) : groups.length === 0 ? (
          <DataTable
            data={sortedHosts}
            columns={columns}
            rowKey={(row) => row.uuid}
            selectable
            selectedKeys={selectedHosts}
            onSelectionChange={setSelectedHosts}
            stickyHeader
            emptyState={
              <EmptyState
                icon={<Globe className="h-12 w-12" />}
                title={t('proxyHosts.noHosts')}
                description={t('proxyHosts.noHostsDescription')}
                action={{
                  label: t('proxyHosts.addHost'),
                  onClick: handleAdd,
                }}
              />
            }
          />
        ) : (
          <div className="space-y-6">
            {groups.map((group) => {
              const groupHosts = groupedHosts.byGroup[group.uuid] ?? []
              return (
                <section key={group.uuid} aria-label={group.name}>
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <span
                        className="inline-block w-3 h-3 rounded-full shrink-0"
                        style={{ backgroundColor: group.color ?? '#6b7280' }}
                        aria-hidden="true"
                      />
                      <h2 className="text-sm font-semibold text-content-primary">{group.name}</h2>
                      <span className="text-xs text-content-muted">
                        {t('proxyGroups.hostCount', { count: groupHosts.length })}
                      </span>
                    </div>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label={`Edit group ${group.name}`}
                        onClick={() => {
                          setEditingGroup(group)
                          setShowGroupForm(true)
                        }}
                      >
                        {t('common.edit')}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-error hover:text-error hover:bg-error/10"
                        aria-label={`Delete group ${group.name}`}
                        onClick={() => setGroupToDelete(group)}
                      >
                        {t('common.delete')}
                      </Button>
                    </div>
                  </div>
                  <DataTable
                    data={groupHosts}
                    columns={columns}
                    rowKey={(row) => row.uuid}
                    selectable
                    selectedKeys={selectedHosts}
                    onSelectionChange={setSelectedHosts}
                    emptyState={
                      <EmptyState
                        icon={<Globe className="h-10 w-10" />}
                        title={t('proxyHosts.noHosts')}
                        description={t('proxyHosts.noHostsDescription')}
                        action={{ label: t('proxyHosts.addHost'), onClick: handleAdd }}
                      />
                    }
                  />
                </section>
              )
            })}
            {groupedHosts.ungrouped.length > 0 && (
              <section aria-label={t('proxyGroups.ungrouped')}>
                <div className="flex items-center gap-2 mb-2">
                  <h2 className="text-sm font-semibold text-content-muted">
                    {t('proxyGroups.ungrouped')}
                  </h2>
                  <span className="text-xs text-content-muted">
                    {t('proxyGroups.hostCount', { count: groupedHosts.ungrouped.length })}
                  </span>
                </div>
                <DataTable
                  data={groupedHosts.ungrouped}
                  columns={columns}
                  rowKey={(row) => row.uuid}
                  selectable
                  selectedKeys={selectedHosts}
                  onSelectionChange={setSelectedHosts}
                  emptyState={null}
                />
              </section>
            )}
            {sortedHosts.length === 0 && (
              <EmptyState
                icon={<Globe className="h-12 w-12" />}
                title={t('proxyHosts.noHosts')}
                description={t('proxyHosts.noHostsDescription')}
                action={{ label: t('proxyHosts.addHost'), onClick: handleAdd }}
              />
            )}
          </div>
        )}

        {/* Add/Edit Form Dialog */}
        {showForm && (
          <ProxyHostForm
            host={editingHost}
            onSubmit={handleSubmit}
            onCancel={() => {
              setShowForm(false)
              setEditingHost(undefined)
            }}
          />
        )}

        {/* Delete Confirmation Dialog */}
        <Dialog open={!!hostToDelete} onOpenChange={(open) => !open && setHostToDelete(null)}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('proxyHosts.deleteConfirmTitle')}</DialogTitle>
              <DialogDescription>
                {t('proxyHosts.deleteConfirmMessage', { name: hostToDelete?.name || hostToDelete?.domain_names }).split('<strong>').map((part, i) => {
                  if (i === 0) return part
                  const [strongText, rest] = part.split('</strong>')
                  return (
                    <span key={i}>
                      <strong>{strongText}</strong>
                      {rest}
                    </span>
                  )
                })}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setHostToDelete(null)}>
                {t('common.cancel')}
              </Button>
              <Button variant="danger" onClick={handleDeleteConfirm}>
                {t('common.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Bulk Apply Settings Dialog */}
        <Dialog open={showBulkApplyModal} onOpenChange={(open) => {
          setShowBulkApplyModal(open)
          if (!open) {
            setBulkSecurityHeaderProfile({ apply: false, profileId: null })
            setApplyProgress(null)
          }
        }}>
          <DialogContent className="max-w-md max-h-[80vh] overflow-hidden flex flex-col">
            <DialogHeader>
              <DialogTitle>{t('proxyHosts.bulkApplyTitle')}</DialogTitle>
              <DialogDescription>
                {t('proxyHosts.bulkApplyDescription', { count: selectedHosts.size }).split('<strong>').map((part, i) => {
                  if (i === 0) return part
                  const [strongText, rest] = part.split('</strong>')
                  return (
                    <span key={i}>
                      <strong className="text-brand-400">{strongText}</strong>
                      {rest}
                    </span>
                  )
                })}
              </DialogDescription>
            </DialogHeader>

            <div className="flex-1 overflow-y-auto space-y-3 py-4">
              {Object.entries(bulkApplySettings).map(([key, cfg]) => (
                <div key={key} className="flex items-center justify-between gap-3 p-3 bg-surface-subtle rounded-lg">
                  <div className="flex items-center gap-3">
                    <Checkbox
                      checked={cfg.apply}
                      onCheckedChange={(checked) => setBulkApplySettings(prev => ({
                        ...prev,
                        [key]: { ...prev[key], apply: !!checked }
                      }))}
                    />
                    <div>
                      <div className="text-sm font-medium text-content-primary">{formatSettingLabel(key)}</div>
                      <div className="text-xs text-content-muted">{settingHelpText(key)}</div>
                    </div>
                  </div>
                  <Switch
                    checked={cfg.value}
                    onCheckedChange={(v) => setBulkApplySettings(prev => ({
                      ...prev,
                      [key]: { ...prev[key], value: v }
                    }))}
                  />
                </div>
              ))}

              {/* Security Header Profile Section */}
              <div className="border-t border-border pt-3 mt-3">
                <div className="flex items-center justify-between gap-3 p-3 bg-surface-subtle rounded-lg">
                  <div className="flex items-center gap-3">
                    <Checkbox
                      checked={bulkSecurityHeaderProfile.apply}
                      onCheckedChange={(checked) => setBulkSecurityHeaderProfile(prev => ({
                        ...prev,
                        apply: !!checked
                      }))}
                    />
                    <div>
                      <div className="text-sm font-medium text-content-primary">
                        {t('proxyHosts.bulkApplySecurityHeaders')}
                      </div>
                      <div className="text-xs text-content-muted">
                        {t('proxyHosts.bulkApplySecurityHeadersHelp')}
                      </div>
                    </div>
                  </div>
                </div>

                {bulkSecurityHeaderProfile.apply && (
                  <div className="mt-3 p-3 bg-surface-subtle rounded-lg space-y-3">
                    <select
                      value={bulkSecurityHeaderProfile.profileId ?? 0}
                      onChange={(e) => setBulkSecurityHeaderProfile(prev => ({
                        ...prev,
                        profileId: e.target.value === "0" ? null : parseInt(e.target.value)
                      }))}
                      className="w-full bg-surface-muted border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-brand-500"
                    >
                      <option value={0}>{t('proxyHosts.noSecurityProfile')}</option>
                      {securityProfiles && securityProfiles.some(p => p.is_preset) && (
                        <optgroup label={t('securityHeaders.systemProfiles')}>
                          {securityProfiles
                            .filter(p => p.is_preset)
                            .sort((a, b) => a.security_score - b.security_score)
                            .map(profile => (
                              <option key={profile.id} value={profile.id}>
                                {profile.name} ({t('common.score')}: {profile.security_score}/100)
                              </option>
                            ))}
                        </optgroup>
                      )}
                      {securityProfiles && securityProfiles.some(p => !p.is_preset) && (
                        <optgroup label={t('securityHeaders.customProfiles')}>
                          {securityProfiles
                            .filter(p => !p.is_preset)
                            .map(profile => (
                              <option key={profile.id} value={profile.id}>
                                {profile.name} ({t('common.score')}: {profile.security_score}/100)
                              </option>
                            ))}
                        </optgroup>
                      )}
                    </select>

                    {bulkSecurityHeaderProfile.profileId === null && (
                      <Alert variant="warning">
                        {t('proxyHosts.removeSecurityHeadersWarning')}
                      </Alert>
                    )}

                    {bulkSecurityHeaderProfile.profileId && (() => {
                      const selected = securityProfiles?.find(p => p.id === bulkSecurityHeaderProfile.profileId)
                      if (!selected) return null
                      return (
                        <div className="text-xs text-content-muted">
                          {selected.description}
                        </div>
                      )
                    })()}
                  </div>
                )}
              </div>
            </div>

            {applyProgress && (
              <div className="border border-brand-500/30 rounded-lg bg-brand-500/10 p-4">
                <div className="flex items-center gap-3 mb-2">
                  <Loader2 className="w-5 h-5 animate-spin text-brand-400" />
                  <span className="text-brand-300 font-medium">
                    {t('proxyHosts.applyingACLs', { current: applyProgress.current, total: applyProgress.total })}
                  </span>
                </div>
                <div className="w-full bg-surface-muted rounded-full h-2">
                  <div
                    className="bg-brand-500 h-2 rounded-full transition-all duration-300"
                    style={{ width: `${(applyProgress.current / applyProgress.total) * 100}%` }}
                  />
                </div>
              </div>
            )}

            <DialogFooter>
              <Button
                variant="ghost"
                onClick={() => {
                  setShowBulkApplyModal(false)
                  setBulkSecurityHeaderProfile({ apply: false, profileId: null })
                  setApplyProgress(null)
                }}
                disabled={applyProgress !== null}
              >
                {t('common.cancel')}
              </Button>
              <Button
                onClick={async () => {
                  const keysToApply = Object.keys(bulkApplySettings).filter(k => bulkApplySettings[k].apply)
                  const hostUUIDs = Array.from(selectedHosts)
                  let totalErrors = 0

                  // Apply boolean settings
                  if (keysToApply.length > 0) {
                    const result = await applyBulkSettingsToHosts({
                      hosts,
                      hostUUIDs,
                      keysToApply,
                      bulkApplySettings,
                      updateHost,
                      setApplyProgress
                    })
                    totalErrors += result.errors
                  }

                  // Apply security header profile if selected
                  if (bulkSecurityHeaderProfile.apply) {
                    try {
                      const result = await bulkUpdateSecurityHeaders(
                        hostUUIDs,
                        bulkSecurityHeaderProfile.profileId
                      )
                      totalErrors += result.errors.length
                    } catch {
                      totalErrors += hostUUIDs.length
                    }
                  }

                  setApplyProgress(null)

                  // Show appropriate toast based on results
                  if (totalErrors > 0 && totalErrors < hostUUIDs.length) {
                    toast.error(t('notifications.partialFailed', { count: totalErrors }))
                  } else if (totalErrors >= hostUUIDs.length) {
                    toast.error(t('notifications.updateFailed'))
                  } else if (keysToApply.length > 0 || bulkSecurityHeaderProfile.apply) {
                    toast.success(t('notifications.updateSuccess'))
                  }

                  setSelectedHosts(new Set())
                  setShowBulkApplyModal(false)
                  setBulkSecurityHeaderProfile({ apply: false, profileId: null })
                }}
                disabled={
                  applyProgress !== null ||
                  (Object.values(bulkApplySettings).every(s => !s.apply) && !bulkSecurityHeaderProfile.apply)
                }
                isLoading={applyProgress !== null}
              >
                {t('proxyHosts.apply')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Bulk ACL Modal Dialog */}
        <Dialog open={showBulkACLModal} onOpenChange={setShowBulkACLModal}>
          <DialogContent className="max-w-md max-h-[80vh] overflow-hidden flex flex-col">
            <DialogHeader>
              <DialogTitle>{t('proxyHosts.applyACLTitle')}</DialogTitle>
              <DialogDescription>
                {t('proxyHosts.applyACLDescription', { count: selectedHosts.size }).split('<strong>').map((part, i) => {
                  if (i === 0) return part
                  const [strongText, rest] = part.split('</strong>')
                  return (
                    <span key={i}>
                      <strong className="text-brand-400">{strongText}</strong>
                      {rest}
                    </span>
                  )
                })}
              </DialogDescription>
            </DialogHeader>

            {/* Action Toggle */}
            <div className="flex gap-2 py-2">
              <Button
                variant={bulkACLAction === 'apply' ? 'primary' : 'secondary'}
                className="flex-1"
                onClick={() => {
                  setBulkACLAction('apply')
                  setSelectedACLs(new Set())
                }}
              >
                {t('proxyHosts.applyACL')}
              </Button>
              <Button
                variant={bulkACLAction === 'remove' ? 'danger' : 'secondary'}
                className="flex-1"
                onClick={() => {
                  setBulkACLAction('remove')
                  setSelectedACLs(new Set())
                }}
              >
                {t('proxyHosts.removeACL')}
              </Button>
            </div>

            {/* ACL Selection List */}
            {bulkACLAction === 'apply' && (
              <div className="flex-1 overflow-y-auto border border-border rounded-lg">
                {(accessLists?.filter((acl: AccessList) => acl.enabled).length ?? 0) > 0 && (
                  <div className="flex items-center justify-between p-2 border-b border-border bg-surface-subtle">
                    <span className="text-sm text-content-muted">
                      {t('proxyHosts.selectedACLCount', { selected: selectedACLs.size, total: accessLists?.filter((acl: AccessList) => acl.enabled).length ?? 0 })}
                    </span>
                    <div className="flex gap-2">
                      <button
                        onClick={() => {
                          const enabledACLs = accessLists?.filter((acl: AccessList) => acl.enabled) || []
                          setSelectedACLs(new Set(enabledACLs.map((acl: AccessList) => acl.id!)))
                        }}
                        className="text-xs text-brand-400 hover:text-brand-300"
                      >
                        {t('proxyHosts.selectAll')}
                      </button>
                      <span className="text-content-muted">|</span>
                      <button
                        onClick={() => setSelectedACLs(new Set())}
                        className="text-xs text-content-muted hover:text-content-primary"
                      >
                        {t('proxyHosts.clear')}
                      </button>
                    </div>
                  </div>
                )}
                <div className="p-2 space-y-1">
                  {accessLists?.filter((acl: AccessList) => acl.enabled).length === 0 ? (
                    <p className="text-content-muted text-sm p-2">{t('proxyHosts.noEnabledACLs')}</p>
                  ) : (
                    accessLists
                      ?.filter((acl: AccessList) => acl.enabled)
                      .map((acl: AccessList) => (
                        <label
                          key={acl.id}
                          className={`flex items-center gap-3 p-3 rounded-lg cursor-pointer transition-colors ${
                            selectedACLs.has(acl.id!)
                              ? 'bg-brand-500/10 border border-brand-500'
                              : 'bg-surface-subtle border border-transparent hover:bg-surface-muted'
                          }`}
                        >
                          <Checkbox
                            checked={selectedACLs.has(acl.id!)}
                            onCheckedChange={(checked) => {
                              const newSelected = new Set(selectedACLs)
                              if (checked) {
                                newSelected.add(acl.id!)
                              } else {
                                newSelected.delete(acl.id!)
                              }
                              setSelectedACLs(newSelected)
                            }}
                          />
                          <div className="flex-1">
                            <span className="text-content-primary font-medium">{acl.name}</span>
                            {acl.type && (
                              <span className="ml-2 text-xs text-content-muted">
                                ({acl.type.replace('_', ' ')})
                              </span>
                            )}
                          </div>
                        </label>
                      ))
                  )}
                </div>
              </div>
            )}

            {/* Remove ACL Confirmation */}
            {bulkACLAction === 'remove' && (
              <div className="flex-1 flex items-center justify-center border border-error/30 rounded-lg bg-error/5 p-6">
                <div className="text-center">
                  <div className="text-4xl mb-3">🚫</div>
                  <p className="text-content-secondary">
                    {t('proxyHosts.removeACLWarning', { count: selectedHosts.size })}
                  </p>
                  <p className="text-content-muted text-sm mt-2">
                    {t('proxyHosts.publicAccessWarning')}
                  </p>
                </div>
              </div>
            )}

            {/* Progress indicator */}
            {applyProgress && (
              <div className="border border-brand-500/30 rounded-lg bg-brand-500/10 p-4">
                <div className="flex items-center gap-3 mb-2">
                  <Loader2 className="w-5 h-5 animate-spin text-brand-400" />
                  <span className="text-brand-300 font-medium">
                    {t('proxyHosts.applyingACLs', { current: applyProgress.current, total: applyProgress.total })}
                  </span>
                </div>
                <div className="w-full bg-surface-muted rounded-full h-2">
                  <div
                    className="bg-brand-500 h-2 rounded-full transition-all duration-300"
                    style={{ width: `${(applyProgress.current / applyProgress.total) * 100}%` }}
                  />
                </div>
              </div>
            )}

            <DialogFooter>
              <Button
                variant="ghost"
                onClick={() => {
                  setShowBulkACLModal(false)
                  setSelectedACLs(new Set())
                  setBulkACLAction('apply')
                  setApplyProgress(null)
                }}
                disabled={isBulkUpdating || applyProgress !== null}
              >
                {t('common.cancel')}
              </Button>
              <Button
                variant={bulkACLAction === 'remove' ? 'danger' : 'primary'}
                onClick={async () => {
                  if (bulkACLAction === 'remove') {
                    await handleBulkApplyACL(null)
                  } else if (selectedACLs.size > 0) {
                    const hostUUIDs = Array.from(selectedHosts)
                    const aclIds = Array.from(selectedACLs)
                    const totalOperations = aclIds.length
                    let completedOperations = 0
                    let totalErrors = 0

                    setApplyProgress({ current: 0, total: totalOperations })

                    for (const aclId of aclIds) {
                      try {
                        const result = await bulkUpdateACL(hostUUIDs, aclId)
                        totalErrors += result.errors.length
                      } catch {
                        totalErrors += hostUUIDs.length
                      }
                      completedOperations++
                      setApplyProgress({ current: completedOperations, total: totalOperations })
                    }

                    setApplyProgress(null)

                    if (totalErrors > 0) {
                      toast.error(t('notifications.updateFailed'))
                    } else {
                      toast.success(t('notifications.updateSuccess'))
                    }

                    setSelectedHosts(new Set())
                    setSelectedACLs(new Set())
                    setShowBulkACLModal(false)
                  }
                }}
                disabled={isBulkUpdating || applyProgress !== null || (bulkACLAction === 'apply' && selectedACLs.size === 0)}
                isLoading={isBulkUpdating || applyProgress !== null}
              >
                {bulkACLAction === 'remove' ? t('proxyHosts.removeACL') : (selectedACLs.size > 0 ? t('proxyHosts.applyCount', { count: selectedACLs.size }) : t('proxyHosts.apply'))}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Bulk Delete Dialog */}
        <Dialog open={showBulkDeleteModal} onOpenChange={setShowBulkDeleteModal}>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <div className="flex items-start gap-3">
                <div className="shrink-0 w-10 h-10 rounded-full bg-error/10 flex items-center justify-center">
                  <AlertTriangle className="h-5 w-5 text-error" />
                </div>
                <div>
                  <DialogTitle>{t('proxyHosts.bulkDeleteTitle', { count: selectedHosts.size })}</DialogTitle>
                  <DialogDescription>
                    {t('proxyHosts.bulkDeleteDescription')}
                  </DialogDescription>
                </div>
              </div>
            </DialogHeader>

            <div className="bg-surface-subtle border border-border rounded-lg p-4 max-h-48 overflow-y-auto">
              <p className="text-xs font-medium text-content-muted uppercase mb-2">{t('proxyHosts.hostsToDelete')}</p>
              <ul className="space-y-1">
                {Array.from(selectedHosts).map((uuid) => {
                  const host = hosts.find(h => h.uuid === uuid)
                  return (
                    <li key={uuid} className="text-sm text-content-primary flex items-center gap-2">
                      <span className="text-error">•</span>
                      <span className="font-medium">{host?.name || t('proxyHosts.unnamed')}</span>
                      <span className="text-content-muted">({host?.domain_names})</span>
                    </li>
                  )
                })}
              </ul>
            </div>

            <Alert variant="info">
              {t('proxyHosts.backupInfo')}
            </Alert>

            <DialogFooter>
              <Button
                variant="ghost"
                onClick={() => setShowBulkDeleteModal(false)}
                disabled={isCreatingBackup}
              >
                {t('common.cancel')}
              </Button>
              <Button
                variant="danger"
                leftIcon={Trash2}
                onClick={handleBulkDelete}
                disabled={isCreatingBackup}
                isLoading={isCreatingBackup}
              >
                {isCreatingBackup ? t('proxyHosts.creatingBackup') : t('proxyHosts.deletePermanently')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Certificate Cleanup Dialog */}
        {showCertCleanupDialog && certCleanupData && (
          <CertificateCleanupDialog
            onConfirm={handleCertCleanupConfirm}
            onCancel={() => {
              setShowCertCleanupDialog(false)
              setCertCleanupData(null)
            }}
            certificates={certCleanupData.certificates}
            hostNames={certCleanupData.hostNames}
            isBulk={certCleanupData.isBulk}
          />
        )}

        {/* Proxy Group Form Dialog */}
        <ProxyGroupForm
          open={showGroupForm}
          onClose={() => {
            setShowGroupForm(false)
            setEditingGroup(undefined)
          }}
          group={editingGroup}
        />

        {/* Delete Group Confirmation Dialog */}
        <Dialog open={!!groupToDelete} onOpenChange={(open) => !open && setGroupToDelete(null)}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('proxyGroups.deleteGroup')}</DialogTitle>
              <DialogDescription>{t('proxyGroups.deleteGroupConfirm')}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setGroupToDelete(null)}>
                {t('common.cancel')}
              </Button>
              <Button
                variant="danger"
                onClick={async () => {
                  if (!groupToDelete) return
                  try {
                    await deleteGroup.mutateAsync(groupToDelete.uuid)
                  } catch {
                    // errors handled by hook toast
                  } finally {
                    setGroupToDelete(null)
                  }
                }}
                isLoading={deleteGroup.isPending}
              >
                {t('common.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Assign to Group Dialog */}
        <Dialog open={showAssignGroupModal} onOpenChange={(open) => !open && setShowAssignGroupModal(false)}>
          <DialogContent className="max-w-sm">
            <DialogHeader>
              <DialogTitle>{t('proxyGroups.assignToGroup')}</DialogTitle>
              <DialogDescription>
                {selectedHosts.size} host(s) selected
              </DialogDescription>
            </DialogHeader>
            <div className="py-2">
              <label htmlFor="assign-group-select" className="block text-sm font-medium text-content-primary mb-1.5">
                {t('proxyGroups.group')}
              </label>
              <select
                id="assign-group-select"
                value={assignTargetGroupUUID ?? ''}
                onChange={(e) => setAssignTargetGroupUUID(e.target.value || null)}
                className="w-full rounded-md border border-border bg-surface-primary text-content-primary px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              >
                <option value="">{t('proxyGroups.ungrouped')}</option>
                {groups.map((g) => (
                  <option key={g.uuid} value={g.uuid}>
                    {g.name}
                  </option>
                ))}
              </select>
            </div>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setShowAssignGroupModal(false)}>
                {t('common.cancel')}
              </Button>
              <Button
                onClick={async () => {
                  const targetGroup = assignTargetGroupUUID
                    ? groups.find((g) => g.uuid === assignTargetGroupUUID) ?? null
                    : null
                  const uuids = Array.from(selectedHosts)
                  try {
                    await Promise.all(
                      uuids.map((uuid) =>
                        updateHost(uuid, {
                          proxy_group_id: targetGroup ? targetGroup.uuid : null,
                        })
                      )
                    )
                    setShowAssignGroupModal(false)
                    setSelectedHosts(new Set())
                  } catch {
                    // errors handled individually
                  }
                }}
              >
                {t('common.save')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </PageShell>
    </>
  )
}
