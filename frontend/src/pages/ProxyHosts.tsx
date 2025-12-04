import { useState, useMemo } from 'react'
import { Loader2, ExternalLink, AlertTriangle, ChevronUp, ChevronDown, CheckSquare, Square, Trash2 } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useProxyHosts } from '../hooks/useProxyHosts'
import { getMonitors, type UptimeMonitor } from '../api/uptime'
import { useCertificates } from '../hooks/useCertificates'
import { useAccessLists } from '../hooks/useAccessLists'
import { getSettings } from '../api/settings'
import { createBackup } from '../api/backups'
import type { ProxyHost } from '../api/proxyHosts'
import compareHosts from '../utils/compareHosts'
import type { AccessList } from '../api/accessLists'
import ProxyHostForm from '../components/ProxyHostForm'
import { Switch } from '../components/ui/Switch'
import { toast } from 'react-hot-toast'
import { formatSettingLabel, settingHelpText, applyBulkSettingsToHosts } from '../utils/proxyHostsHelpers'
import { ConfigReloadOverlay } from '../components/LoadingStates'

// Helper functions extracted for unit testing and reuse
// Helpers moved to ../utils/proxyHostsHelpers to keep component files component-only for fast refresh

type SortColumn = 'name' | 'domain' | 'forward'
type SortDirection = 'asc' | 'desc'

export default function ProxyHosts() {
  const { hosts, loading, isFetching, error, createHost, updateHost, deleteHost, bulkUpdateACL, isBulkUpdating, isCreating, isUpdating, isDeleting } = useProxyHosts()
  const { certificates } = useCertificates()
  const { data: accessLists } = useAccessLists()
  const [showForm, setShowForm] = useState(false)
  const [editingHost, setEditingHost] = useState<ProxyHost | undefined>()
  const [sortColumn, setSortColumn] = useState<SortColumn>('name')
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc')
  const [selectedHosts, setSelectedHosts] = useState<Set<string>>(new Set())
  const [showBulkACLModal, setShowBulkACLModal] = useState(false)
  const [showBulkApplyModal, setShowBulkApplyModal] = useState(false)
  const [showBulkDeleteModal, setShowBulkDeleteModal] = useState(false)
  const [isCreatingBackup, setIsCreatingBackup] = useState(false)
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
  })

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
  })

  const linkBehavior = settings?.['ui.domain_link_behavior'] || 'new_tab'

  // Determine if any mutation is in progress
  const isApplyingConfig = isCreating || isUpdating || isDeleting || isBulkUpdating

  // Determine contextual message based on operation
  const getMessage = () => {
    if (isCreating) return { message: 'Ferrying new host...', submessage: 'Charon is crossing the Styx' }
    if (isUpdating) return { message: 'Guiding changes across...', submessage: 'Configuration in transit' }
    if (isDeleting) return { message: 'Returning to shore...', submessage: 'Host departure in progress' }
    if (isBulkUpdating) return { message: `Ferrying ${selectedHosts.size} souls...`, submessage: 'Bulk operation crossing the river' }
    return { message: 'Ferrying configuration...', submessage: 'Charon is crossing the Styx' }
  }

  const { message, submessage } = getMessage()

  // Create a map of domain -> certificate status for quick lookup
  // Handles both single domains and comma-separated multi-domain certs
  const certStatusByDomain = useMemo(() => {
    const map: Record<string, { status: string; provider: string }> = {}
    certificates.forEach(cert => {
      // Handle comma-separated domains (SANs)
      const domains = cert.domain.split(',').map(d => d.trim().toLowerCase())
      domains.forEach(domain => {
        // Only set if not already set (first cert wins)
        if (!map[domain]) {
          map[domain] = { status: cert.status, provider: cert.provider }
        }
      })
    })
    return map
  }, [certificates])

  // Sort hosts based on current sort column and direction
  const sortedHosts = useMemo(() => [...hosts].sort((a, b) => compareHosts(a, b, sortColumn, sortDirection)), [hosts, sortColumn, sortDirection])

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

  const handleDomainClick = (e: React.MouseEvent, url: string) => {
    if (linkBehavior === 'new_window') {
      e.preventDefault()
      window.open(url, '_blank', 'noopener,noreferrer,width=1024,height=768')
    }
  }



  // local usage now relies on the exported settingHelpText helper

  // local usage now relies on exported settingKeyToField helper

  const handleAdd = () => {
    setEditingHost(undefined)
    setShowForm(true)
  }

  const handleEdit = (host: ProxyHost) => {
    setEditingHost(host)
    setShowForm(true)
  }

  const handleSubmit = async (data: Partial<ProxyHost>) => {
    if (editingHost) {
      await updateHost(editingHost.uuid, data)
    } else {
      await createHost(data)
    }
    setShowForm(false)
    setEditingHost(undefined)
  }

  const handleDelete = async (uuid: string) => {
    const host = hosts.find(h => h.uuid === uuid)
    if (!host) return

    if (!confirm('Are you sure you want to delete this proxy host?')) return

    try {
      // See if there are uptime monitors associated with this host (match by upstream_host / forward_host)
      let associatedMonitors: UptimeMonitor[] = []
      try {
        const monitors = await getMonitors()
        associatedMonitors = monitors.filter(m => m.upstream_host === host.forward_host || (m.proxy_host_id && m.proxy_host_id === (host as unknown as { id?: number }).id))
      } catch {
        // ignore errors fetching uptime data; continue with host deletion
      }

      if (associatedMonitors.length > 0) {
        const deleteUptime = confirm('This proxy host has uptime monitors associated with it. Delete the monitors as well?')
        await deleteHost(uuid, deleteUptime)
      } else {
        await deleteHost(uuid)
      }
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to delete')
    }
  }

  const toggleHostSelection = (uuid: string) => {
    setSelectedHosts(prev => {
      const next = new Set(prev)
      if (next.has(uuid)) {
        next.delete(uuid)
      } else {
        next.add(uuid)
      }
      return next
    })
  }

  const toggleSelectAll = () => {
    if (selectedHosts.size === hosts.length) {
      setSelectedHosts(new Set())
    } else {
      setSelectedHosts(new Set(hosts.map(h => h.uuid)))
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

      // Delete each host
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

  return (
    <>
      {isApplyingConfig && (
        <ConfigReloadOverlay
          message={message}
          submessage={submessage}
          type="charon"
        />
      )}
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <h1 className="text-3xl font-bold text-white">Proxy Hosts</h1>
          {isFetching && !loading && <Loader2 className="animate-spin text-blue-400" size={24} />}
        </div>
        <div className="flex gap-3">
          {selectedHosts.size > 0 && (
            <div className="flex items-center gap-2">
              <span className="text-gray-400 text-sm">
                {selectedHosts.size} {selectedHosts.size === hosts.length && '(all)'} selected
              </span>
              <button
                onClick={() => setShowBulkApplyModal(true)}
                className="px-4 py-2 bg-indigo-700 hover:bg-indigo-600 text-white rounded-lg font-medium transition-colors"
              >
                Bulk Apply
              </button>
              <button
                onClick={() => setShowBulkACLModal(true)}
                className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors"
                disabled={isBulkUpdating}
              >
                {isBulkUpdating ? 'Updating...' : 'Manage ACL'}
              </button>
              <button
                onClick={() => setShowBulkDeleteModal(true)}
                className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg font-medium transition-colors flex items-center gap-2"
              >
                <Trash2 size={16} />
                Delete
              </button>
            </div>
          )}
          <button
            onClick={handleAdd}
            className="px-4 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors"
          >
            Add Proxy Host
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded mb-6">
          {error}
        </div>
      )}

      {/* Bulk Apply Modal */}
      {showBulkApplyModal && (
        <div
          className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
          onClick={() => setShowBulkApplyModal(false)}
        >
          <div
            className="bg-dark-card border border-gray-800 rounded-lg p-6 max-w-md w-full mx-4 max-h-[80vh] overflow-hidden flex flex-col"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="text-xl font-bold text-white mb-4">Bulk Apply Settings</h2>
            <div className="space-y-4 flex-1 overflow-hidden flex flex-col">
              <p className="text-sm text-gray-400">
                Applying settings to <span className="text-blue-400 font-medium">{selectedHosts.size}</span> selected host(s)
              </p>

              <div className="flex-1 overflow-y-auto border border-gray-700 rounded-lg p-3 space-y-3">
                {Object.entries(bulkApplySettings).map(([key, cfg]) => (
                  <div key={key} className="flex items-center justify-between gap-3 p-2 bg-gray-900/30 rounded">
                    <div>
                      <div className="flex items-center gap-2">
                        <input
                          type="checkbox"
                          checked={cfg.apply}
                          onChange={(e) => setBulkApplySettings(prev => ({ ...prev, [key]: { ...prev[key], apply: e.target.checked } }))}
                          className="w-4 h-4 rounded border-gray-600 text-blue-500 bg-gray-700"
                        />
                        <div>
                          <div className="text-white font-medium">{formatSettingLabel(key)}</div>
                          <div className="text-xs text-gray-400">{settingHelpText(key)}</div>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className="text-xs text-gray-400">Set:</span>
                      <Switch
                        checked={cfg.value}
                        onCheckedChange={(v: boolean) => setBulkApplySettings(prev => ({ ...prev, [key]: { ...prev[key], value: v } }))}
                      />
                    </div>
                  </div>
                ))}
              </div>

              {applyProgress && (
                <div className="border border-blue-800/50 rounded-lg bg-blue-900/20 p-4">
                  <div className="flex items-center gap-3 mb-2">
                    <Loader2 className="w-5 h-5 animate-spin text-blue-400" />
                    <span className="text-blue-300 font-medium">
                      Applying settings... ({applyProgress.current}/{applyProgress.total})
                    </span>
                  </div>
                  <div className="w-full bg-gray-700 rounded-full h-2">
                    <div
                      className="bg-blue-500 h-2 rounded-full transition-all duration-300"
                      style={{ width: `${(applyProgress.current / applyProgress.total) * 100}%` }}
                    />
                  </div>
                </div>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  onClick={() => {
                    setShowBulkApplyModal(false)
                  }}
                  className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors"
                  disabled={applyProgress !== null}
                >
                  Cancel
                </button>
                <button
                  onClick={async () => {
                    const keysToApply = Object.keys(bulkApplySettings).filter(k => bulkApplySettings[k].apply)
                    if (keysToApply.length === 0) return

                    const hostUUIDs = Array.from(selectedHosts)
                    const result = await applyBulkSettingsToHosts({ hosts, hostUUIDs, keysToApply, bulkApplySettings, updateHost, setApplyProgress })

                    if (result.errors > 0) {
                      toast.error(`Applied settings with ${result.errors} error(s)`)
                    } else {
                      toast.success(`Applied settings to ${hostUUIDs.length} host(s)`)
                    }

                    setSelectedHosts(new Set())
                    setShowBulkApplyModal(false)
                  }}
                  disabled={applyProgress !== null || Object.values(bulkApplySettings).every(s => !s.apply)}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {(applyProgress !== null) && <Loader2 className="w-4 h-4 animate-spin mr-2" />}
                  Apply
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      <div className="bg-dark-card rounded-lg border border-gray-800 overflow-hidden">
        {loading ? (
          <div className="text-center text-gray-400 py-12">Loading...</div>
        ) : hosts.length === 0 ? (
          <div className="text-center text-gray-400 py-12">
            No proxy hosts configured yet. Click "Add Proxy Host" to get started.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full table-fixed min-w-0">
              <thead className="bg-gray-900 border-b border-gray-800">
                <tr>
                  <th
                    onClick={() => handleSort('name')}
                    style={{ width: '20%' }}
                    className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-200 transition-colors"
                  >
                    <div className="flex items-center gap-1">
                      Name
                      <SortIcon column="name" />
                    </div>
                  </th>
                  <th
                    onClick={() => handleSort('domain')}
                    style={{ width: '26%' }}
                    className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-200 transition-colors"
                  >
                    <div className="flex items-center gap-1">
                      Domain
                      <SortIcon column="domain" />
                    </div>
                  </th>
                  <th
                    onClick={() => handleSort('forward')}
                    style={{ width: '18%' }}
                    className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-200 transition-colors"
                  >
                    <div className="flex items-center gap-1">
                      Forward To
                      <SortIcon column="forward" />
                    </div>
                  </th>
                  <th style={{ width: '8%' }} className="px-6 py-3 text-center text-xs font-medium text-gray-400 uppercase tracking-wider">
                    SSL
                  </th>
                  <th style={{ width: '10%' }} className="px-6 py-3 text-center text-xs font-medium text-gray-400 uppercase tracking-wider">
                    Status
                  </th>
                  <th style={{ width: '12%' }} className="px-6 py-3 text-right text-xs font-medium text-gray-400 uppercase tracking-wider">
                    Actions
                  </th>
                  <th style={{ width: '6%' }} className="px-6 py-3 text-center text-xs font-medium text-gray-400 uppercase tracking-wider">
                    <button
                      onClick={toggleSelectAll}
                      role="checkbox"
                      aria-checked={selectedHosts.size === hosts.length}
                      className="text-gray-400 hover:text-white transition-colors"
                      title={selectedHosts.size === hosts.length ? 'Deselect all' : 'Select all'}
                    >
                      {selectedHosts.size === hosts.length ? (
                        <CheckSquare size={18} />
                      ) : (
                        <Square size={18} />
                      )}
                    </button>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800">
                {sortedHosts.map((host) => (
                  <tr key={host.uuid} className="hover:bg-gray-900/50">
                    <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm font-medium text-white max-w-full truncate">
                          {host.name || <span className="text-gray-500 italic">Unnamed</span>}
                        </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm font-medium text-white">
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
                                  className="hover:text-blue-400 hover:underline flex items-center gap-1 truncate block max-w-full"
                                  style={{ maxWidth: '100%' }}
                                >
                                  <span className="truncate block max-w-[40ch]">{d}</span>
                                  <ExternalLink size={12} className="opacity-50" />
                                </a>
                              </div>
                            )
                          })}
                        </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-gray-300">
                        {host.forward_scheme}://{host.forward_host}:{host.forward_port}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-center">
                      {(() => {
                        // Get the primary domain to look up cert status (case-insensitive)
                        const primaryDomain = host.domain_names.split(',')[0]?.trim().toLowerCase()
                        const certInfo = certStatusByDomain[primaryDomain]
                        const isUntrusted = certInfo?.status === 'untrusted'
                        const isStaging = certInfo?.provider?.includes('staging')

                        return (
                          <div className="flex flex-col gap-2">
                            {/* Row 1: Proxy Badges */}
                            <div className="flex flex-wrap justify-center gap-2">
                              {host.ssl_forced && (
                                isUntrusted || isStaging ? (
                                  <span className="px-2 py-1 text-xs bg-orange-900/30 text-orange-400 rounded flex items-center gap-1">
                                    <AlertTriangle size={12} />
                                    SSL (Staging)
                                  </span>
                                ) : (
                                  <span className="px-2 py-1 text-xs bg-green-900/30 text-green-400 rounded">
                                    SSL
                                  </span>
                                )
                              )}
                              {host.websocket_support && (
                                <span className="px-2 py-1 text-xs bg-blue-900/30 text-blue-400 rounded">
                                  WS
                                </span>
                              )}
                            </div>
                            {/* Row 2: Security Badges */}
                            {host.access_list_id && (
                              <div className="flex flex-wrap justify-center gap-2">
                                <span className="px-2 py-1 text-xs bg-purple-900/30 text-purple-400 rounded">
                                  ACL
                                </span>
                              </div>
                            )}
                            {/* Certificate info below badges */}
                            {host.certificate && host.certificate.provider === 'custom' && (
                              <div className="text-xs text-gray-400">
                                {host.certificate.name} (Custom)
                              </div>
                            )}
                            {host.ssl_forced && !host.certificate && (isUntrusted || isStaging) && (
                              <div className="text-xs text-orange-400">
                                ⚠️ Staging cert - browsers won't trust
                              </div>
                            )}
                          </div>
                        )
                      })()}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-center">
                      <Switch
                        checked={host.enabled}
                        onCheckedChange={(checked) => updateHost(host.uuid, { enabled: checked })}
                      />
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <button
                        onClick={() => handleEdit(host)}
                        className="text-blue-400 hover:text-blue-300 mr-4"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(host.uuid)}
                        className="text-red-400 hover:text-red-300"
                      >
                        Delete
                      </button>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-center">
                      <button
                        onClick={() => toggleHostSelection(host.uuid)}
                        role="checkbox"
                        aria-checked={selectedHosts.has(host.uuid)}
                        aria-label={`Select ${host.name}`}
                        className="text-gray-400 hover:text-white transition-colors"
                      >
                        {selectedHosts.has(host.uuid) ? (
                          <CheckSquare size={18} className="text-blue-400" />
                        ) : (
                          <Square size={18} />
                        )}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

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

      {/* Bulk ACL Modal */}
      {showBulkACLModal && (
        <div
          className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
          onClick={() => setShowBulkACLModal(false)}
        >
          <div
            className="bg-dark-card border border-gray-800 rounded-lg p-6 max-w-md w-full mx-4 max-h-[80vh] overflow-hidden flex flex-col"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="text-xl font-bold text-white mb-4">Apply Access List</h2>
            <div className="space-y-4 flex-1 overflow-hidden flex flex-col">
              <p className="text-sm text-gray-400">
                Applying to <span className="text-blue-400 font-medium">{selectedHosts.size}</span> selected host(s)
              </p>
              <p className="text-xs text-gray-500 mt-1">
                Note: Each proxy host can have a single Access Control List applied. Selecting multiple lists will apply them sequentially and the last applied list will be the effective one for each host.
              </p>

              {/* Action Toggle */}
              <div className="flex gap-2">
                <button
                  onClick={() => {
                    setBulkACLAction('apply')
                    setSelectedACLs(new Set())
                  }}
                  className={`flex-1 px-3 py-2 rounded-lg font-medium transition-colors ${
                    bulkACLAction === 'apply'
                      ? 'bg-blue-600 text-white'
                      : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
                  }`}
                >
                  Apply ACL
                </button>
                <button
                  onClick={() => {
                    setBulkACLAction('remove')
                    setSelectedACLs(new Set())
                  }}
                  className={`flex-1 px-3 py-2 rounded-lg font-medium transition-colors ${
                    bulkACLAction === 'remove'
                      ? 'bg-red-600 text-white'
                      : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
                  }`}
                >
                  Remove ACL
                </button>
              </div>

              {/* ACL Selection List */}
              {bulkACLAction === 'apply' && (
                <div className="flex-1 overflow-y-auto border border-gray-700 rounded-lg">
                  {/* Select All / Clear header */}
                  {(accessLists?.filter((acl: AccessList) => acl.enabled).length ?? 0) > 0 && (
                    <div className="flex items-center justify-between p-2 border-b border-gray-700 bg-gray-800/50">
                      <span className="text-sm text-gray-400">
                        {selectedACLs.size} of {accessLists?.filter((acl: AccessList) => acl.enabled).length ?? 0} selected
                      </span>
                      <div className="flex gap-2">
                        <button
                          onClick={() => {
                            const enabledACLs = accessLists?.filter((acl: AccessList) => acl.enabled) || []
                            setSelectedACLs(new Set(enabledACLs.map((acl: AccessList) => acl.id!)))
                          }}
                          className="text-xs text-blue-400 hover:text-blue-300"
                        >
                          Select All
                        </button>
                        <span className="text-gray-600">|</span>
                        <button
                          onClick={() => setSelectedACLs(new Set())}
                          className="text-xs text-gray-400 hover:text-gray-300"
                        >
                          Clear
                        </button>
                      </div>
                    </div>
                  )}
                  <div className="p-2 space-y-1">
                    {accessLists?.filter((acl: AccessList) => acl.enabled).length === 0 ? (
                      <p className="text-gray-500 text-sm p-2">No enabled access lists available</p>
                    ) : (
                      accessLists
                        ?.filter((acl: AccessList) => acl.enabled)
                        .map((acl: AccessList) => (
                          <label
                            key={acl.id}
                            className={`flex items-center gap-3 p-3 rounded-lg cursor-pointer transition-colors ${
                              selectedACLs.has(acl.id!)
                                ? 'bg-blue-600/20 border border-blue-500'
                                : 'bg-gray-800/50 border border-transparent hover:bg-gray-800'
                            }`}
                          >
                            <input
                              type="checkbox"
                              checked={selectedACLs.has(acl.id!)}
                              onChange={(e) => {
                                const newSelected = new Set(selectedACLs)
                                if (e.target.checked) {
                                  newSelected.add(acl.id!)
                                } else {
                                  newSelected.delete(acl.id!)
                                }
                                setSelectedACLs(newSelected)
                              }}
                              className="w-4 h-4 rounded border-gray-600 text-blue-500 focus:ring-blue-500 focus:ring-offset-0 bg-gray-700"
                            />
                            <div className="flex-1">
                              <span className="text-white font-medium">{acl.name}</span>
                              {acl.type && (
                                <span className="ml-2 text-xs text-gray-500">
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
                <div className="flex-1 flex items-center justify-center border border-red-900/50 rounded-lg bg-red-900/10 p-6">
                  <div className="text-center">
                    <div className="text-4xl mb-3">🚫</div>
                    <p className="text-gray-300">
                      This will remove the access list from all {selectedHosts.size} selected host(s).
                    </p>
                    <p className="text-gray-500 text-sm mt-2">
                      The hosts will become publicly accessible.
                    </p>
                  </div>
                </div>
              )}

              {/* Progress indicator */}
              {applyProgress && (
                <div className="border border-blue-800/50 rounded-lg bg-blue-900/20 p-4">
                  <div className="flex items-center gap-3 mb-2">
                    <Loader2 className="w-5 h-5 animate-spin text-blue-400" />
                    <span className="text-blue-300 font-medium">
                      Applying ACLs... ({applyProgress.current}/{applyProgress.total})
                    </span>
                  </div>
                  <div className="w-full bg-gray-700 rounded-full h-2">
                    <div
                      className="bg-blue-500 h-2 rounded-full transition-all duration-300"
                      style={{ width: `${(applyProgress.current / applyProgress.total) * 100}%` }}
                    />
                  </div>
                </div>
              )}

              {/* Action Buttons */}
              <div className="flex justify-end gap-2 pt-2">
                <button
                  onClick={() => {
                    setShowBulkACLModal(false)
                    setSelectedACLs(new Set())
                    setBulkACLAction('apply')
                    setApplyProgress(null)
                  }}
                  className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors"
                  disabled={isBulkUpdating || applyProgress !== null}
                >
                  Cancel
                </button>
                <button
                  onClick={async () => {
                    if (bulkACLAction === 'remove') {
                      await handleBulkApplyACL(null)
                    } else if (selectedACLs.size > 0) {
                      // Apply each selected ACL sequentially with progress
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
                        toast.error(`Applied ${selectedACLs.size} ACL(s) with some errors`)
                      } else {
                        toast.success(`Applied ${selectedACLs.size} ACL(s) to ${selectedHosts.size} host(s)`)
                      }

                      setSelectedHosts(new Set())
                      setSelectedACLs(new Set())
                      setShowBulkACLModal(false)
                    }
                  }}
                  disabled={isBulkUpdating || applyProgress !== null || (bulkACLAction === 'apply' && selectedACLs.size === 0)}
                  className={`px-4 py-2 rounded-lg font-medium transition-colors flex items-center gap-2 ${
                    bulkACLAction === 'remove'
                      ? 'bg-red-600 hover:bg-red-500 text-white'
                      : 'bg-blue-600 hover:bg-blue-500 text-white'
                  } disabled:opacity-50 disabled:cursor-not-allowed`}
                >
                  {(isBulkUpdating || applyProgress !== null) && <Loader2 className="w-4 h-4 animate-spin" />}
                  {bulkACLAction === 'remove' ? 'Remove ACL' : `Apply ${selectedACLs.size > 0 ? `(${selectedACLs.size})` : ''}`}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Bulk Delete Modal */}
      {showBulkDeleteModal && (
        <div
          className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
          onClick={() => setShowBulkDeleteModal(false)}
        >
          <div
            className="bg-dark-card border border-red-900/50 rounded-lg p-6 max-w-lg w-full mx-4"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start gap-3 mb-4">
              <div className="flex-shrink-0 w-10 h-10 rounded-full bg-red-900/30 flex items-center justify-center">
                <AlertTriangle className="h-5 w-5 text-red-400" />
              </div>
              <div className="flex-1">
                <h2 className="text-xl font-bold text-white">Delete {selectedHosts.size} Proxy Host{selectedHosts.size > 1 ? 's' : ''}?</h2>
                <p className="text-sm text-gray-400 mt-1">
                  This action cannot be undone. A backup will be created automatically before deletion.
                </p>
              </div>
            </div>

            <div className="bg-gray-900/50 border border-gray-800 rounded-lg p-4 mb-4 max-h-48 overflow-y-auto">
              <p className="text-xs font-medium text-gray-400 uppercase mb-2">Hosts to be deleted:</p>
              <ul className="space-y-1">
                {Array.from(selectedHosts).map((uuid) => {
                  const host = hosts.find(h => h.uuid === uuid)
                  return (
                    <li key={uuid} className="text-sm text-white flex items-center gap-2">
                      <span className="text-red-400">•</span>
                      <span className="font-medium">{host?.name || 'Unnamed'}</span>
                      <span className="text-gray-500">({host?.domain_names})</span>
                    </li>
                  )
                })}
              </ul>
            </div>

            <div className="bg-blue-900/20 border border-blue-800/50 rounded-lg p-3 mb-4">
              <p className="text-xs text-blue-300 flex items-start gap-2">
                <span className="text-blue-400">ℹ️</span>
                <span>An automatic backup will be created before deletion. You can restore from the Backups page if needed.</span>
              </p>
            </div>

            <div className="flex justify-end gap-2">
              <button
                onClick={() => setShowBulkDeleteModal(false)}
                className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors"
                disabled={isCreatingBackup}
              >
                Cancel
              </button>
              <button
                onClick={handleBulkDelete}
                className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg font-medium transition-colors flex items-center gap-2"
                disabled={isCreatingBackup}
              >
                {isCreatingBackup ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Creating Backup...
                  </>
                ) : (
                  <>
                    <Trash2 size={16} />
                    Delete Permanently
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
      </div>
    </>
  )
}
