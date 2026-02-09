import { useState } from 'react'
import { format } from 'date-fns'
import { Download, Filter, X } from 'lucide-react'
import { PageShell } from '../components/layout/PageShell'
import { useAuditLogs, type AuditLogFilters, type AuditLog } from '../hooks/useAuditLogs'
import { exportAuditLogsCSV } from '../api/auditLogs'
import { DataTable, type Column } from '../components/ui/DataTable'
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  Button,
  Badge,
  Input,
} from '../components/ui'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../components/ui/Dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../components/ui/Select'
import { toast } from '../utils/toast'

/** Audit log detail modal */
function AuditLogDetailModal({
  log,
  isOpen,
  onClose,
}: {
  log: AuditLog | null
  isOpen: boolean
  onClose: () => void
}) {
  if (!log) return null

  let parsedDetails: Record<string, unknown> = {}
  try {
    parsedDetails = JSON.parse(log.details)
  } catch {
    parsedDetails = { raw: log.details }
  }

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Audit Log Details</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-sm font-medium text-content-secondary">UUID</label>
              <p className="text-sm text-content-primary font-mono">{log.uuid}</p>
            </div>
            <div>
              <label className="text-sm font-medium text-content-secondary">Timestamp</label>
              <p className="text-sm text-content-primary">
                {format(new Date(log.created_at), 'PPpp')}
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-content-secondary">Actor</label>
              <p className="text-sm text-content-primary">{log.actor}</p>
            </div>
            <div>
              <label className="text-sm font-medium text-content-secondary">Action</label>
              <Badge variant="outline">{log.action}</Badge>
            </div>
            <div>
              <label className="text-sm font-medium text-content-secondary">Category</label>
              <Badge variant="primary">{log.event_category}</Badge>
            </div>
            {log.resource_uuid && (
              <div>
                <label className="text-sm font-medium text-content-secondary">Resource UUID</label>
                <p className="text-sm text-content-primary font-mono">{log.resource_uuid}</p>
              </div>
            )}
            {log.ip_address && (
              <div>
                <label className="text-sm font-medium text-content-secondary">IP Address</label>
                <p className="text-sm text-content-primary">{log.ip_address}</p>
              </div>
            )}
            {log.user_agent && (
              <div className="col-span-2">
                <label className="text-sm font-medium text-content-secondary">User Agent</label>
                <p className="text-sm text-content-primary break-all">{log.user_agent}</p>
              </div>
            )}
          </div>

          <div>
            <label className="text-sm font-medium text-content-secondary">Details</label>
            <pre className="mt-2 p-3 bg-surface-subtle rounded-lg text-xs overflow-auto max-h-96">
              {JSON.stringify(parsedDetails, null, 2)}
            </pre>
          </div>
        </div>
        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function AuditLogs() {
  const [page, setPage] = useState(1)
  const [limit] = useState(50)
  const [filters, setFilters] = useState<AuditLogFilters>({})
  const [showFilters, setShowFilters] = useState(false)
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null)
  const [isExporting, setIsExporting] = useState(false)

  const { data, isLoading } = useAuditLogs(filters, page, limit)

  const handleFilterChange = (key: keyof AuditLogFilters, value: string) => {
    setFilters((prev) => ({
      ...prev,
      [key]: value || undefined,
    }))
    setPage(1) // Reset to first page when filters change
  }

  const handleClearFilters = () => {
    setFilters({})
    setPage(1)
  }

  const handleExport = async () => {
    setIsExporting(true)
    try {
      const csv = await exportAuditLogsCSV(filters)
      const blob = new Blob([csv], { type: 'text/csv' })
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `audit-logs-${format(new Date(), 'yyyy-MM-dd-HHmmss')}.csv`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      window.URL.revokeObjectURL(url)
      toast.success('Audit logs exported successfully')
    } catch (error) {
      toast.error('Failed to export audit logs')
      console.error('Export error:', error)
    } finally {
      setIsExporting(false)
    }
  }

  const columns: Column<AuditLog>[] = [
    {
      key: 'created_at',
      header: 'Timestamp',
      sortable: true,
      width: '200px',
      cell: (log) => (
        <span className="text-sm">
          {format(new Date(log.created_at), 'MMM d, yyyy HH:mm:ss')}
        </span>
      ),
    },
    {
      key: 'actor',
      header: 'Actor',
      sortable: true,
      cell: (log) => <span className="text-sm font-medium">{log.actor}</span>,
    },
    {
      key: 'action',
      header: 'Action',
      sortable: true,
      cell: (log) => <Badge variant="outline">{log.action}</Badge>,
    },
    {
      key: 'event_category',
      header: 'Category',
      sortable: true,
      cell: (log) => <Badge variant="primary">{log.event_category}</Badge>,
    },
    {
      key: 'resource_uuid',
      header: 'Resource',
      cell: (log) =>
        log.resource_uuid ? (
          <span className="text-sm font-mono text-content-muted">{log.resource_uuid.slice(0, 8)}...</span>
        ) : (
          <span className="text-sm text-content-muted">—</span>
        ),
    },
    {
      key: 'ip_address',
      header: 'IP Address',
      cell: (log) =>
        log.ip_address ? (
          <span className="text-sm">{log.ip_address}</span>
        ) : (
          <span className="text-sm text-content-muted">—</span>
        ),
    },
  ]

  const hasActiveFilters = Object.values(filters).some((v) => v !== undefined && v !== '')

  const headerActions = (
    <div className="flex items-center gap-2">
      <Button
        variant="secondary"
        onClick={() => setShowFilters(!showFilters)}
        className="relative"
      >
        <Filter className="w-4 h-4 mr-2" />
        Filters
        {hasActiveFilters && (
          <span className="absolute -top-1 -right-1 h-4 w-4 bg-brand-500 rounded-full text-[10px] text-white flex items-center justify-center">
            {Object.values(filters).filter((v) => v).length}
          </span>
        )}
      </Button>
      <Button
        variant="secondary"
        onClick={handleExport}
        disabled={isExporting || !data?.logs.length}
      >
        <Download className="w-4 h-4 mr-2" />
        Export CSV
      </Button>
    </div>
  )

  return (
    <PageShell
      title="Audit Logs"
      description="View and filter security audit events"
      actions={headerActions}
    >
      {/* Filters Card */}
      {showFilters && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>Filters</CardTitle>
              <Button variant="ghost" size="sm" onClick={handleClearFilters}>
                <X className="w-4 h-4 mr-1" />
                Clear All
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <label className="block text-sm font-medium mb-2">Category</label>
                <Select
                  value={filters.event_category || 'all'}
                  onValueChange={(value) => handleFilterChange('event_category', value === 'all' ? '' : value)}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="All categories" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All categories</SelectItem>
                    <SelectItem value="dns_provider">DNS Provider</SelectItem>
                    <SelectItem value="certificate">Certificate</SelectItem>
                    <SelectItem value="proxy_host">Proxy Host</SelectItem>
                    <SelectItem value="user">User</SelectItem>
                    <SelectItem value="system">System</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Actor</label>
                <Input
                  type="text"
                  placeholder="Filter by actor..."
                  value={filters.actor || ''}
                  onChange={(e) => handleFilterChange('actor', e.target.value)}
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Action</label>
                <Input
                  type="text"
                  placeholder="Filter by action..."
                  value={filters.action || ''}
                  onChange={(e) => handleFilterChange('action', e.target.value)}
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Start Date</label>
                <Input
                  type="datetime-local"
                  value={filters.start_date || ''}
                  onChange={(e) => handleFilterChange('start_date', e.target.value)}
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">End Date</label>
                <Input
                  type="datetime-local"
                  value={filters.end_date || ''}
                  onChange={(e) => handleFilterChange('end_date', e.target.value)}
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Resource UUID</label>
                <Input
                  type="text"
                  placeholder="Filter by resource..."
                  value={filters.resource_uuid || ''}
                  onChange={(e) => handleFilterChange('resource_uuid', e.target.value)}
                />
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Audit Logs Table */}
      <Card>
        <CardContent className="p-0">
          <DataTable
            data={data?.logs || []}
            columns={columns}
            rowKey={(log) => log.uuid}
            isLoading={isLoading}
            onRowClick={(log) => setSelectedLog(log)}
            emptyState={
              <div className="text-center py-12">
                <p className="text-content-muted">No audit logs found</p>
                {hasActiveFilters && (
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handleClearFilters}
                    className="mt-4"
                  >
                    Clear Filters
                  </Button>
                )}
              </div>
            }
          />

          {/* Pagination */}
          {data && data.total > 0 && (
            <div className="flex items-center justify-between px-6 py-4 border-t border-border">
              <div className="text-sm text-content-secondary">
                Showing {(page - 1) * limit + 1} to {Math.min(page * limit, data.total)} of {data.total} entries
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page === 1}
                >
                  Previous
                </Button>
                <span className="text-sm text-content-secondary">
                  Page {page} of {Math.ceil(data.total / limit)}
                </span>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setPage((p) => p + 1)}
                  disabled={page * limit >= data.total}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Detail Modal */}
      <AuditLogDetailModal
        log={selectedLog}
        isOpen={!!selectedLog}
        onClose={() => setSelectedLog(null)}
      />
    </PageShell>
  )
}
