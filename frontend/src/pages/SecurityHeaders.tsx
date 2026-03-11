import { Plus, Pencil, Trash2, Shield, Copy, Eye, Info } from 'lucide-react';
import { useState } from 'react';
import toast from 'react-hot-toast';
import { useTranslation } from 'react-i18next';



import { createBackup } from '../api/backups';
import { PageShell } from '../components/layout/PageShell';
import { SecurityHeaderProfileForm } from '../components/SecurityHeaderProfileForm';
import { SecurityScoreDisplay } from '../components/SecurityScoreDisplay';
import {
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
  Tooltip,
  TooltipTrigger,
  TooltipContent,
  TooltipProvider,
} from '../components/ui';
import {
  useSecurityHeaderProfiles,
  useCreateSecurityHeaderProfile,
  useUpdateSecurityHeaderProfile,
  useDeleteSecurityHeaderProfile,
} from '../hooks/useSecurityHeaders';

import type { SecurityHeaderProfile, CreateProfileRequest } from '../api/securityHeaders';

export default function SecurityHeaders() {
  const { t } = useTranslation();
  const { data: profiles, isLoading } = useSecurityHeaderProfiles();
  const createMutation = useCreateSecurityHeaderProfile();
  const updateMutation = useUpdateSecurityHeaderProfile();
  const deleteMutation = useDeleteSecurityHeaderProfile();

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
      toast.loading(t('securityHeaders.creatingBackup'), { id: 'backup-toast' });
      await createBackup();
      toast.success(t('securityHeaders.backupCreated'), { id: 'backup-toast' });

      deleteMutation.mutate(profile.id, {
        onSuccess: () => {
          setShowDeleteConfirm(null);
          setEditingProfile(null);
          toast.success(t('securityHeaders.deleteSuccess', { name: profile.name }));
        },
        onError: (error: Error) => {
          toast.error(t('securityHeaders.deleteFailed', { error: error.message }));
        },
        onSettled: () => {
          setIsDeleting(false);
        },
      });
    } catch {
      toast.error(t('securityHeaders.backupFailed'), { id: 'backup-toast' });
      setIsDeleting(false);
    }
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

  // Get tooltip content for preset types
  const getPresetTooltip = (presetType: string): string => {
    switch (presetType) {
      case 'basic':
        return t('securityHeaders.presets.basic');
      case 'api-friendly':
        return t('securityHeaders.presets.apiFriendly');
      case 'strict':
        return t('securityHeaders.presets.strict');
      case 'paranoid':
        return t('securityHeaders.presets.paranoid');
      default:
        return '';
    }
  };

  return (
    <PageShell
      title={t('securityHeaders.title')}
      description={t('securityHeaders.description')}
      actions={
        <Button onClick={() => setShowCreateForm(true)}>
          <Plus className="w-4 h-4 mr-2" />
          {t('securityHeaders.createProfile')}
        </Button>
      }
    >
      {/* Info Alert */}
      <Alert variant="info" className="mb-6">
        <Shield className="w-4 h-4" />
        <div>
          <p className="font-semibold">{t('securityHeaders.alertTitle')}</p>
          <p className="text-sm mt-1">
            {t('securityHeaders.alertDescription')}
          </p>
        </div>
      </Alert>

      {/* Quick Presets (Read-Only) */}
      {presetProfiles.length > 0 && (
        <div className="mb-8">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
            {t('securityHeaders.systemProfiles')}
          </h2>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
            {t('securityHeaders.systemProfilesDescription')}
          </p>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            <TooltipProvider>
            {presetProfiles.map((profile: SecurityHeaderProfile) => (
              <Card key={profile.id} className="p-4">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1">
                    <div className="flex items-center gap-1.5">
                      <h3 className="font-semibold text-gray-900 dark:text-white">{profile.name}</h3>
                      {profile.preset_type && getPresetTooltip(profile.preset_type) && (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <button type="button" className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors">
                              <Info className="w-4 h-4" />
                            </button>
                          </TooltipTrigger>
                          <TooltipContent side="top" className="max-w-xs whitespace-pre-line text-left">
                            {getPresetTooltip(profile.preset_type)}
                          </TooltipContent>
                        </Tooltip>
                      )}
                    </div>
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
                    <Eye className="h-4 w-4 mr-1" /> {t('securityHeaders.view')}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleCloneProfile(profile)}
                  >
                    <Copy className="h-4 w-4 mr-1" /> {t('securityHeaders.clone')}
                  </Button>
                </div>
              </Card>
            ))}
            </TooltipProvider>
          </div>
        </div>
      )}

      {/* Custom Profiles Section */}
      <div>
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">{t('securityHeaders.customProfiles')}</h2>

        {isLoading ? (
          <SkeletonTable rows={3} />
        ) : customProfiles.length === 0 ? (
          <EmptyState
            icon={<Shield className="w-12 h-12" />}
            title={t('securityHeaders.noCustomProfiles')}
            description={t('securityHeaders.noCustomProfilesDescription')}
            action={{
              label: t('securityHeaders.createProfile'),
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
                      {t('securityHeaders.updated')} {new Date(profile.updated_at).toLocaleDateString()}
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
                    {t('common.edit')}
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
                    aria-label={t('common.delete')}
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
              {editingProfile
                ? (editingProfile.is_preset ? t('securityHeaders.viewProfile') : t('securityHeaders.editProfile'))
                : t('securityHeaders.createProfileTitle')}
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
            <DialogTitle>{t('securityHeaders.confirmDeletion')}</DialogTitle>
          </DialogHeader>
          <p className="text-gray-600 dark:text-gray-400">
            {t('securityHeaders.deleteConfirmMessage', { name: showDeleteConfirm?.name })}
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDeleteConfirm(null)} disabled={isDeleting}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="danger"
              onClick={() => showDeleteConfirm && handleDeleteWithBackup(showDeleteConfirm)}
              disabled={isDeleting}
            >
              {isDeleting ? t('securityHeaders.deleting') : t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  );
}
