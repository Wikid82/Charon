import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  useCreateRemoteTarget,
  useUpdateRemoteTarget,
  useTestRemoteTarget,
  type RemoteTarget,
  type RemoteTargetConfig,
  type RemoteTargetSecrets,
} from '../../hooks/useRemoteTargets'
import {
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  Input,
  Label,
} from '../ui'
import { toast } from '../../utils/toast'

export interface RemoteTargetFormDialogProps {
  /** Whether the dialog is open. */
  open: boolean
  /** The target being edited, or `null`/`undefined` when creating a new one. */
  target?: RemoteTarget | null
  onClose: () => void
}

/**
 * Create/edit form for a remote storage target (spec §3.3.2, §3.7, §3.8).
 * Type switch between S3 and SFTP; secret inputs are `type="password"` and
 * blank-on-edit ("leave blank to keep current") — never pre-populated from
 * the API, since the API never returns secrets.
 *
 * SFTP host-key discovery: `POST .../remote-targets/:uuid/test` is also used
 * (with no `host_key_fingerprint` stored yet) to discover the remote host's
 * key without ever authenticating (spec §3.7). NOTE: the current backend's
 * `Test` handler looks up an existing persisted `RemoteStorageTarget` by
 * UUID, so pre-create discovery (before the target has a UUID) uses a
 * placeholder id and will 404 against a real server today; this only
 * exercises the mocked E2E flow until a backend follow-up adds a stateless
 * "test this draft config" endpoint.
 */
export function RemoteTargetFormDialog({ open, target, onClose }: RemoteTargetFormDialogProps) {
  const { t } = useTranslation()
  const isEdit = Boolean(target)
  const createMutation = useCreateRemoteTarget()
  const updateMutation = useUpdateRemoteTarget()
  const testMutation = useTestRemoteTarget()

  const [type, setType] = useState<'s3' | 'sftp'>(target?.type ?? 's3')
  const [name, setName] = useState(target?.name ?? '')

  const [endpoint, setEndpoint] = useState(target?.config.endpoint ?? '')
  const [region, setRegion] = useState(target?.config.region ?? '')
  const [bucket, setBucket] = useState(target?.config.bucket ?? '')
  const [pathPrefix, setPathPrefix] = useState(target?.config.path_prefix ?? '')
  const [useSSL, setUseSSL] = useState(target?.config.use_ssl ?? true)
  const [forcePathStyle, setForcePathStyle] = useState(target?.config.force_path_style ?? false)
  const [accessKeyId, setAccessKeyId] = useState('')
  const [secretAccessKey, setSecretAccessKey] = useState('')

  const [host, setHost] = useState(target?.config.host ?? '')
  const [port, setPort] = useState(String(target?.config.port ?? 22))
  const [path, setPath] = useState(target?.config.path ?? '')
  const [username, setUsername] = useState(target?.config.username ?? '')
  const [password, setPassword] = useState('')
  const [hostKeyFingerprint, setHostKeyFingerprint] = useState(target?.config.host_key_fingerprint ?? '')
  const [discoveredFingerprint, setDiscoveredFingerprint] = useState<string | null>(null)

  useEffect(() => {
    if (!open) return
    setType(target?.type ?? 's3')
    setName(target?.name ?? '')
    setEndpoint(target?.config.endpoint ?? '')
    setRegion(target?.config.region ?? '')
    setBucket(target?.config.bucket ?? '')
    setPathPrefix(target?.config.path_prefix ?? '')
    setUseSSL(target?.config.use_ssl ?? true)
    setForcePathStyle(target?.config.force_path_style ?? false)
    setAccessKeyId('')
    setSecretAccessKey('')
    setHost(target?.config.host ?? '')
    setPort(String(target?.config.port ?? 22))
    setPath(target?.config.path ?? '')
    setUsername(target?.config.username ?? '')
    setPassword('')
    setHostKeyFingerprint(target?.config.host_key_fingerprint ?? '')
    setDiscoveredFingerprint(null)
    // Only re-seed when the dialog (re)opens or targets a different record.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, target?.uuid])

  const buildConfig = (): RemoteTargetConfig =>
    type === 's3'
      ? {
          endpoint,
          region,
          bucket,
          path_prefix: pathPrefix,
          use_ssl: useSSL,
          force_path_style: forcePathStyle,
        }
      : {
          host,
          port: parseInt(port, 10) || 0,
          path,
          username,
          host_key_fingerprint: hostKeyFingerprint || undefined,
        }

  const buildSecrets = (): RemoteTargetSecrets | undefined => {
    if (type === 's3') {
      if (!accessKeyId && !secretAccessKey) return undefined
      return {
        access_key_id: accessKeyId || undefined,
        secret_access_key: secretAccessKey || undefined,
      }
    }
    if (!password) return undefined
    return { password }
  }

  const isSaving = createMutation.isPending || updateMutation.isPending

  const handleSubmit = () => {
    const payload = { name, type, enabled: target?.enabled ?? true, config: buildConfig(), secrets: buildSecrets() }
    if (isEdit && target) {
      updateMutation.mutate(
        { uuid: target.uuid, payload },
        { onSuccess: onClose, onError: (error: Error) => toast.error(error.message) }
      )
    } else {
      createMutation.mutate(payload, {
        onSuccess: onClose,
        onError: (error: Error) => toast.error(error.message),
      })
    }
  }

  const handleDiscoverHostKey = () => {
    testMutation.mutate(target?.uuid ?? 'draft', {
      onSuccess: (result) => {
        if (result.discovered_fingerprint) {
          setDiscoveredFingerprint(result.discovered_fingerprint)
        }
      },
      onError: (error: Error) => toast.error(error.message),
    })
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t('backups.remoteTargets.editTitle') : t('backups.remoteTargets.createTitle')}
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-4 px-6 max-h-[65vh] overflow-y-auto">
          <Input
            id="backup-remote-target-name"
            label={t('common.name')}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />

          {!isEdit && (
            <fieldset className="space-y-2">
              <legend className="text-sm font-medium text-content-secondary mb-1.5">
                {t('backups.remoteTargets.type')}
              </legend>
              <div className="flex gap-4">
                <label className="flex items-center gap-2 text-sm text-content-primary cursor-pointer">
                  <input
                    type="radio"
                    name="backup-remote-target-type"
                    value="s3"
                    checked={type === 's3'}
                    onChange={() => setType('s3')}
                  />
                  {t('backups.remoteTargets.typeS3')}
                </label>
                <label className="flex items-center gap-2 text-sm text-content-primary cursor-pointer">
                  <input
                    type="radio"
                    name="backup-remote-target-type"
                    value="sftp"
                    checked={type === 'sftp'}
                    onChange={() => setType('sftp')}
                  />
                  {t('backups.remoteTargets.typeSftp')}
                </label>
              </div>
            </fieldset>
          )}

          {type === 's3' ? (
            <div className="space-y-4">
              <Input
                id="backup-remote-target-endpoint"
                label={t('backups.remoteTargets.endpoint')}
                value={endpoint}
                onChange={(e) => setEndpoint(e.target.value)}
              />
              <div className="grid grid-cols-2 gap-4">
                <Input
                  id="backup-remote-target-region"
                  label={t('backups.remoteTargets.region')}
                  value={region}
                  onChange={(e) => setRegion(e.target.value)}
                />
                <Input
                  id="backup-remote-target-bucket"
                  label={t('backups.remoteTargets.bucket')}
                  value={bucket}
                  onChange={(e) => setBucket(e.target.value)}
                />
              </div>
              <Input
                id="backup-remote-target-path-prefix"
                label={t('backups.remoteTargets.pathPrefix')}
                value={pathPrefix}
                onChange={(e) => setPathPrefix(e.target.value)}
              />
              <div className="flex items-center gap-2">
                <Checkbox
                  id="backup-remote-target-use-ssl"
                  checked={useSSL}
                  onCheckedChange={(checked) => setUseSSL(checked === true)}
                />
                <Label htmlFor="backup-remote-target-use-ssl">{t('backups.remoteTargets.useSsl')}</Label>
              </div>
              <div className="flex items-center gap-2">
                <Checkbox
                  id="backup-remote-target-force-path-style"
                  checked={forcePathStyle}
                  onCheckedChange={(checked) => setForcePathStyle(checked === true)}
                />
                <Label htmlFor="backup-remote-target-force-path-style">
                  {t('backups.remoteTargets.forcePathStyle')}
                </Label>
              </div>
              <Input
                id="backup-remote-target-access-key-id"
                label={t('backups.remoteTargets.accessKeyId')}
                type="password"
                value={accessKeyId}
                onChange={(e) => setAccessKeyId(e.target.value)}
                placeholder={isEdit ? t('backups.remoteTargets.keepCurrent') : undefined}
                autoComplete="new-password"
              />
              <Input
                id="backup-remote-target-secret-access-key"
                label={t('backups.remoteTargets.secretAccessKey')}
                type="password"
                value={secretAccessKey}
                onChange={(e) => setSecretAccessKey(e.target.value)}
                placeholder={isEdit ? t('backups.remoteTargets.keepCurrent') : undefined}
                autoComplete="new-password"
              />
              {isEdit && (
                <p className="text-xs text-content-muted">{t('backups.remoteTargets.leaveBlankToKeepCurrent')}</p>
              )}
            </div>
          ) : (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <Input
                  id="backup-remote-target-host"
                  label={t('backups.remoteTargets.host')}
                  value={host}
                  onChange={(e) => setHost(e.target.value)}
                />
                <Input
                  id="backup-remote-target-port"
                  label={t('backups.remoteTargets.port')}
                  type="number"
                  value={port}
                  onChange={(e) => setPort(e.target.value)}
                />
              </div>
              <Input
                id="backup-remote-target-path"
                label={t('backups.remoteTargets.path')}
                value={path}
                onChange={(e) => setPath(e.target.value)}
              />
              <Input
                id="backup-remote-target-username"
                label={t('backups.remoteTargets.username')}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
              <Input
                id="backup-remote-target-password"
                label={t('backups.remoteTargets.password')}
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={isEdit ? t('backups.remoteTargets.keepCurrent') : undefined}
                autoComplete="new-password"
              />
              {isEdit && (
                <p className="text-xs text-content-muted">{t('backups.remoteTargets.leaveBlankToKeepCurrent')}</p>
              )}

              <div className="flex items-center gap-3">
                <Button
                  type="button"
                  variant="secondary"
                  onClick={handleDiscoverHostKey}
                  isLoading={testMutation.isPending}
                >
                  {t('backups.remoteTargets.discoverHostKey')}
                </Button>
                {discoveredFingerprint && (
                  <Button type="button" variant="ghost" onClick={() => setHostKeyFingerprint(discoveredFingerprint)}>
                    {t('backups.remoteTargets.confirmHostKey')}
                  </Button>
                )}
              </div>
              {discoveredFingerprint && (
                <p data-testid="backup-remote-target-host-key-fingerprint" className="text-xs text-content-secondary">
                  {discoveredFingerprint}
                </p>
              )}
              <Input
                id="backup-remote-target-host-key"
                data-testid="backup-remote-target-host-key-input"
                label={t('backups.remoteTargets.hostKeyFingerprint')}
                value={hostKeyFingerprint}
                onChange={(e) => setHostKeyFingerprint(e.target.value)}
              />
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="secondary" onClick={onClose} disabled={isSaving}>
            {t('common.cancel')}
          </Button>
          <Button variant="primary" onClick={handleSubmit} isLoading={isSaving}>
            {isEdit ? t('common.save') : t('common.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
