import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Plus, Pencil, Trash2, TestTube2, ExternalLink, Shield } from 'lucide-react';
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
import { PageShell } from '../components/layout/PageShell';
import {
  Badge,
  Button,
  Alert,
  DataTable,
  EmptyState,
  SkeletonTable,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Input,
  Card,
  type Column,
} from '../components/ui';

export default function AccessLists() {
  const { t } = useTranslation();
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
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
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
      toast.loading(t('accessLists.creatingBackup'), { id: 'backup-toast' });
      await createBackup();
      toast.success(t('accessLists.backupCreated'), { id: 'backup-toast' });

      // Now delete
      deleteMutation.mutate(acl.id, {
        onSuccess: () => {
          setShowDeleteConfirm(null);
          setEditingACL(null);
          toast.success(`"${acl.name}" ${t('accessLists.deletedWithBackup')}`);
        },
        onError: (error) => {
          toast.error(`${t('accessLists.failedToDelete')}: ${error.message}`);
        },
        onSettled: () => {
          setIsDeleting(false);
        },
      });
    } catch {
      toast.error(t('accessLists.failedToCreateBackup'), { id: 'backup-toast' });
      setIsDeleting(false);
    }
  };

  const handleBulkDeleteWithBackup = async () => {
    if (selectedIds.size === 0) return;

    setIsDeleting(true);
    try {
      // Create backup before deletion
      toast.loading(t('accessLists.creatingBulkBackup'), { id: 'backup-toast' });
      await createBackup();
      toast.success(t('accessLists.backupCreated'), { id: 'backup-toast' });

      // Delete each selected ACL
      const deletePromises = Array.from(selectedIds).map(
        (id) =>
          new Promise<void>((resolve, reject) => {
            deleteMutation.mutate(Number(id), {
              onSuccess: () => resolve(),
              onError: (error) => reject(error),
            });
          })
      );

      await Promise.all(deletePromises);
      setSelectedIds(new Set());
      setShowBulkDeleteConfirm(false);
      toast.success(t('accessLists.bulkDeletedWithBackup', { count: selectedIds.size }));
    } catch {
      toast.error(t('accessLists.failedToDeleteSome'));
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
            toast.success(`✅ ${t('accessLists.ipAllowed', { ip: testIP })}\n${result.reason}`);
          } else {
            toast.error(`🚫 ${t('accessLists.ipBlocked', { ip: testIP })}\n${result.reason}`);
          }
        },
      }
    );
  };

  const getRulesDisplay = (acl: AccessList) => {
    if (acl.local_network_only) {
      return <Badge variant="primary" size="sm">🏠 {t('accessLists.rfc1918Only')}</Badge>;
    }

    if (acl.type.startsWith('geo_')) {
      const countries = acl.country_codes?.split(',').filter(Boolean) || [];
      return (
        <div className="flex flex-wrap gap-1">
          {countries.slice(0, 3).map((code) => (
            <Badge key={code} variant="outline" size="sm">{code}</Badge>
          ))}
          {countries.length > 3 && <span className="text-xs text-content-muted">+{countries.length - 3}</span>}
        </div>
      );
    }

    try {
      const rules = JSON.parse(acl.ip_rules || '[]');
      return (
        <div className="flex flex-wrap gap-1">
          {rules.slice(0, 2).map((rule: { cidr: string }, idx: number) => (
            <Badge key={idx} variant="outline" size="sm" className="font-mono">{rule.cidr}</Badge>
          ))}
          {rules.length > 2 && <span className="text-xs text-content-muted">+{rules.length - 2}</span>}
        </div>
      );
    } catch {
      return <span className="text-content-muted">-</span>;
    }
  };

  const getTypeBadge = (acl: AccessList) => {
    const type = acl.type;
    if (type === 'whitelist' || type === 'geo_whitelist') {
      return <Badge variant="success" size="sm">{t('accessLists.allow')}</Badge>;
    }
    // blacklist or geo_blacklist
    return <Badge variant="error" size="sm">{t('accessLists.deny')}</Badge>;
  };

  const columns: Column<AccessList>[] = [
    {
      key: 'name',
      header: t('common.name'),
      sortable: true,
      cell: (acl) => (
        <div>
          <p className="font-medium text-content-primary">{acl.name}</p>
          {acl.description && (
            <p className="text-sm text-content-secondary">{acl.description}</p>
          )}
        </div>
      ),
    },
    {
      key: 'type',
      header: t('common.type'),
      sortable: true,
      cell: (acl) => getTypeBadge(acl),
    },
    {
      key: 'rules',
      header: t('accessLists.rules'),
      cell: (acl) => getRulesDisplay(acl),
    },
    {
      key: 'status',
      header: t('common.status'),
      sortable: true,
      cell: (acl) => (
        <Badge variant={acl.enabled ? 'success' : 'default'} size="sm">
          {acl.enabled ? t('common.enabled') : t('common.disabled')}
        </Badge>
      ),
    },
    {
      key: 'actions',
      header: t('common.actions'),
      cell: (acl) => (
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={(e) => {
              e.stopPropagation();
              setTestingACL(acl);
              setTestIP('');
            }}
            title={t('accessLists.testIp')}
          >
            <TestTube2 className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={(e) => {
              e.stopPropagation();
              setEditingACL(acl);
            }}
            title={t('common.edit')}
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={(e) => {
              e.stopPropagation();
              setShowDeleteConfirm(acl);
            }}
            title={t('common.delete')}
            disabled={deleteMutation.isPending || isDeleting}
          >
            <Trash2 className="h-4 w-4 text-error" />
          </Button>
        </div>
      ),
    },
  ];

  // Header actions
  const headerActions = (
    <div className="flex items-center gap-2">
      {selectedIds.size > 0 && (
        <Button
          variant="danger"
          size="sm"
          onClick={() => setShowBulkDeleteConfirm(true)}
          disabled={isDeleting}
        >
          <Trash2 className="h-4 w-4 mr-2" />
          {t('common.delete')} ({selectedIds.size})
        </Button>
      )}
      <Button
        variant="secondary"
        onClick={() => window.open('https://wikid82.github.io/charon/security#acl-best-practices-by-service-type', '_blank')}
      >
        <ExternalLink className="h-4 w-4 mr-2" />
        {t('accessLists.bestPractices')}
      </Button>
      <Button onClick={() => setShowCreateForm(true)}>
        <Plus className="h-4 w-4 mr-2" />
        {t('accessLists.createAccessList')}
      </Button>
    </div>
  );

  if (isLoading) {
    return (
      <PageShell
        title={t('accessLists.title')}
        description={t('accessLists.description')}
        actions={headerActions}
      >
        <SkeletonTable rows={5} columns={5} />
      </PageShell>
    );
  }

  return (
    <PageShell
      title={t('accessLists.title')}
      description={t('accessLists.description')}
      actions={headerActions}
    >
      {/* CGNAT Warning */}
      {showCGNATWarning && accessLists && accessLists.length > 0 && (
        <Alert
          variant="warning"
          title={t('accessLists.cgnatWarningTitle')}
          dismissible
          onDismiss={() => setShowCGNATWarning(false)}
        >
          <div className="space-y-2">
            <p>
              {t('accessLists.cgnatWarningMessage')}
            </p>
            <details className="text-xs">
              <summary className="cursor-pointer hover:text-content-primary font-medium mb-1">{t('accessLists.solutionsIfLockedOut')}</summary>
              <ul className="list-disc list-inside space-y-1 mt-2 ml-2">
                <li>{t('accessLists.solutionLocalNetwork')}</li>
                <li>{t('accessLists.solutionWhitelist')}</li>
                <li>{t('accessLists.solutionTestIp')}</li>
                <li>{t('accessLists.solutionDisableAcl')}</li>
                <li>{t('accessLists.solutionVpn')}</li>
              </ul>
            </details>
          </div>
        </Alert>
      )}

      {/* Create Form */}
      {showCreateForm && (
        <Card className="p-6">
          <h2 className="text-xl font-bold text-content-primary mb-4">{t('accessLists.createAccessList')}</h2>
          <AccessListForm
            onSubmit={handleCreate}
            onCancel={() => setShowCreateForm(false)}
            isLoading={createMutation.isPending}
          />
        </Card>
      )}

      {/* Edit Form */}
      {editingACL && (
        <Card className="p-6">
          <h2 className="text-xl font-bold text-content-primary mb-4">{t('accessLists.editAccessList')}</h2>
          <AccessListForm
            initialData={editingACL}
            onSubmit={handleUpdate}
            onCancel={() => setEditingACL(null)}
            onDelete={() => setShowDeleteConfirm(editingACL)}
            isLoading={updateMutation.isPending}
            isDeleting={isDeleting}
          />
        </Card>
      )}

      {/* Delete Confirmation Dialog */}
      <Dialog open={showDeleteConfirm !== null} onOpenChange={() => setShowDeleteConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('accessLists.deleteAccessList')}</DialogTitle>
          </DialogHeader>
          <p className="text-content-secondary py-4">
            {t('accessLists.deleteConfirmation', { name: showDeleteConfirm?.name })}
          </p>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setShowDeleteConfirm(null)} disabled={isDeleting}>
              {t('common.cancel')}
            </Button>
            <Button variant="danger" onClick={() => showDeleteConfirm && handleDeleteWithBackup(showDeleteConfirm)} disabled={isDeleting}>
              {isDeleting ? t('common.deleting') : t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Bulk Delete Confirmation Dialog */}
      <Dialog open={showBulkDeleteConfirm} onOpenChange={setShowBulkDeleteConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('accessLists.deleteSelectedAccessLists')}</DialogTitle>
          </DialogHeader>
          <p className="text-content-secondary py-4">
            {t('accessLists.bulkDeleteConfirmation', { count: selectedIds.size })}
          </p>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setShowBulkDeleteConfirm(false)} disabled={isDeleting}>
              {t('common.cancel')}
            </Button>
            <Button variant="danger" onClick={handleBulkDeleteWithBackup} disabled={isDeleting}>
              {isDeleting ? t('common.deleting') : t('accessLists.deleteItems', { count: selectedIds.size })}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Test IP Dialog */}
      <Dialog open={testingACL !== null} onOpenChange={() => setTestingACL(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('accessLists.testIpAddress')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div>
              <label className="block text-sm font-medium text-content-secondary mb-2">{t('accessLists.accessList')}</label>
              <p className="text-sm text-content-primary">{testingACL?.name}</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-content-secondary mb-2">{t('accessLists.ipAddress')}</label>
              <div className="flex gap-2">
                <Input
                  value={testIP}
                  onChange={(e) => setTestIP(e.target.value)}
                  placeholder="192.168.1.100"
                  onKeyDown={(e) => e.key === 'Enter' && handleTestIP()}
                  className="flex-1"
                />
                <Button onClick={handleTestIP} disabled={testIPMutation.isPending}>
                  <TestTube2 className="h-4 w-4 mr-2" />
                  {t('common.test')}
                </Button>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setTestingACL(null)}>
              {t('common.close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Empty State or DataTable */}
      {!showCreateForm && !editingACL && (
        <>
          {(!accessLists || accessLists.length === 0) ? (
            <EmptyState
              icon={<Shield className="h-12 w-12" />}
              title={t('accessLists.noAccessLists')}
              description={t('accessLists.noAccessListsDescription')}
              action={{
                label: t('accessLists.createAccessList'),
                onClick: () => setShowCreateForm(true),
              }}
            />
          ) : (
            <DataTable
              data={accessLists}
              columns={columns}
              rowKey={(acl) => String(acl.id)}
              selectable
              selectedKeys={selectedIds}
              onSelectionChange={setSelectedIds}
              emptyState={
                <EmptyState
                  icon={<Shield className="h-12 w-12" />}
                  title={t('accessLists.noAccessLists')}
                  description={t('accessLists.noAccessListsDescription')}
                  action={{
                    label: t('accessLists.createAccessList'),
                    onClick: () => setShowCreateForm(true),
                  }}
                />
              }
            />
          )}
        </>
      )}
    </PageShell>
  );
}
