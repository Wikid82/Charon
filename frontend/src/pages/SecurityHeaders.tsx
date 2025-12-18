import { useState } from 'react';
import { Plus, Pencil, Trash2, Shield, Copy, Eye, Play } from 'lucide-react';
import {
  useSecurityHeaderProfiles,
  useCreateSecurityHeaderProfile,
  useUpdateSecurityHeaderProfile,
  useDeleteSecurityHeaderProfile,
  useApplySecurityHeaderPreset,
} from '../hooks/useSecurityHeaders';
import { SecurityHeaderProfileForm } from '../components/SecurityHeaderProfileForm';
import { SecurityScoreDisplay } from '../components/SecurityScoreDisplay';
import type { SecurityHeaderProfile, CreateProfileRequest } from '../api/securityHeaders';
import { createBackup } from '../api/backups';
import toast from 'react-hot-toast';
import { PageShell } from '../components/layout/PageShell';
import {
  Badge,
  Button,
  Alert,
  Card,
  EmptyState,
  SkeletonTable,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../components/ui';

export default function SecurityHeaders() {
  const { data: profiles, isLoading } = useSecurityHeaderProfiles();
  const createMutation = useCreateSecurityHeaderProfile();
  const updateMutation = useUpdateSecurityHeaderProfile();
  const deleteMutation = useDeleteSecurityHeaderProfile();
  const applyPresetMutation = useApplySecurityHeaderPreset();

  const [showCreateForm, setShowCreateForm] = useState(false);
  const [editingProfile, setEditingProfile] = useState<SecurityHeaderProfile | null>(null);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState<SecurityHeaderProfile | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const handleCreate = (data: CreateProfileRequest) => {
    createMutation.mutate(data, {
      onSuccess: () => setShowCreateForm(false),
    });
  };

  const handleUpdate = (data: CreateProfileRequest) => {
    if (!editingProfile) return;
    updateMutation.mutate(
      { id: editingProfile.id, data },
      {
        onSuccess: () => setEditingProfile(null),
      }
    );
  };

  const handleDeleteWithBackup = async (profile: SecurityHeaderProfile) => {
    setIsDeleting(true);
    try {
      toast.loading('Creating backup before deletion...', { id: 'backup-toast' });
      await createBackup();
      toast.success('Backup created', { id: 'backup-toast' });

      deleteMutation.mutate(profile.id, {
        onSuccess: () => {
          setShowDeleteConfirm(null);
          setEditingProfile(null);
          toast.success(`"${profile.name}" deleted. A backup was created before deletion.`);
        },
        onError: (error: Error) => {
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

  const handleApplyPreset = (presetType: string) => {
    const name = `${presetType.charAt(0).toUpperCase() + presetType.slice(1)} Security Profile`;
    applyPresetMutation.mutate({ preset_type: presetType, name });
  };

  const handleCloneProfile = (profile: SecurityHeaderProfile) => {
    const clonedData: CreateProfileRequest = {
      name: `${profile.name} (Copy)`,
      description: profile.description,
      hsts_enabled: profile.hsts_enabled,
      hsts_max_age: profile.hsts_max_age,
      hsts_include_subdomains: profile.hsts_include_subdomains,
      hsts_preload: profile.hsts_preload,
      csp_enabled: profile.csp_enabled,
      csp_directives: profile.csp_directives,
      csp_report_only: profile.csp_report_only,
      csp_report_uri: profile.csp_report_uri,
      x_frame_options: profile.x_frame_options,
      x_content_type_options: profile.x_content_type_options,
      referrer_policy: profile.referrer_policy,
      permissions_policy: profile.permissions_policy,
      cross_origin_opener_policy: profile.cross_origin_opener_policy,
      cross_origin_resource_policy: profile.cross_origin_resource_policy,
      cross_origin_embedder_policy: profile.cross_origin_embedder_policy,
      xss_protection: profile.xss_protection,
      cache_control_no_store: profile.cache_control_no_store,
    };

    createMutation.mutate(clonedData);
  };

  const customProfiles = profiles?.filter((p: SecurityHeaderProfile) => !p.is_preset) || [];
  const presetProfiles = (profiles?.filter((p: SecurityHeaderProfile) => p.is_preset) || [])
    .sort((a, b) => a.security_score - b.security_score);

  return (
    <PageShell
      title="Security Headers"
      description="Configure HTTP security headers for your proxy hosts"
      actions={
        <Button onClick={() => setShowCreateForm(true)}>
          <Plus className="w-4 h-4 mr-2" />
          Create Profile
        </Button>
      }
    >
      {/* Info Alert */}
      <Alert variant="info" className="mb-6">
        <Shield className="w-4 h-4" />
        <div>
          <p className="font-semibold">Secure Your Applications</p>
          <p className="text-sm mt-1">
            Security headers protect against common web vulnerabilities. Use presets for quick setup or create custom
            profiles for fine-grained control.
          </p>
        </div>
      </Alert>

      {/* Quick Presets (Read-Only) */}
      {presetProfiles.length > 0 && (
        <div className="mb-8">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Quick Presets</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {presetProfiles.map((profile: SecurityHeaderProfile) => (
              <Card key={profile.id} className="p-4">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1">
                    <h3 className="font-semibold text-gray-900 dark:text-white">{profile.name}</h3>
                    <Badge
                      variant={profile.preset_type === 'basic' ? 'outline' : profile.preset_type === 'strict' ? 'warning' : 'error'}
                      className="mt-1"
                    >
                      {profile.preset_type}
                    </Badge>
                  </div>
                  <SecurityScoreDisplay
                    score={profile.security_score}
                    size="sm"
                    showDetails={false}
                  />
                </div>
                {profile.description && (
                  <p className="text-sm text-gray-600 dark:text-gray-400 mb-3">{profile.description}</p>
                )}
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setEditingProfile(profile)}
                  >
                    <Eye className="h-4 w-4 mr-1" /> View
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleApplyPreset(profile.preset_type)}
                    disabled={applyPresetMutation.isPending}
                  >
                    <Play className="h-4 w-4 mr-1" /> Apply
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleCloneProfile(profile)}
                  >
                    <Copy className="h-4 w-4 mr-1" /> Clone
                  </Button>
                </div>
              </Card>
            ))}
          </div>
        </div>
      )}

      {/* Custom Profiles Section */}
      <div>
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Custom Profiles</h2>

        {isLoading ? (
          <SkeletonTable rows={3} />
        ) : customProfiles.length === 0 ? (
          <EmptyState
            icon={<Shield className="w-12 h-12" />}
            title="No custom profiles yet"
            description="Create a custom security header profile or apply a preset to get started"
            action={{
              label: 'Create Profile',
              onClick: () => setShowCreateForm(true),
            }}
          />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {customProfiles.map((profile: SecurityHeaderProfile) => (
              <Card key={profile.id} className="p-4">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1">
                    <h3 className="font-semibold text-gray-900 dark:text-white">{profile.name}</h3>
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      Updated {new Date(profile.updated_at).toLocaleDateString()}
                    </p>
                  </div>
                  <SecurityScoreDisplay
                    score={profile.security_score}
                    size="sm"
                    showDetails={false}
                  />
                </div>
                {profile.description && (
                  <p className="text-sm text-gray-600 dark:text-gray-400 mb-3 line-clamp-2">{profile.description}</p>
                )}
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setEditingProfile(profile)}
                    className="flex-1"
                  >
                    <Pencil className="w-3 h-3 mr-1" />
                    Edit
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleCloneProfile(profile)}
                  >
                    <Copy className="w-3 h-3" />
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setShowDeleteConfirm(profile)}
                  >
                    <Trash2 className="w-3 h-3" />
                  </Button>
                </div>
              </Card>
            ))}
          </div>
        )}
      </div>

      {/* Create/Edit Dialog */}
      <Dialog open={showCreateForm || editingProfile !== null} onOpenChange={(open: boolean) => {
        if (!open) {
          setShowCreateForm(false);
          setEditingProfile(null);
        }
      }}>
        <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editingProfile ? (editingProfile.is_preset ? 'View' : 'Edit') : 'Create'} Security Header Profile
            </DialogTitle>
          </DialogHeader>
          <SecurityHeaderProfileForm
            initialData={editingProfile || undefined}
            onSubmit={editingProfile ? handleUpdate : handleCreate}
            onCancel={() => {
              setShowCreateForm(false);
              setEditingProfile(null);
            }}
            onDelete={editingProfile && !editingProfile.is_preset ? () => setShowDeleteConfirm(editingProfile) : undefined}
            isLoading={createMutation.isPending || updateMutation.isPending}
            isDeleting={isDeleting}
          />
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={showDeleteConfirm !== null} onOpenChange={(open: boolean) => !open && setShowDeleteConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Confirm Deletion</DialogTitle>
          </DialogHeader>
          <p className="text-gray-600 dark:text-gray-400">
            Are you sure you want to delete "{showDeleteConfirm?.name}"? A backup will be created before deletion.
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDeleteConfirm(null)} disabled={isDeleting}>
              Cancel
            </Button>
            <Button
              variant="danger"
              onClick={() => showDeleteConfirm && handleDeleteWithBackup(showDeleteConfirm)}
              disabled={isDeleting}
            >
              {isDeleting ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  );
}
