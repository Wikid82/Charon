import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  useCreateRemoteTarget,
  useUpdateRemoteTarget,
  useTestRemoteTarget,
  useTestDraftRemoteTarget,
  useStartRemoteTargetOAuth,
  type RemoteTarget,
  type RemoteTargetConfig,
  type RemoteTargetSecrets,
  type TestRemoteTargetResponse,
} from '../../hooks/useRemoteTargets'
import { toast } from '../../utils/toast'
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

import type { RemoteTargetType } from '../../api/backups'


export interface RemoteTargetFormDialogProps {
  /** Whether the dialog is open. */
  open: boolean
  /** The target being edited, or `null`/`undefined` when creating a new one. */
  target?: RemoteTarget | null
  onClose: () => void
}

/** Provider types whose create flow needs an OAuth "Connect" round trip (spec §3.6). */
const OAUTH_TYPES = new Set<RemoteTargetType>(['dropbox', 'google_drive'])

/**
 * Create/edit form for a remote storage target (spec §3.3.2, §3.6, §3.7, §3.8).
 * Five-way type switch (S3/SFTP/WebDAV/Dropbox/Google Drive); secret inputs
 * are `type="password"` and blank-on-edit ("leave blank to keep current") —
 * never pre-populated from the API, since the API never returns secrets.
 *
 * SFTP host-key discovery: when editing an existing target, `POST
 * .../remote-targets/:uuid/test` is used (with no `host_key_fingerprint`
 * stored yet) to discover the remote host's key without ever authenticating
 * (spec §3.7). When creating a brand-new target (no UUID/nothing saved yet),
 * the stateless `POST .../remote-targets/test-draft` endpoint is used
 * instead, built from the in-progress form fields. Both paths surface the
 * result the same way via `discoveredFingerprint`.
 *
 * Dropbox/Google Drive OAuth (spec §3.6): creating one of these two types
 * saves the target (no token yet, `oauth_status: "not_connected"`) and then
 * immediately starts the OAuth flow for the just-created UUID, full-page
 * redirecting to the provider's consent screen — there is no popup/postMessage
 * path (see spec §3.6 rationale: E2E-testability, popup-blocker fragility,
 * COOP forward-compatibility). If `oauth/start` itself fails (most commonly
 * `public_url_not_configured` — the admin hasn't set `app.public_url` in
 * System Settings yet), the target still exists in `not_connected` state for
 * a later retry from `RemoteTargetsCard`'s Connect button; this dialog just
 * surfaces the error and closes rather than redirecting.
 */
export function RemoteTargetFormDialog({ open, target, onClose }: RemoteTargetFormDialogProps) {
  const { t } = useTranslation()
  const isEdit = Boolean(target)
  const createMutation = useCreateRemoteTarget()
  const updateMutation = useUpdateRemoteTarget()
  const testMutation = useTestRemoteTarget()
  const testDraftMutation = useTestDraftRemoteTarget()
  const startOAuthMutation = useStartRemoteTargetOAuth()

  const [type, setType] = useState<RemoteTargetType>(target?.type ?? 's3')
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

  const [webdavUrl, setWebdavUrl] = useState(target?.config.webdav?.url ?? '')
  const [webdavUsername, setWebdavUsername] = useState(target?.config.webdav?.username ?? '')
  const [webdavBasePath, setWebdavBasePath] = useState(target?.config.webdav?.base_path ?? '')
  const [webdavInsecureSkipVerify, setWebdavInsecureSkipVerify] = useState(
    target?.config.webdav?.insecure_skip_verify ?? false
  )
  const [webdavPassword, setWebdavPassword] = useState('')

  const [dropboxAppKey, setDropboxAppKey] = useState(target?.config.dropbox?.app_key ?? '')
  const [dropboxFolderPath, setDropboxFolderPath] = useState(target?.config.dropbox?.folder_path ?? '')
  const [dropboxAppSecret, setDropboxAppSecret] = useState('')

  const [gdriveClientId, setGdriveClientId] = useState(target?.config.google_drive?.client_id ?? '')
  const [gdriveFolderPath, setGdriveFolderPath] = useState(target?.config.google_drive?.folder_path ?? '')
  const [gdriveClientSecret, setGdriveClientSecret] = useState('')

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
    setWebdavUrl(target?.config.webdav?.url ?? '')
    setWebdavUsername(target?.config.webdav?.username ?? '')
    setWebdavBasePath(target?.config.webdav?.base_path ?? '')
    setWebdavInsecureSkipVerify(target?.config.webdav?.insecure_skip_verify ?? false)
    setWebdavPassword('')
    setDropboxAppKey(target?.config.dropbox?.app_key ?? '')
    setDropboxFolderPath(target?.config.dropbox?.folder_path ?? '')
    setDropboxAppSecret('')
    setGdriveClientId(target?.config.google_drive?.client_id ?? '')
    setGdriveFolderPath(target?.config.google_drive?.folder_path ?? '')
    setGdriveClientSecret('')
    // Only re-seed when the dialog (re)opens or targets a different record.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, target?.uuid])

  const buildConfig = (): RemoteTargetConfig => {
    switch (type) {
      case 's3':
        return {
          endpoint,
          region,
          bucket,
          path_prefix: pathPrefix,
          use_ssl: useSSL,
          force_path_style: forcePathStyle,
        }
      case 'webdav':
        return {
          webdav: {
            url: webdavUrl,
            username: webdavUsername,
            base_path: webdavBasePath,
            insecure_skip_verify: webdavInsecureSkipVerify,
          },
        }
      case 'dropbox':
        return { dropbox: { app_key: dropboxAppKey, folder_path: dropboxFolderPath } }
      case 'google_drive':
        return { google_drive: { client_id: gdriveClientId, folder_path: gdriveFolderPath } }
      case 'sftp':
      default:
        return {
          host,
          port: parseInt(port, 10) || 0,
          path,
          username,
          host_key_fingerprint: hostKeyFingerprint || undefined,
        }
    }
  }

  const buildSecrets = (): RemoteTargetSecrets | undefined => {
    switch (type) {
      case 's3':
        if (!accessKeyId && !secretAccessKey) return undefined
        return {
          access_key_id: accessKeyId || undefined,
          secret_access_key: secretAccessKey || undefined,
        }
      case 'webdav':
        return webdavPassword ? { password: webdavPassword } : undefined
      case 'dropbox':
        return dropboxAppSecret ? { oauth_client_secret: dropboxAppSecret } : undefined
      case 'google_drive':
        return gdriveClientSecret ? { oauth_client_secret: gdriveClientSecret } : undefined
      case 'sftp':
      default:
        return password ? { password } : undefined
    }
  }

  const isSaving = createMutation.isPending || updateMutation.isPending || startOAuthMutation.isPending

  const startOAuthForNewTarget = (created: RemoteTarget) => {
    startOAuthMutation.mutate(created.uuid, {
      onSuccess: (result) => {
        // Full-page redirect (spec §3.6) — the page is navigating away
        // regardless, so onClose() is intentionally not called here.
        window.location.href = result.authorize_url
      },
      onError: (error: Error) => {
        // e.g. public_url_not_configured: the target still exists in
        // not_connected state for a later retry (spec §3.9); nothing left
        // to do in this dialog but surface the error and close.
        toast.error(error.message)
        onClose()
      },
    })
  }

  const handleSubmit = () => {
    const payload = { name, type, enabled: target?.enabled ?? true, config: buildConfig(), secrets: buildSecrets() }
    if (isEdit && target) {
      updateMutation.mutate(
        { uuid: target.uuid, payload },
        { onSuccess: onClose, onError: (error: Error) => toast.error(error.message) }
      )
    } else if (OAUTH_TYPES.has(type)) {
      createMutation.mutate(payload, {
        onSuccess: startOAuthForNewTarget,
        onError: (error: Error) => toast.error(error.message),
      })
    } else {
      createMutation.mutate(payload, {
        onSuccess: onClose,
        onError: (error: Error) => toast.error(error.message),
      })
    }
  }

  const submitLabel = isEdit
    ? t('common.save')
    : OAUTH_TYPES.has(type)
      ? t('backups.remoteTargets.saveAndConnect')
      : t('common.create')

  const handleDiscoverHostKeyCallbacks = {
    onSuccess: (result: TestRemoteTargetResponse) => {
      if (result.discovered_fingerprint) {
        setDiscoveredFingerprint(result.discovered_fingerprint)
      }
    },
    onError: (error: Error) => toast.error(error.message),
  }

  const handleDiscoverHostKey = () => {
    if (isEdit && target) {
      testMutation.mutate(target.uuid, handleDiscoverHostKeyCallbacks)
    } else {
      testDraftMutation.mutate(
        { type: 'sftp', config: { host, port: parseInt(port, 10) || 0, path, username } },
        handleDiscoverHostKeyCallbacks
      )
    }
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
                <label className="flex items-center gap-2 text-sm text-content-primary cursor-pointer">
                  <input
                    type="radio"
                    name="backup-remote-target-type"
                    value="webdav"
                    checked={type === 'webdav'}
                    onChange={() => setType('webdav')}
                  />
                  {t('backups.remoteTargets.typeWebdav')}
                </label>
                <label className="flex items-center gap-2 text-sm text-content-primary cursor-pointer">
                  <input
                    type="radio"
                    name="backup-remote-target-type"
                    value="dropbox"
                    checked={type === 'dropbox'}
                    onChange={() => setType('dropbox')}
                  />
                  {t('backups.remoteTargets.typeDropbox')}
                </label>
                <label className="flex items-center gap-2 text-sm text-content-primary cursor-pointer">
                  <input
                    type="radio"
                    name="backup-remote-target-type"
                    value="google_drive"
                    checked={type === 'google_drive'}
                    onChange={() => setType('google_drive')}
                  />
                  {t('backups.remoteTargets.typeGoogleDrive')}
                </label>
              </div>
            </fieldset>
          )}

          {type === 's3' && (
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
          )}

          {type === 'sftp' && (
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
                  isLoading={testMutation.isPending || testDraftMutation.isPending}
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

          {type === 'webdav' && (
            <div className="space-y-4">
              <Input
                id="backup-remote-target-webdav-url"
                label={t('backups.remoteTargets.webdavUrl')}
                value={webdavUrl}
                onChange={(e) => setWebdavUrl(e.target.value)}
              />
              <Input
                id="backup-remote-target-webdav-username"
                label={t('backups.remoteTargets.username')}
                value={webdavUsername}
                onChange={(e) => setWebdavUsername(e.target.value)}
              />
              <Input
                id="backup-remote-target-webdav-base-path"
                label={t('backups.remoteTargets.basePath')}
                value={webdavBasePath}
                onChange={(e) => setWebdavBasePath(e.target.value)}
              />
              <div className="flex items-center gap-2">
                <Checkbox
                  id="backup-remote-target-webdav-insecure-skip-verify"
                  checked={webdavInsecureSkipVerify}
                  onCheckedChange={(checked) => setWebdavInsecureSkipVerify(checked === true)}
                />
                <Label htmlFor="backup-remote-target-webdav-insecure-skip-verify">
                  {t('backups.remoteTargets.insecureSkipVerify')}
                </Label>
              </div>
              <Input
                id="backup-remote-target-webdav-password"
                label={t('backups.remoteTargets.password')}
                type="password"
                value={webdavPassword}
                onChange={(e) => setWebdavPassword(e.target.value)}
                placeholder={isEdit ? t('backups.remoteTargets.keepCurrent') : undefined}
                autoComplete="new-password"
              />
              {isEdit && (
                <p className="text-xs text-content-muted">{t('backups.remoteTargets.leaveBlankToKeepCurrent')}</p>
              )}
            </div>
          )}

          {type === 'dropbox' && (
            <div className="space-y-4">
              <Input
                id="backup-remote-target-app-key"
                label={t('backups.remoteTargets.dropboxAppKey')}
                value={dropboxAppKey}
                onChange={(e) => setDropboxAppKey(e.target.value)}
              />
              <Input
                id="backup-remote-target-app-secret"
                label={t('backups.remoteTargets.dropboxAppSecret')}
                type="password"
                value={dropboxAppSecret}
                onChange={(e) => setDropboxAppSecret(e.target.value)}
                placeholder={isEdit ? t('backups.remoteTargets.keepCurrent') : undefined}
                autoComplete="new-password"
              />
              <Input
                id="backup-remote-target-folder-path"
                label={t('backups.remoteTargets.folderPath')}
                value={dropboxFolderPath}
                onChange={(e) => setDropboxFolderPath(e.target.value)}
              />
              {isEdit && (
                <p className="text-xs text-content-muted">{t('backups.remoteTargets.leaveBlankToKeepCurrent')}</p>
              )}
            </div>
          )}

          {type === 'google_drive' && (
            <div className="space-y-4">
              <Input
                id="backup-remote-target-client-id"
                label={t('backups.remoteTargets.googleDriveClientId')}
                value={gdriveClientId}
                onChange={(e) => setGdriveClientId(e.target.value)}
              />
              <Input
                id="backup-remote-target-client-secret"
                label={t('backups.remoteTargets.googleDriveClientSecret')}
                type="password"
                value={gdriveClientSecret}
                onChange={(e) => setGdriveClientSecret(e.target.value)}
                placeholder={isEdit ? t('backups.remoteTargets.keepCurrent') : undefined}
                autoComplete="new-password"
              />
              <Input
                id="backup-remote-target-folder-path"
                label={t('backups.remoteTargets.folderPath')}
                value={gdriveFolderPath}
                onChange={(e) => setGdriveFolderPath(e.target.value)}
              />
              {isEdit && (
                <p className="text-xs text-content-muted">{t('backups.remoteTargets.leaveBlankToKeepCurrent')}</p>
              )}
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="secondary" onClick={onClose} disabled={isSaving}>
            {t('common.cancel')}
          </Button>
          <Button variant="primary" onClick={handleSubmit} isLoading={isSaving}>
            {submitLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
