import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Key, Shield, AlertTriangle, CheckCircle, Clock, RefreshCw, AlertCircle } from 'lucide-react'
import {
  useEncryptionStatus,
  useRotateKey,
  useRotationHistory,
  useValidateKeys,
  type RotationHistoryEntry,
} from '../hooks/useEncryption'
import { toast } from '../utils/toast'
import { PageShell } from '../components/layout/PageShell'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  Button,
  Badge,
  Alert,
  Progress,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  Skeleton,
} from '../components/ui'

// Skeleton loader for status cards
function StatusCardSkeleton() {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-6 w-16 rounded-full" />
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-3/4" />
        </div>
      </CardContent>
    </Card>
  )
}

// Loading skeleton for the page
function EncryptionPageSkeleton({ t }: { t: (key: string) => string }) {
  return (
    <PageShell
      title={t('encryption.title')}
      description={t('encryption.description')}
    >
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatusCardSkeleton />
        <StatusCardSkeleton />
        <StatusCardSkeleton />
        <StatusCardSkeleton />
      </div>
      <Skeleton className="h-64 w-full rounded-lg" />
    </PageShell>
  )
}

// Confirmation dialog for key rotation
interface RotationConfirmDialogProps {
  isOpen: boolean
  onClose: () => void
  onConfirm: () => void
  isPending: boolean
}

function RotationConfirmDialog({ isOpen, onClose, onConfirm, isPending }: RotationConfirmDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <AlertTriangle className="w-5 h-5 text-warning" />
            {t('encryption.confirmRotationTitle')}
          </DialogTitle>
          <DialogDescription>
            {t('encryption.confirmRotationMessage')}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3 py-4">
          <Alert variant="warning">
            <p className="text-sm">{t('encryption.rotationWarning1')}</p>
          </Alert>
          <Alert variant="info">
            <p className="text-sm">{t('encryption.rotationWarning2')}</p>
          </Alert>
        </div>
        <DialogFooter>
          <Button variant="secondary" onClick={onClose} disabled={isPending}>
            {t('common.cancel')}
          </Button>
          <Button variant="primary" onClick={onConfirm} disabled={isPending}>
            {isPending ? t('encryption.rotating') : t('encryption.confirmRotate')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function EncryptionManagement() {
  const { t } = useTranslation()
  const [showConfirmDialog, setShowConfirmDialog] = useState(false)
  const [isRotating, setIsRotating] = useState(false)

  // Fetch status with auto-refresh during rotation
  const { data: status, isLoading } = useEncryptionStatus(isRotating ? 5000 : undefined)
  const { data: history } = useRotationHistory()
  const rotateMutation = useRotateKey()
  const validateMutation = useValidateKeys()

  // Stop auto-refresh when rotation completes
  useEffect(() => {
    if (isRotating && rotateMutation.isSuccess) {
      setIsRotating(false)
    }
  }, [isRotating, rotateMutation.isSuccess])

  const handleRotateClick = () => {
    setShowConfirmDialog(true)
  }

  const handleConfirmRotation = () => {
    setShowConfirmDialog(false)
    setIsRotating(true)

    rotateMutation.mutate(undefined, {
      onSuccess: (result) => {
        toast.success(
          t('encryption.rotationSuccess', {
            count: result.success_count,
            total: result.total_providers,
            duration: result.duration,
          })
        )
        if (result.failure_count > 0) {
          toast.warning(
            t('encryption.rotationPartialFailure', { count: result.failure_count })
          )
        }
      },
      onError: (error: unknown) => {
        const msg = error instanceof Error ? error.message : String(error)
        toast.error(t('encryption.rotationError', { error: msg }))
        setIsRotating(false)
      },
    })
  }

  const handleValidateClick = () => {
    validateMutation.mutate(undefined, {
      onSuccess: (result) => {
        if (result.valid) {
          toast.success(t('encryption.validationSuccess'))
          if (result.warnings && result.warnings.length > 0) {
            result.warnings.forEach((warning) => toast.warning(warning))
          }
        } else {
          toast.error(t('encryption.validationError'))
          if (result.errors && result.errors.length > 0) {
            result.errors.forEach((error) => toast.error(error))
          }
        }
      },
      onError: (error: unknown) => {
        const msg = error instanceof Error ? error.message : String(error)
        toast.error(t('encryption.validationFailed', { error: msg }))
      },
    })
  }

  if (isLoading) {
    return <EncryptionPageSkeleton t={t} />
  }

  if (!status) {
    return (
      <PageShell
        title={t('encryption.title')}
        description={t('encryption.description')}
      >
        <Alert variant="error" title={t('common.error')}>
          {t('encryption.failedToLoadStatus')}
        </Alert>
      </PageShell>
    )
  }

  const hasOlderVersions = status.providers_on_older_versions > 0
  const rotationDisabled = isRotating || !status.next_key_configured

  return (
    <>
      <PageShell
        title={t('encryption.title')}
        description={t('encryption.description')}
      >
        {/* Status Overview Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          {/* Current Key Version */}
          <Card data-testid="encryption-current-version">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-base">{t('encryption.currentVersion')}</CardTitle>
                <Key className="w-5 h-5 text-brand-500" />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-content-primary">
                {t('encryption.versionNumber', { version: status.current_version })}
              </div>
              <p className="text-sm text-content-muted mt-2">
                {t('encryption.activeEncryptionKey')}
              </p>
            </CardContent>
          </Card>

          {/* Providers on Current Version */}
          <Card data-testid="encryption-providers-updated">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-base">{t('encryption.providersUpdated')}</CardTitle>
                <CheckCircle className="w-5 h-5 text-success" />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-success">
                {status.providers_on_current_version}
              </div>
              <p className="text-sm text-content-muted mt-2">
                {t('encryption.providersOnCurrentVersion')}
              </p>
            </CardContent>
          </Card>

          {/* Providers on Older Versions */}
          <Card data-testid="encryption-providers-outdated">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-base">{t('encryption.providersOutdated')}</CardTitle>
                <AlertCircle className={`w-5 h-5 ${hasOlderVersions ? 'text-warning' : 'text-content-muted'}`} />
              </div>
            </CardHeader>
            <CardContent>
              <div className={`text-3xl font-bold ${hasOlderVersions ? 'text-warning' : 'text-content-muted'}`}>
                {status.providers_on_older_versions}
              </div>
              <p className="text-sm text-content-muted mt-2">
                {t('encryption.providersNeedRotation')}
              </p>
            </CardContent>
          </Card>

          {/* Next Key Configured */}
          <Card data-testid="encryption-next-key">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-base">{t('encryption.nextKey')}</CardTitle>
                <Shield className={`w-5 h-5 ${status.next_key_configured ? 'text-success' : 'text-content-muted'}`} />
              </div>
            </CardHeader>
            <CardContent>
              <Badge variant={status.next_key_configured ? 'success' : 'default'} className="mb-2">
                {status.next_key_configured ? t('encryption.configured') : t('encryption.notConfigured')}
              </Badge>
              <p className="text-sm text-content-muted">
                {t('encryption.nextKeyDescription')}
              </p>
            </CardContent>
          </Card>
        </div>

        {/* Legacy Keys Warning */}
        {status.legacy_key_count > 0 && (
          <Alert variant="info" title={t('encryption.legacyKeysDetected')}>
            <p>
              {t('encryption.legacyKeysMessage', { count: status.legacy_key_count })}
            </p>
          </Alert>
        )}

        {/* Actions Section */}
        <Card data-testid="encryption-actions-card">
          <CardHeader>
            <CardTitle>{t('encryption.actions')}</CardTitle>
            <CardDescription>{t('encryption.actionsDescription')}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap gap-3">
              <Button
                variant="primary"
                onClick={handleRotateClick}
                disabled={rotationDisabled}
                data-testid="rotate-key-btn"
              >
                <RefreshCw className={`w-4 h-4 mr-2 ${isRotating ? 'animate-spin' : ''}`} />
                {isRotating ? t('encryption.rotating') : t('encryption.rotateKey')}
              </Button>
              <Button
                variant="secondary"
                onClick={handleValidateClick}
                disabled={validateMutation.isPending}
                data-testid="validate-config-btn"
              >
                <CheckCircle className="w-4 h-4 mr-2" />
                {validateMutation.isPending ? t('encryption.validating') : t('encryption.validateConfig')}
              </Button>
            </div>

            {!status.next_key_configured && (
              <Alert variant="warning">
                <p className="text-sm">{t('encryption.nextKeyRequired')}</p>
              </Alert>
            )}

            {isRotating && (
              <div className="space-y-2">
                <div className="flex items-center justify-between text-sm">
                  <span className="text-content-secondary">{t('encryption.rotationInProgress')}</span>
                  <Clock className="w-4 h-4 text-content-muted animate-pulse" />
                </div>
                <Progress value={undefined} className="h-2" />
              </div>
            )}
          </CardContent>
        </Card>

        {/* Environment Variable Guide */}
        <Card>
          <CardHeader>
            <CardTitle>{t('encryption.environmentGuide')}</CardTitle>
            <CardDescription>{t('encryption.environmentGuideDescription')}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="bg-surface-muted rounded-md p-4 font-mono text-sm">
              <div className="space-y-1">
                <div className="text-success"># Current encryption key (required)</div>
                <div>CHARON_ENCRYPTION_KEY=&lt;base64-encoded-32-byte-key&gt;</div>
                <div className="text-success mt-3"># During rotation: new key</div>
                <div>CHARON_ENCRYPTION_KEY_V2=&lt;new-base64-encoded-key&gt;</div>
                <div className="text-success mt-3"># Legacy keys for decryption</div>
                <div>CHARON_ENCRYPTION_KEY_V1=&lt;old-key&gt;</div>
              </div>
            </div>

            <div className="space-y-3 text-sm text-content-secondary">
              <div>
                <strong className="text-content-primary">{t('encryption.step1')}:</strong>{' '}
                {t('encryption.step1Description')}
              </div>
              <div>
                <strong className="text-content-primary">{t('encryption.step2')}:</strong>{' '}
                {t('encryption.step2Description')}
              </div>
              <div>
                <strong className="text-content-primary">{t('encryption.step3')}:</strong>{' '}
                {t('encryption.step3Description')}
              </div>
              <div>
                <strong className="text-content-primary">{t('encryption.step4')}:</strong>{' '}
                {t('encryption.step4Description')}
              </div>
            </div>

            <Alert variant="warning">
              <p className="text-sm">{t('encryption.retentionWarning')}</p>
            </Alert>
          </CardContent>
        </Card>

        {/* Rotation History */}
        {history && history.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle>{t('encryption.rotationHistory')}</CardTitle>
              <CardDescription>{t('encryption.rotationHistoryDescription')}</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="border-b border-border">
                    <tr className="text-left">
                      <th className="pb-3 font-medium text-content-secondary">{t('encryption.date')}</th>
                      <th className="pb-3 font-medium text-content-secondary">{t('encryption.actor')}</th>
                      <th className="pb-3 font-medium text-content-secondary">{t('encryption.action')}</th>
                      <th className="pb-3 font-medium text-content-secondary">{t('encryption.details')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {history.slice(0, 10).map((entry: RotationHistoryEntry) => {
                      const details = entry.details ? JSON.parse(entry.details) : {}
                      return (
                        <tr key={entry.uuid}>
                          <td className="py-3 text-content-primary">
                            {new Date(entry.created_at).toLocaleString()}
                          </td>
                          <td className="py-3 text-content-primary">{entry.actor}</td>
                          <td className="py-3">
                            <Badge variant="default" size="sm">
                              {entry.action}
                            </Badge>
                          </td>
                          <td className="py-3 text-content-muted">
                            {details.new_key_version && (
                              <span>
                                {t('encryption.versionNumber', { version: details.new_key_version })}
                              </span>
                            )}
                            {details.duration && <span className="ml-2">({details.duration})</span>}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        )}
      </PageShell>

      {/* Confirmation Dialog */}
      <RotationConfirmDialog
        isOpen={showConfirmDialog}
        onClose={() => setShowConfirmDialog(false)}
        onConfirm={handleConfirmRotation}
        isPending={rotateMutation.isPending}
      />
    </>
  )
}
