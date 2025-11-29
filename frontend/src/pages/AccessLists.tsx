import { useState } from 'react';
import { Button } from '../components/ui/Button';
import { Plus, Pencil, Trash2, TestTube2, ExternalLink, AlertTriangle, CheckSquare, Square } from 'lucide-react';
import {
  useAccessLists,
  useCreateAccessList,
  useUpdateAccessList,
  useDeleteAccessList,
  useTestIP,
} from '../hooks/useAccessLists';
import { AccessListForm, type AccessListFormData } from '../components/AccessListForm';
import type { AccessList } from '../api/accessLists';
import { createBackup } from '../api/backups';
import toast from 'react-hot-toast';

// Confirmation Dialog Component
function ConfirmDialog({
  isOpen,
  title,
  message,
  confirmLabel,
  onConfirm,
  onCancel,
  isLoading,
}: {
  isOpen: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
  isLoading?: boolean;
}) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onCancel}>
      <div className="bg-dark-card border border-gray-800 rounded-lg p-6 max-w-md w-full mx-4" onClick={(e) => e.stopPropagation()}>
        <h2 className="text-xl font-bold text-white mb-2">{title}</h2>
        <p className="text-gray-400 mb-6">{message}</p>
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
            Cancel
          </Button>
          <Button variant="danger" onClick={onConfirm} disabled={isLoading}>
            {isLoading ? 'Processing...' : confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}

export default function AccessLists() {
  const { data: accessLists, isLoading } = useAccessLists();
  const createMutation = useCreateAccessList();
  const updateMutation = useUpdateAccessList();
  const deleteMutation = useDeleteAccessList();
  const testIPMutation = useTestIP();

  const [showCreateForm, setShowCreateForm] = useState(false);
  const [editingACL, setEditingACL] = useState<AccessList | null>(null);
  const [testingACL, setTestingACL] = useState<AccessList | null>(null);
  const [testIP, setTestIP] = useState('');
  const [showCGNATWarning, setShowCGNATWarning] = useState(true);

  // Selection state for bulk operations
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [showBulkDeleteConfirm, setShowBulkDeleteConfirm] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState<AccessList | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const handleCreate = (data: AccessListFormData) => {
    createMutation.mutate(data, {
      onSuccess: () => setShowCreateForm(false),
    });
  };

  const handleUpdate = (data: AccessListFormData) => {
    if (!editingACL) return;
    updateMutation.mutate(
      { id: editingACL.id, data },
      {
        onSuccess: () => setEditingACL(null),
      }
    );
  };

  const handleDeleteWithBackup = async (acl: AccessList) => {
    setIsDeleting(true);
    try {
      // Create backup before deletion
      toast.loading('Creating backup before deletion...', { id: 'backup-toast' });
      await createBackup();
      toast.success('Backup created', { id: 'backup-toast' });

      // Now delete
      deleteMutation.mutate(acl.id, {
        onSuccess: () => {
          setShowDeleteConfirm(null);
          setEditingACL(null);
          toast.success(`"${acl.name}" deleted. A backup was created before deletion.`);
        },
        onError: (error) => {
          toast.error(`Failed to delete: ${error.message}`);
        },
        onSettled: () => {
          setIsDeleting(false);
        },
      });
    } catch {
      toast.error('Failed to create backup', { id: 'backup-toast' });
      setIsDeleting(false);
    }
  };

  const handleBulkDeleteWithBackup = async () => {
    if (selectedIds.size === 0) return;

    setIsDeleting(true);
    try {
      // Create backup before deletion
      toast.loading('Creating backup before bulk deletion...', { id: 'backup-toast' });
      await createBackup();
      toast.success('Backup created', { id: 'backup-toast' });

      // Delete each selected ACL
      const deletePromises = Array.from(selectedIds).map(
        (id) =>
          new Promise<void>((resolve, reject) => {
            deleteMutation.mutate(id, {
              onSuccess: () => resolve(),
              onError: (error) => reject(error),
            });
          })
      );

      await Promise.all(deletePromises);
      setSelectedIds(new Set());
      setShowBulkDeleteConfirm(false);
      toast.success(`${selectedIds.size} access list(s) deleted. A backup was created before deletion.`);
    } catch {
      toast.error('Failed to delete some items');
    } finally {
      setIsDeleting(false);
    }
  };

  const handleTestIP = () => {
    if (!testingACL || !testIP.trim()) return;

    testIPMutation.mutate(
      { id: testingACL.id, ipAddress: testIP.trim() },
      {
        onSuccess: (result) => {
          if (result.allowed) {
            toast.success(`✅ IP ${testIP} would be ALLOWED\n${result.reason}`);
          } else {
            toast.error(`🚫 IP ${testIP} would be BLOCKED\n${result.reason}`);
          }
        },
      }
    );
  };

  const toggleSelectAll = () => {
    if (!accessLists) return;
    if (selectedIds.size === accessLists.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(accessLists.map((acl) => acl.id)));
    }
  };

  const toggleSelect = (id: number) => {
    const newSelected = new Set(selectedIds);
    if (newSelected.has(id)) {
      newSelected.delete(id);
    } else {
      newSelected.add(id);
    }
    setSelectedIds(newSelected);
  };

  const getRulesDisplay = (acl: AccessList) => {
    if (acl.local_network_only) {
      return <span className="text-xs bg-blue-900/30 text-blue-300 px-2 py-1 rounded">🏠 RFC1918 Only</span>;
    }

    if (acl.type.startsWith('geo_')) {
      const countries = acl.country_codes?.split(',').filter(Boolean) || [];
      return (
        <div className="flex flex-wrap gap-1">
          {countries.slice(0, 3).map((code) => (
            <span key={code} className="text-xs bg-gray-700 px-2 py-1 rounded">{code}</span>
          ))}
          {countries.length > 3 && <span className="text-xs text-gray-400">+{countries.length - 3}</span>}
        </div>
      );
    }

    try {
      const rules = JSON.parse(acl.ip_rules || '[]');
      return (
        <div className="flex flex-wrap gap-1">
          {rules.slice(0, 2).map((rule: { cidr: string }, idx: number) => (
            <span key={idx} className="text-xs font-mono bg-gray-700 px-2 py-1 rounded">{rule.cidr}</span>
          ))}
          {rules.length > 2 && <span className="text-xs text-gray-400">+{rules.length - 2}</span>}
        </div>
      );
    } catch {
      return <span className="text-gray-500">-</span>;
    }
  };

  if (isLoading) {
    return <div className="p-8 text-center text-white">Loading access lists...</div>;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Access Control Lists</h1>
          <p className="text-gray-400 mt-1">
            Manage IP-based and geo-blocking rules for your proxy hosts
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => window.open('https://wikid82.github.io/charon/docs/security.html#acl-best-practices-by-service-type', '_blank')}
          >
            <ExternalLink className="h-4 w-4 mr-2" />
            Best Practices
          </Button>
          <Button onClick={() => setShowCreateForm(true)}>
            <Plus className="h-4 w-4 mr-2" />
            Create Access List
          </Button>
        </div>
      </div>

      {/* CGNAT Warning */}
      {showCGNATWarning && accessLists && accessLists.length > 0 && (
        <div className="bg-orange-900/20 border border-orange-800/50 rounded-lg p-4">
          <div className="flex items-start gap-3">
            <AlertTriangle className="h-5 w-5 text-orange-400 flex-shrink-0 mt-0.5" />
            <div className="flex-1">
              <h3 className="text-sm font-semibold text-orange-300 mb-1">CGNAT & Mobile Network Warning</h3>
              <p className="text-sm text-orange-200/90 mb-2">
                If you're using T-Mobile 5G Home Internet, Starlink, or other CGNAT connections, geo-blocking may not work as expected.
                Your IP may appear to be from a data center location, not your physical location.
              </p>
              <details className="text-xs text-orange-200/80">
                <summary className="cursor-pointer hover:text-orange-100 font-medium mb-1">Solutions if you're locked out:</summary>
                <ul className="list-disc list-inside space-y-1 mt-2 ml-2">
                  <li>Access via local network IP (192.168.x.x) - ACLs don't apply to local IPs</li>
                  <li>Add your current IP to a whitelist ACL</li>
                  <li>Use "Test IP" below to check what IP the server sees</li>
                  <li>Disable the ACL temporarily to regain access</li>
                  <li>Connect via VPN with a known good IP address</li>
                </ul>
              </details>
            </div>
            <button
              onClick={() => setShowCGNATWarning(false)}
              className="text-orange-400 hover:text-orange-300 text-xl leading-none"
              title="Dismiss"
            >
              ×
            </button>
          </div>
        </div>
      )}

      {/* Empty State */}
      {(!accessLists || accessLists.length === 0) && !showCreateForm && !editingACL && (
        <div className="bg-dark-card border border-gray-800 rounded-lg p-12 text-center">
          <div className="text-gray-500 mb-4 text-4xl">🛡️</div>
          <h3 className="text-lg font-semibold text-white mb-2">No Access Lists</h3>
          <p className="text-gray-400 mb-4">
            Create your first access list to control who can access your services
          </p>
          <Button onClick={() => setShowCreateForm(true)}>
            <Plus className="h-4 w-4 mr-2" />
            Create Access List
          </Button>
        </div>
      )}

      {/* Create Form */}
      {showCreateForm && (
        <div className="bg-dark-card border border-gray-800 rounded-lg p-6">
          <h2 className="text-xl font-bold text-white mb-4">Create Access List</h2>
          <AccessListForm
            onSubmit={handleCreate}
            onCancel={() => setShowCreateForm(false)}
            isLoading={createMutation.isPending}
          />
        </div>
      )}

      {/* Edit Form */}
      {editingACL && (
        <div className="bg-dark-card border border-gray-800 rounded-lg p-6">
          <h2 className="text-xl font-bold text-white mb-4">Edit Access List</h2>
          <AccessListForm
            initialData={editingACL}
            onSubmit={handleUpdate}
            onCancel={() => setEditingACL(null)}
            onDelete={() => setShowDeleteConfirm(editingACL)}
            isLoading={updateMutation.isPending}
            isDeleting={isDeleting}
          />
        </div>
      )}

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        isOpen={showDeleteConfirm !== null}
        title="Delete Access List"
        message={`Are you sure you want to delete "${showDeleteConfirm?.name}"? A backup will be created before deletion.`}
        confirmLabel="Delete"
        onConfirm={() => showDeleteConfirm && handleDeleteWithBackup(showDeleteConfirm)}
        onCancel={() => setShowDeleteConfirm(null)}
        isLoading={isDeleting}
      />

      {/* Bulk Delete Confirmation Dialog */}
      <ConfirmDialog
        isOpen={showBulkDeleteConfirm}
        title="Delete Selected Access Lists"
        message={`Are you sure you want to delete ${selectedIds.size} access list(s)? A backup will be created before deletion.`}
        confirmLabel={`Delete ${selectedIds.size} Items`}
        onConfirm={handleBulkDeleteWithBackup}
        onCancel={() => setShowBulkDeleteConfirm(false)}
        isLoading={isDeleting}
      />

      {/* Test IP Modal */}
      {testingACL && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setTestingACL(null)}>
          <div className="bg-dark-card border border-gray-800 rounded-lg p-6 max-w-md w-full mx-4" onClick={(e) => e.stopPropagation()}>
            <h2 className="text-xl font-bold text-white mb-4">Test IP Address</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">Access List</label>
                <p className="text-sm text-white">{testingACL.name}</p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">IP Address</label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={testIP}
                    onChange={(e) => setTestIP(e.target.value)}
                    placeholder="192.168.1.100"
                    onKeyDown={(e) => e.key === 'Enter' && handleTestIP()}
                    className="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                  <Button onClick={handleTestIP} disabled={testIPMutation.isPending}>
                    <TestTube2 className="h-4 w-4 mr-2" />
                    Test
                  </Button>
                </div>
              </div>
              <div className="flex justify-end">
                <Button variant="secondary" onClick={() => setTestingACL(null)}>
                  Close
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Table */}
      {accessLists && accessLists.length > 0 && !showCreateForm && !editingACL && (
        <div className="bg-dark-card border border-gray-800 rounded-lg overflow-hidden">
          {/* Bulk Actions Bar */}
          {selectedIds.size > 0 && (
            <div className="bg-gray-900 border-b border-gray-800 px-6 py-3 flex items-center justify-between">
              <span className="text-sm text-gray-300">
                {selectedIds.size} item(s) selected
              </span>
              <Button
                variant="danger"
                size="sm"
                onClick={() => setShowBulkDeleteConfirm(true)}
                disabled={isDeleting}
              >
                <Trash2 className="h-4 w-4 mr-2" />
                Delete Selected
              </Button>
            </div>
          )}
          <table className="w-full">
            <thead className="bg-gray-900/50 border-b border-gray-800">
              <tr>
                <th className="px-4 py-3 text-left">
                  <button
                    onClick={toggleSelectAll}
                    className="text-gray-400 hover:text-white"
                    title={selectedIds.size === accessLists.length ? 'Deselect all' : 'Select all'}
                  >
                    {selectedIds.size === accessLists.length ? (
                      <CheckSquare className="h-5 w-5" />
                    ) : (
                      <Square className="h-5 w-5" />
                    )}
                  </button>
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase">Name</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase">Type</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase">Rules</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase">Status</th>
                <th className="px-6 py-3 text-right text-xs font-medium text-gray-400 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {accessLists.map((acl) => (
                <tr key={acl.id} className={`hover:bg-gray-900/30 ${selectedIds.has(acl.id) ? 'bg-blue-900/20' : ''}`}>
                  <td className="px-4 py-4">
                    <button
                      onClick={() => toggleSelect(acl.id)}
                      className="text-gray-400 hover:text-white"
                    >
                      {selectedIds.has(acl.id) ? (
                        <CheckSquare className="h-5 w-5 text-blue-400" />
                      ) : (
                        <Square className="h-5 w-5" />
                      )}
                    </button>
                  </td>
                  <td className="px-6 py-4">
                    <div>
                      <p className="font-medium text-white">{acl.name}</p>
                      {acl.description && (
                        <p className="text-sm text-gray-400">{acl.description}</p>
                      )}
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <span className="px-2 py-1 text-xs bg-gray-700 border border-gray-600 rounded">
                      {acl.type.replace('_', ' ')}
                    </span>
                  </td>
                  <td className="px-6 py-4">{getRulesDisplay(acl)}</td>
                  <td className="px-6 py-4">
                    <span className={`px-2 py-1 text-xs rounded ${acl.enabled ? 'bg-green-900/30 text-green-300' : 'bg-gray-700 text-gray-400'}`}>
                      {acl.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex justify-end gap-2">
                      <button
                        onClick={() => {
                          setTestingACL(acl);
                          setTestIP('');
                        }}
                        className="text-gray-400 hover:text-blue-400"
                        title="Test IP"
                      >
                        <TestTube2 className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => setEditingACL(acl)}
                        className="text-gray-400 hover:text-blue-400"
                        title="Edit"
                      >
                        <Pencil className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => setShowDeleteConfirm(acl)}
                        className="text-gray-400 hover:text-red-400"
                        title="Delete"
                        disabled={deleteMutation.isPending || isDeleting}
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
