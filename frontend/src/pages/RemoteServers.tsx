import { useState } from 'react'
import { Plus, Pencil, Trash2, Server, LayoutGrid, LayoutList } from 'lucide-react'
import { useRemoteServers } from '../hooks/useRemoteServers'
import type { RemoteServer } from '../api/remoteServers'
import RemoteServerForm from '../components/RemoteServerForm'
import { PageShell } from '../components/layout/PageShell'
import {
  Badge,
  Button,
  Alert,
  DataTable,
  EmptyState,
  SkeletonTable,
  SkeletonCard,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Card,
  type Column,
} from '../components/ui'

export default function RemoteServers() {
  const { servers, loading, error, createServer, updateServer, deleteServer } = useRemoteServers()
  const [showForm, setShowForm] = useState(false)
  const [editingServer, setEditingServer] = useState<RemoteServer | undefined>()
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid')
  const [deleteConfirm, setDeleteConfirm] = useState<RemoteServer | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  const handleAdd = () => {
    setEditingServer(undefined)
    setShowForm(true)
  }

  const handleEdit = (server: RemoteServer) => {
    setEditingServer(server)
    setShowForm(true)
  }

  const handleSubmit = async (data: Partial<RemoteServer>) => {
    if (editingServer) {
      await updateServer(editingServer.uuid, data)
    } else {
      await createServer(data)
    }
    setShowForm(false)
    setEditingServer(undefined)
  }

  const handleDelete = async (server: RemoteServer) => {
    setIsDeleting(true)
    try {
      await deleteServer(server.uuid)
      setDeleteConfirm(null)
    } finally {
      setIsDeleting(false)
    }
  }

  const columns: Column<RemoteServer>[] = [
    {
      key: 'name',
      header: 'Name',
      sortable: true,
      cell: (server) => (
        <span className="font-medium text-content-primary">{server.name}</span>
      ),
    },
    {
      key: 'provider',
      header: 'Provider',
      sortable: true,
      cell: (server) => (
        <Badge variant="outline" size="sm">{server.provider}</Badge>
      ),
    },
    {
      key: 'host',
      header: 'Host',
      cell: (server) => (
        <span className="font-mono text-sm text-content-secondary">{server.host}</span>
      ),
    },
    {
      key: 'port',
      header: 'Port',
      cell: (server) => (
        <span className="font-mono text-sm text-content-secondary">{server.port}</span>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      sortable: true,
      cell: (server) => (
        <Badge variant={server.enabled ? 'success' : 'default'} size="sm">
          {server.enabled ? 'Enabled' : 'Disabled'}
        </Badge>
      ),
    },
    {
      key: 'actions',
      header: 'Actions',
      cell: (server) => (
        <div className="flex items-center justify-end gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={(e) => {
              e.stopPropagation()
              handleEdit(server)
            }}
            title="Edit"
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={(e) => {
              e.stopPropagation()
              setDeleteConfirm(server)
            }}
            title="Delete"
          >
            <Trash2 className="h-4 w-4 text-error" />
          </Button>
        </div>
      ),
    },
  ]

  // Header actions
  const headerActions = (
    <div className="flex items-center gap-2">
      <div className="flex bg-surface-muted rounded-lg p-1">
        <button
          onClick={() => setViewMode('grid')}
          className={`p-2 rounded transition-colors ${
            viewMode === 'grid'
              ? 'bg-brand-500 text-white'
              : 'text-content-muted hover:text-content-primary'
          }`}
          title="Grid view"
        >
          <LayoutGrid className="w-4 h-4" />
        </button>
        <button
          onClick={() => setViewMode('list')}
          className={`p-2 rounded transition-colors ${
            viewMode === 'list'
              ? 'bg-brand-500 text-white'
              : 'text-content-muted hover:text-content-primary'
          }`}
          title="List view"
        >
          <LayoutList className="w-4 h-4" />
        </button>
      </div>
      <Button onClick={handleAdd}>
        <Plus className="w-4 h-4 mr-2" />
        Add Server
      </Button>
    </div>
  )

  if (loading) {
    return (
      <PageShell
        title="Remote Servers"
        description="Manage backend servers for your proxy hosts"
        actions={headerActions}
      >
        {viewMode === 'grid' ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {[1, 2, 3].map((i) => (
              <SkeletonCard key={i} showImage={false} lines={4} />
            ))}
          </div>
        ) : (
          <SkeletonTable rows={5} columns={6} />
        )}
      </PageShell>
    )
  }

  return (
    <PageShell
      title="Remote Servers"
      description="Manage backend servers for your proxy hosts"
      actions={headerActions}
    >
      {error && (
        <Alert variant="error" title="Error">
          {error}
        </Alert>
      )}

      {servers.length === 0 ? (
        <EmptyState
          icon={<Server className="h-12 w-12" />}
          title="No Remote Servers"
          description="Add servers to quickly select backends when creating proxy hosts"
          action={{
            label: 'Add Server',
            onClick: handleAdd,
          }}
        />
      ) : viewMode === 'grid' ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {servers.map((server) => (
            <Card key={server.uuid} className="flex flex-col">
              <div className="p-6 flex-1">
                <div className="flex items-start justify-between mb-4">
                  <div>
                    <h3 className="text-lg font-semibold text-content-primary mb-1">{server.name}</h3>
                    <Badge variant="outline" size="sm">{server.provider}</Badge>
                  </div>
                  <Badge variant={server.enabled ? 'success' : 'default'} size="sm">
                    {server.enabled ? 'Enabled' : 'Disabled'}
                  </Badge>
                </div>
                <div className="space-y-2 mb-4">
                  <div className="flex items-center gap-2 text-sm">
                    <span className="text-content-muted">Host:</span>
                    <span className="text-content-primary font-mono">{server.host}</span>
                  </div>
                  <div className="flex items-center gap-2 text-sm">
                    <span className="text-content-muted">Port:</span>
                    <span className="text-content-primary font-mono">{server.port}</span>
                  </div>
                  {server.username && (
                    <div className="flex items-center gap-2 text-sm">
                      <span className="text-content-muted">User:</span>
                      <span className="text-content-primary font-mono">{server.username}</span>
                    </div>
                  )}
                </div>
              </div>
              <div className="flex gap-2 px-6 pb-6 pt-4 border-t border-border">
                <Button
                  variant="secondary"
                  size="sm"
                  className="flex-1"
                  onClick={() => handleEdit(server)}
                >
                  <Pencil className="w-4 h-4 mr-2" />
                  Edit
                </Button>
                <Button
                  variant="danger"
                  size="sm"
                  className="flex-1"
                  onClick={() => setDeleteConfirm(server)}
                >
                  <Trash2 className="w-4 h-4 mr-2" />
                  Delete
                </Button>
              </div>
            </Card>
          ))}
        </div>
      ) : (
        <DataTable
          data={servers}
          columns={columns}
          rowKey={(server) => server.uuid}
          emptyState={
            <EmptyState
              icon={<Server className="h-12 w-12" />}
              title="No Remote Servers"
              description="Add servers to quickly select backends when creating proxy hosts"
              action={{
                label: 'Add Server',
                onClick: handleAdd,
              }}
            />
          }
        />
      )}

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteConfirm !== null} onOpenChange={() => setDeleteConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Remote Server</DialogTitle>
          </DialogHeader>
          <p className="text-content-secondary py-4">
            Are you sure you want to delete &quot;{deleteConfirm?.name}&quot;? This action cannot be undone.
          </p>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setDeleteConfirm(null)} disabled={isDeleting}>
              Cancel
            </Button>
            <Button
              variant="danger"
              onClick={() => deleteConfirm && handleDelete(deleteConfirm)}
              disabled={isDeleting}
            >
              {isDeleting ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Add/Edit Form Modal */}
      {showForm && (
        <RemoteServerForm
          server={editingServer}
          onSubmit={handleSubmit}
          onCancel={() => {
            setShowForm(false)
            setEditingServer(undefined)
          }}
        />
      )}
    </PageShell>
  )
}
