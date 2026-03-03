import { useState, useEffect, useCallback, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Switch } from '../components/ui/Switch'
import { Alert, AlertDescription } from '../components/ui/Alert'
import { Label } from '../components/ui/Label'
import { toast } from '../utils/toast'
import client from '../api/client'
import {
  listUsers,
  inviteUser,
  deleteUser,
  updateUser,
  updateUserPermissions,
  resendInvite,
  getProfile,
  updateProfile,
  regenerateApiKey,
} from '../api/users'
import type { User, InviteUserRequest, PermissionMode, UpdateUserPermissionsRequest } from '../api/users'
import { getProxyHosts } from '../api/proxyHosts'
import type { ProxyHost } from '../api/proxyHosts'
import { useAuth } from '../hooks/useAuth'
import {
  Users,
  UserPlus,
  Mail,
  Shield,
  Trash2,
  Settings,
  X,
  Check,
  AlertCircle,
  Clock,
  Copy,
  Loader2,
  ExternalLink,
  AlertTriangle,
  Pencil,
  Key,
  Lock,
  UserCircle,
} from 'lucide-react'

interface InviteModalProps {
  isOpen: boolean
  onClose: () => void
  proxyHosts: ProxyHost[]
}

function InviteModal({ isOpen, onClose, proxyHosts }: InviteModalProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [email, setEmail] = useState('')
  const [emailError, setEmailError] = useState<string | null>(null)
  const [role, setRole] = useState<'user' | 'admin' | 'passthrough'>('user')
  const [permissionMode, setPermissionMode] = useState<PermissionMode>('allow_all')
  const [selectedHosts, setSelectedHosts] = useState<number[]>([])
  const [inviteResult, setInviteResult] = useState<{
    inviteUrl: string
    emailSent: boolean
    expiresAt: string
  } | null>(null)
  const hasUsableInviteUrl = (inviteUrl?: string): inviteUrl is string => {
    const normalized = (inviteUrl ?? '').trim()
    return normalized.length > 0 && normalized !== '[REDACTED]'
  }

  const [urlPreview, setUrlPreview] = useState<{
    preview_url: string
    base_url: string
    is_configured: boolean
    warning: boolean
    warning_message: string
  } | null>(null)

  const validateEmail = (emailValue: string): boolean => {
    if (!emailValue) {
      setEmailError(null)
      return false
    }
    const emailRegex = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/
    if (!emailRegex.test(emailValue)) {
      setEmailError(t('users.invalidEmail'))
      return false
    }
    setEmailError(null)
    return true
  }

  // Keyboard navigation - close on Escape
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }

    if (isOpen) {
      document.addEventListener('keydown', handleKeyDown)
      return () => document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen, onClose])

  // Fetch preview when email changes
  useEffect(() => {
    if (email && email.includes('@')) {
      const fetchPreview = async () => {
        try {
          const response = await client.post('/users/preview-invite-url', { email })
          setUrlPreview(response.data)
        } catch {
          setUrlPreview(null)
        }
      }
      const debounce = setTimeout(fetchPreview, 500)
      return () => clearTimeout(debounce)
    } else {
      setUrlPreview(null)
    }
  }, [email])

  const inviteMutation = useMutation({
    mutationFn: async () => {
      const request: InviteUserRequest = {
        email,
        role,
        permission_mode: permissionMode,
        permitted_hosts: selectedHosts,
      }
      return inviteUser(request)
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setInviteResult({
        inviteUrl: data.invite_url ?? '',
        emailSent: data.email_sent,
        expiresAt: data.expires_at,
      })
      if (data.email_sent) {
        toast.success(t('users.inviteSent'))
      } else {
        toast.success(t('users.inviteCreated'))
      }
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || t('users.inviteFailed'))
    },
  })

  const copyInviteLink = async () => {
    if (inviteResult?.inviteUrl && hasUsableInviteUrl(inviteResult.inviteUrl)) {
      const link = inviteResult.inviteUrl

      try {
        await navigator.clipboard.writeText(link)
      } catch {
        const textarea = document.createElement('textarea')
        textarea.value = link
        textarea.setAttribute('readonly', 'true')
        textarea.style.position = 'absolute'
        textarea.style.left = '-9999px'
        document.body.appendChild(textarea)
        textarea.select()
        document.execCommand('copy')
        document.body.removeChild(textarea)
      }

      toast.success(t('users.inviteLinkCopied'))
    }
  }

  const handleClose = () => {
    setEmail('')
    setEmailError(null)
    setRole('user' as 'user' | 'admin' | 'passthrough')
    setPermissionMode('allow_all')
    setSelectedHosts([])
    setInviteResult(null)
    setUrlPreview(null)
    onClose()
  }

  const toggleHost = (hostId: number) => {
    setSelectedHosts((prev) =>
      prev.includes(hostId) ? prev.filter((id) => id !== hostId) : [...prev, hostId]
    )
  }

  if (!isOpen) return null

  return (
    <>
      {/* Layer 1: Background overlay (z-40) */}
      <div className="fixed inset-0 bg-black/50 z-40" onClick={handleClose} />

      {/* Layer 2: Form container (z-50, pointer-events-none) */}
      <div className="fixed inset-0 flex items-center justify-center pointer-events-none z-50">

        {/* Layer 3: Form content (pointer-events-auto) */}
        <div
          className="bg-dark-card border border-gray-800 rounded-lg w-full max-w-lg max-h-[90vh] overflow-y-auto pointer-events-auto"
          role="dialog"
          aria-modal="true"
          aria-labelledby="invite-modal-title"
        >
        <div className="flex items-center justify-between p-4 border-b border-gray-800">
          <h3 id="invite-modal-title" className="text-lg font-semibold text-white flex items-center gap-2">
            <UserPlus className="h-5 w-5" />
            {t('users.inviteUser')}
          </h3>
          <button onClick={handleClose} className="text-gray-400 hover:text-white" aria-label={t('common.close')}>
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="p-4 space-y-4">
          {inviteResult ? (
            <div className="space-y-4">
              <div className="bg-green-900/20 border border-green-800 rounded-lg p-4">
                <div className="flex items-center gap-2 text-green-400 mb-2">
                  <Check className="h-5 w-5" />
                  <span className="font-medium">{t('users.inviteSuccess')}</span>
                </div>
                {inviteResult.emailSent ? (
                  <p className="text-sm text-gray-300">
                    {t('users.inviteEmailSent')}
                  </p>
                ) : (
                  <p className="text-sm text-gray-300">
                    {t('users.inviteEmailNotSent')}
                  </p>
                )}
              </div>

              <div className="space-y-2">
                <label className="block text-sm font-medium text-gray-300">
                  {t('users.inviteLink')}
                </label>
                {hasUsableInviteUrl(inviteResult.inviteUrl) ? (
                  <div className="flex gap-2">
                    <Input
                      type="text"
                      value={inviteResult.inviteUrl}
                      readOnly
                      className="flex-1 text-sm"
                    />
                    <Button onClick={copyInviteLink} aria-label={t('users.copyInviteLink')} title={t('users.copyInviteLink')}>
                      <Copy className="h-4 w-4" />
                    </Button>
                  </div>
                ) : (
                  <p className="text-sm text-gray-300">
                    {t('users.inviteLinkHiddenForSecurity', { defaultValue: 'Invite link is hidden for security. Share the invite through configured email delivery.' })}
                  </p>
                )}
                <p className="text-xs text-gray-500">
                  {t('users.expires')}: {new Date(inviteResult.expiresAt).toLocaleString()}
                </p>
              </div>

              <Button onClick={handleClose} className="w-full">
                {t('users.done')}
              </Button>
            </div>
          ) : (
            <>
              <div>
                <Input
                  label={t('users.emailAddress')}
                  type="email"
                  value={email}
                  onChange={(e) => {
                    setEmail(e.target.value)
                    validateEmail(e.target.value)
                  }}
                  placeholder="user@example.com"
                />
                {emailError && (
                  <p className="mt-1 text-xs text-red-400" role="alert">
                    {emailError}
                  </p>
                )}
              </div>

              <div className="w-full">
                <label htmlFor="invite-user-role" className="block text-sm font-medium text-gray-300 mb-1.5">
                  {t('users.role')}
                </label>
                <select
                  id="invite-user-role"
                  value={role}
                  onChange={(e) => setRole(e.target.value as 'user' | 'admin' | 'passthrough')}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="user">{t('users.roleUser')}</option>
                  <option value="admin">{t('users.roleAdmin')}</option>
                  <option value="passthrough">{t('users.rolePassthrough')}</option>
                </select>
                <p className="mt-1 text-xs text-gray-500">
                  {role === 'admin' && t('users.roleAdminDescription')}
                  {role === 'user' && t('users.roleUserDescription')}
                  {role === 'passthrough' && t('users.rolePassthroughDescription')}
                </p>
              </div>

              {(role === 'user' || role === 'passthrough') && (
                <>
                  <div className="w-full">
                    <label htmlFor="invite-permission-mode" className="block text-sm font-medium text-gray-300 mb-1.5">
                      {t('users.permissionMode')}
                    </label>
                    <select
                      id="invite-permission-mode"
                      value={permissionMode}
                      onChange={(e) => setPermissionMode(e.target.value as PermissionMode)}
                      className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                      <option value="allow_all">{t('users.allowAllBlacklist')}</option>
                      <option value="deny_all">{t('users.denyAllWhitelist')}</option>
                    </select>
                    <p className="mt-1 text-xs text-gray-500">
                      {permissionMode === 'allow_all'
                        ? t('users.allowAllDescription')
                        : t('users.denyAllDescription')}
                    </p>
                  </div>

                  <div className="w-full">
                    <label className="block text-sm font-medium text-gray-300 mb-1.5">
                      {permissionMode === 'allow_all' ? t('users.blockedHosts') : t('users.allowedHosts')}
                    </label>
                    <div className="max-h-48 overflow-y-auto border border-gray-700 rounded-lg">
                      {proxyHosts.length === 0 ? (
                        <p className="p-3 text-sm text-gray-500">{t('users.noProxyHosts')}</p>
                      ) : (
                        proxyHosts.map((host) => (
                          <label
                            key={host.uuid}
                            className="flex items-center gap-3 p-3 hover:bg-gray-800/50 cursor-pointer border-b border-gray-800 last:border-0"
                          >
                            <input
                              type="checkbox"
                              checked={selectedHosts.includes(
                                parseInt(host.uuid.split('-')[0], 16) || 0
                              )}
                              onChange={() =>
                                toggleHost(parseInt(host.uuid.split('-')[0], 16) || 0)
                              }
                              className="rounded bg-gray-900 border-gray-700 text-blue-500 focus:ring-blue-500"
                            />
                            <div>
                              <p className="text-sm text-white">{host.name || host.domain_names}</p>
                              <p className="text-xs text-gray-500">{host.domain_names}</p>
                            </div>
                          </label>
                        ))
                      )}
                    </div>
                  </div>
                </>
              )}

              {/* URL Preview */}
              {urlPreview && (
                <div className="space-y-2 p-4 bg-gray-900/50 rounded-lg border border-gray-700">
                  <div className="flex items-center gap-2">
                    <ExternalLink className="h-4 w-4 text-gray-400" />
                    <Label className="text-sm font-medium text-gray-300">
                      {t('users.inviteUrlPreview')}
                    </Label>
                  </div>
                  <div className="text-sm font-mono text-gray-400 break-all bg-gray-950 p-2 rounded">
                    {urlPreview.preview_url.replace('SAMPLE_TOKEN_PREVIEW', '...')}
                  </div>
                  {urlPreview.warning && (
                    <Alert variant="warning" className="mt-2">
                      <AlertTriangle className="h-4 w-4" />
                      <AlertDescription className="text-xs">
                        {t('users.inviteUrlWarning')}
                        <Link to="/settings/system" className="ml-1 underline">
                          {t('users.configureApplicationUrl')}
                        </Link>
                      </AlertDescription>
                    </Alert>
                  )}
                </div>
              )}

              <div className="flex gap-3 pt-4 border-t border-gray-700">
                <Button variant="secondary" onClick={handleClose} className="flex-1">
                  {t('common.cancel')}
                </Button>
                <Button
                  onClick={() => inviteMutation.mutate()}
                  isLoading={inviteMutation.isPending}
                  disabled={!email || !!emailError}
                  className="flex-1"
                >
                  <Mail className="h-4 w-4 mr-2" />
                  {t('users.sendInvite')}
                </Button>
              </div>
            </>
          )}
        </div>
        </div>
      </div>
    </>
  )
}

interface PermissionsModalProps {
  isOpen: boolean
  onClose: () => void
  user: User | null
  proxyHosts: ProxyHost[]
}

function PermissionsModal({ isOpen, onClose, user, proxyHosts }: PermissionsModalProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [permissionMode, setPermissionMode] = useState<PermissionMode>('allow_all')
  const [selectedHosts, setSelectedHosts] = useState<number[]>([])

  // Update state when user changes
  useEffect(() => {
    if (user) {
      setPermissionMode(user.permission_mode || 'allow_all')
      setSelectedHosts(user.permitted_hosts || [])
    }
  }, [user])

  // Keyboard navigation - close on Escape
  const handleClose = useCallback(() => {
    onClose()
  }, [onClose])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        handleClose()
      }
    }

    if (isOpen) {
      document.addEventListener('keydown', handleKeyDown)
      return () => document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen, handleClose])

  const updatePermissionsMutation = useMutation({
    mutationFn: async () => {
      if (!user) return
      const request: UpdateUserPermissionsRequest = {
        permission_mode: permissionMode,
        permitted_hosts: selectedHosts,
      }
      return updateUserPermissions(user.id, request)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success(t('users.permissionsUpdated'))
      onClose()
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || t('users.permissionsUpdateFailed'))
    },
  })

  const toggleHost = (hostId: number) => {
    setSelectedHosts((prev) =>
      prev.includes(hostId) ? prev.filter((id) => id !== hostId) : [...prev, hostId]
    )
  }

  if (!isOpen || !user) return null

  return (
    <>
      {/* Layer 1: Background overlay (z-40) */}
      <div className="fixed inset-0 bg-black/50 z-40" onClick={onClose} />

      {/* Layer 2: Form container (z-50, pointer-events-none) */}
      <div className="fixed inset-0 flex items-center justify-center pointer-events-none z-50">

        {/* Layer 3: Form content (pointer-events-auto) */}
        <div
          className="bg-dark-card border border-gray-800 rounded-lg w-full max-w-lg max-h-[90vh] overflow-y-auto pointer-events-auto"
          role="dialog"
          aria-modal="true"
          aria-labelledby="permissions-modal-title"
        >
        <div className="flex items-center justify-between p-4 border-b border-gray-800">
          <h3 id="permissions-modal-title" className="text-lg font-semibold text-white flex items-center gap-2">
            <Shield className="h-5 w-5" />
            {t('users.editPermissions')} - {user.name || user.email}
          </h3>
          <button onClick={onClose} className="text-gray-400 hover:text-white" aria-label={t('common.close')}>
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="p-4 space-y-4">
          <div className="w-full">
            <label htmlFor="edit-permission-mode" className="block text-sm font-medium text-gray-300 mb-1.5">
              {t('users.permissionMode')}
            </label>
            <select
              id="edit-permission-mode"
              value={permissionMode}
              onChange={(e) => setPermissionMode(e.target.value as PermissionMode)}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="allow_all">{t('users.allowAllBlacklist')}</option>
              <option value="deny_all">{t('users.denyAllWhitelist')}</option>
            </select>
            <p className="mt-1 text-xs text-gray-500">
              {permissionMode === 'allow_all'
                ? t('users.allowAllDescription')
                : t('users.denyAllDescription')}
            </p>
          </div>

          <div className="w-full">
            <label className="block text-sm font-medium text-gray-300 mb-1.5">
              {permissionMode === 'allow_all' ? t('users.blockedHosts') : t('users.allowedHosts')}
            </label>
            <div className="max-h-64 overflow-y-auto border border-gray-700 rounded-lg">
              {proxyHosts.length === 0 ? (
                <p className="p-3 text-sm text-gray-500">{t('users.noProxyHosts')}</p>
              ) : (
                proxyHosts.map((host) => (
                  <label
                    key={host.uuid}
                    className="flex items-center gap-3 p-3 hover:bg-gray-800/50 cursor-pointer border-b border-gray-800 last:border-0"
                  >
                    <input
                      type="checkbox"
                      checked={selectedHosts.includes(
                        parseInt(host.uuid.split('-')[0], 16) || 0
                      )}
                      onChange={() =>
                        toggleHost(parseInt(host.uuid.split('-')[0], 16) || 0)
                      }
                      className="rounded bg-gray-900 border-gray-700 text-blue-500 focus:ring-blue-500"
                    />
                    <div>
                      <p className="text-sm text-white">{host.name || host.domain_names}</p>
                      <p className="text-xs text-gray-500">{host.domain_names}</p>
                    </div>
                  </label>
                ))
              )}
            </div>
          </div>

          <div className="flex gap-3 pt-4 border-t border-gray-700">
            <Button variant="secondary" onClick={onClose} className="flex-1">
              {t('common.cancel')}
            </Button>
            <Button
              onClick={() => updatePermissionsMutation.mutate()}
              isLoading={updatePermissionsMutation.isPending}
              className="flex-1"
            >
              {t('users.savePermissions')}
            </Button>
          </div>
        </div>
        </div>
      </div>
    </>
  )
}

interface UserDetailModalProps {
  isOpen: boolean
  onClose: () => void
  user: User | null
  isSelf: boolean
}

function UserDetailModal({ isOpen, onClose, user, isSelf }: UserDetailModalProps) {
  const { t } = useTranslation()
  const { changePassword } = useAuth()
  const queryClient = useQueryClient()
  const dialogRef = useRef<HTMLDivElement>(null)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [showPasswordSection, setShowPasswordSection] = useState(false)
  const [apiKeyMasked, setApiKeyMasked] = useState('')

  useEffect(() => {
    if (user) {
      setName(user.name || '')
      setEmail(user.email || '')
      setShowPasswordSection(false)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    }
  }, [user])

  // Fetch profile for API key info when editing self
  const { data: profile } = useQuery({
    queryKey: ['profile'],
    queryFn: getProfile,
    enabled: isOpen && isSelf,
  })

  useEffect(() => {
    if (profile) {
      setApiKeyMasked(profile.api_key_masked || '')
    }
  }, [profile])

  // Focus trap and Escape handling
  useEffect(() => {
    if (!isOpen) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
        return
      }
      // Focus trap
      if (e.key === 'Tab' && dialogRef.current) {
        const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        )
        if (focusable.length === 0) return
        const first = focusable[0]
        const last = focusable[focusable.length - 1]
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault()
          last.focus()
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault()
          first.focus()
        }
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    // Focus first focusable element on open
    requestAnimationFrame(() => {
      const first = dialogRef.current?.querySelector<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      )
      first?.focus()
    })

    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onClose])

  const profileMutation = useMutation({
    mutationFn: async () => {
      if (isSelf) {
        return updateProfile({ name, email })
      }
      return updateUser(user!.id, { name, email })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      if (isSelf) queryClient.invalidateQueries({ queryKey: ['profile'] })
      toast.success(t('users.profileUpdated'))
      onClose()
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || t('users.profileUpdateFailed'))
    },
  })

  const passwordMutation = useMutation({
    mutationFn: async () => {
      if (newPassword !== confirmPassword) {
        throw new Error(t('users.passwordMismatch'))
      }
      return changePassword(currentPassword, newPassword)
    },
    onSuccess: () => {
      toast.success(t('users.passwordChanged'))
      setShowPasswordSection(false)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    },
    onError: (error: unknown) => {
      const err = error as { message?: string }
      toast.error(err.message || t('users.passwordChangeFailed'))
    },
  })

  const regenApiKeyMutation = useMutation({
    mutationFn: regenerateApiKey,
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      setApiKeyMasked(data.api_key_masked)
      toast.success(t('users.apiKeyRegenerated'))
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || t('users.apiKeyRegenerateFailed'))
    },
  })

  if (!isOpen || !user) return null

  return (
    <>
      <div className="fixed inset-0 bg-black/50 z-40" onClick={onClose} />
      <div className="fixed inset-0 flex items-center justify-center pointer-events-none z-50">
        <div
          ref={dialogRef}
          className="bg-dark-card border border-gray-800 rounded-lg w-full max-w-lg max-h-[90vh] overflow-y-auto pointer-events-auto"
          role="dialog"
          aria-modal="true"
          aria-labelledby="user-detail-modal-title"
        >
          <div className="flex items-center justify-between p-4 border-b border-gray-800">
            <h3 id="user-detail-modal-title" className="text-lg font-semibold text-white flex items-center gap-2">
              <Pencil className="h-5 w-5" />
              {isSelf ? t('users.myProfile') : t('users.editUser')}
            </h3>
            <button onClick={onClose} className="text-gray-400 hover:text-white" aria-label={t('common.close')}>
              <X className="h-5 w-5" />
            </button>
          </div>

          <div className="p-4 space-y-4">
            {/* Name & Email */}
            <div>
              <Input
                label={t('common.name')}
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            <div>
              <Input
                label={t('users.emailAddress')}
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>

            <div className="flex gap-3 pt-2">
              <Button
                onClick={() => profileMutation.mutate()}
                isLoading={profileMutation.isPending}
                className="flex-1"
              >
                {t('common.save')}
              </Button>
            </div>

            {/* Password Section — self only */}
            {isSelf && (
              <div className="border-t border-gray-700 pt-4">
                <button
                  onClick={() => setShowPasswordSection(!showPasswordSection)}
                  className="flex items-center gap-2 text-sm font-medium text-gray-300 hover:text-white"
                >
                  <Lock className="h-4 w-4" />
                  {t('users.changePassword')}
                </button>
                {showPasswordSection && (
                  <div className="mt-3 space-y-3">
                    <Input
                      label={t('users.currentPassword')}
                      type="password"
                      value={currentPassword}
                      onChange={(e) => setCurrentPassword(e.target.value)}
                    />
                    <Input
                      label={t('users.newPassword')}
                      type="password"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                    />
                    <Input
                      label={t('users.confirmPassword')}
                      type="password"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                    />
                    {newPassword && confirmPassword && newPassword !== confirmPassword && (
                      <p className="text-xs text-red-400" role="alert">{t('users.passwordMismatch')}</p>
                    )}
                    <Button
                      onClick={() => passwordMutation.mutate()}
                      isLoading={passwordMutation.isPending}
                      disabled={!currentPassword || !newPassword || newPassword !== confirmPassword}
                      variant="secondary"
                    >
                      {t('users.changePassword')}
                    </Button>
                  </div>
                )}
              </div>
            )}

            {/* API Key Section — self only */}
            {isSelf && (
              <div className="border-t border-gray-700 pt-4">
                <div className="flex items-center gap-2 mb-2">
                  <Key className="h-4 w-4 text-gray-400" />
                  <span className="text-sm font-medium text-gray-300">{t('users.apiKey')}</span>
                </div>
                {apiKeyMasked && (
                  <p className="text-sm font-mono text-gray-500 mb-2">{apiKeyMasked}</p>
                )}
                <Button
                  variant="secondary"
                  onClick={() => {
                    if (confirm(t('users.apiKeyConfirm'))) {
                      regenApiKeyMutation.mutate()
                    }
                  }}
                  isLoading={regenApiKeyMutation.isPending}
                >
                  {t('users.regenerateApiKey')}
                </Button>
              </div>
            )}
          </div>
        </div>
      </div>
    </>
  )
}

export default function UsersPage() {
  const { t } = useTranslation()
  const { user: authUser } = useAuth()
  const queryClient = useQueryClient()
  const [inviteModalOpen, setInviteModalOpen] = useState(false)
  const [permissionsModalOpen, setPermissionsModalOpen] = useState(false)
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [detailModalOpen, setDetailModalOpen] = useState(false)
  const [detailUser, setDetailUser] = useState<User | null>(null)
  const [isSelfEdit, setIsSelfEdit] = useState(false)

  const { data: users, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: listUsers,
  })

  const { data: proxyHosts = [] } = useQuery({
    queryKey: ['proxyHosts'],
    queryFn: getProxyHosts,
  })

  const toggleEnabledMutation = useMutation({
    mutationFn: async ({ id, enabled }: { id: number; enabled: boolean }) => {
      return updateUser(id, { enabled })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success(t('users.userUpdated'))
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || t('users.userUpdateFailed'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success(t('users.userDeleted'))
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || t('users.userDeleteFailed'))
    },
  })

  const resendInviteMutation = useMutation({
    mutationFn: resendInvite,
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      if (data.email_sent) {
        toast.success(t('users.inviteResent'))
      } else {
        toast.success(t('users.inviteCreatedNoEmail'))
      }
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || t('users.resendFailed'))
    },
  })

  const openPermissions = (user: User) => {
    setSelectedUser(user)
    setPermissionsModalOpen(true)
  }

  const openDetail = (user: User, self: boolean) => {
    setDetailUser(user)
    setIsSelfEdit(self)
    setDetailModalOpen(true)
  }

  const currentUser = users?.find((u) => u.id === authUser?.user_id)

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-8 w-8 animate-spin text-blue-500" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Users className="h-6 w-6 text-blue-500" />
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">{t('users.title')}</h1>
        </div>
        <Button onClick={() => setInviteModalOpen(true)}>
          <UserPlus className="h-4 w-4 mr-2" />
          {t('users.inviteUser')}
        </Button>
      </div>

      {/* My Profile Card */}
      {currentUser && (
        <Card>
          <div className="flex items-center justify-between p-4">
            <div className="flex items-center gap-3">
              <UserCircle className="h-10 w-10 text-blue-500" />
              <div>
                <h2 className="text-sm font-semibold text-white">{t('users.myProfile')}</h2>
                <p className="text-sm text-white">{currentUser.name || t('users.noName')}</p>
                <p className="text-xs text-gray-500">{currentUser.email}</p>
              </div>
            </div>
            <Button variant="secondary" onClick={() => openDetail(currentUser, true)}>
              <Pencil className="h-4 w-4 mr-2" />
              {t('users.editUser')}
            </Button>
          </div>
        </Card>
      )}

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-800">
                <th scope="col" className="text-left py-3 px-4 text-sm font-medium text-gray-400">{t('users.columnUser')}</th>
                <th scope="col" className="text-left py-3 px-4 text-sm font-medium text-gray-400">{t('users.columnRole')}</th>
                <th scope="col" className="text-left py-3 px-4 text-sm font-medium text-gray-400">{t('common.status')}</th>
                <th scope="col" className="text-left py-3 px-4 text-sm font-medium text-gray-400">{t('users.columnPermissions')}</th>
                <th scope="col" className="text-left py-3 px-4 text-sm font-medium text-gray-400">{t('common.enabled')}</th>
                <th scope="col" className="text-right py-3 px-4 text-sm font-medium text-gray-400">{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {users?.map((user) => (
                <tr key={user.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="py-3 px-4">
                    <div>
                      <p className="text-sm font-medium text-white">{user.name || t('users.noName')}</p>
                      <p className="text-xs text-gray-500">{user.email}</p>
                    </div>
                  </td>
                  <td className="py-3 px-4">
                    <span
                      className={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${
                        user.role === 'admin'
                          ? 'bg-purple-900/30 text-purple-400'
                          : user.role === 'passthrough'
                            ? 'bg-gray-900/30 text-gray-400'
                            : 'bg-blue-900/30 text-blue-400'
                      }`}
                    >
                      {user.role === 'admin' && t('users.roleAdmin')}
                      {user.role === 'user' && t('users.roleUser')}
                      {user.role === 'passthrough' && t('users.rolePassthrough')}
                    </span>
                  </td>
                  <td className="py-3 px-4">
                    {user.invite_status === 'pending' ? (
                      <span className="inline-flex items-center gap-1 text-yellow-400 text-xs">
                        <Clock className="h-3 w-3" />
                        {t('users.pendingInvite')}
                      </span>
                    ) : user.invite_status === 'expired' ? (
                      <span className="inline-flex items-center gap-1 text-red-400 text-xs">
                        <AlertCircle className="h-3 w-3" />
                        {t('users.inviteExpired')}
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-green-400 text-xs">
                        <Check className="h-3 w-3" />
                        {t('common.active')}
                      </span>
                    )}
                  </td>
                  <td className="py-3 px-4">
                    <span className="text-xs text-gray-400">
                      {user.permission_mode === 'deny_all' ? t('users.whitelist') : t('users.blacklist')}
                    </span>
                  </td>
                  <td className="py-3 px-4">
                    <Switch
                      checked={user.enabled}
                      onChange={() =>
                        toggleEnabledMutation.mutate({
                          id: user.id,
                          enabled: !user.enabled,
                        })
                      }
                      disabled={user.role === 'admin'}
                    />
                  </td>
                  <td className="py-3 px-4">
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => openDetail(user, user.id === authUser?.user_id)}
                        className="p-1.5 text-gray-400 hover:text-white hover:bg-gray-800 rounded"
                        title={t('users.editUser')}
                        aria-label={t('users.editUser')}
                      >
                        <Pencil className="h-4 w-4" />
                      </button>
                      {user.invite_status === 'pending' && (
                        <button
                          onClick={() => resendInviteMutation.mutate(user.id)}
                          className="p-1.5 text-gray-400 hover:text-blue-400 hover:bg-gray-800 rounded"
                          title={t('users.resendInvite')}
                          aria-label={t('users.resendInvite')}
                          disabled={resendInviteMutation.isPending}
                        >
                          <Mail className="h-4 w-4" />
                        </button>
                      )}
                      {user.role !== 'admin' && (
                        <button
                          onClick={() => openPermissions(user)}
                          className="p-1.5 text-gray-400 hover:text-white hover:bg-gray-800 rounded"
                          title={t('users.editPermissions')}
                          aria-label={t('users.editPermissions')}
                        >
                          <Settings className="h-4 w-4" />
                        </button>
                      )}
                      <button
                        onClick={() => {
                          if (confirm(t('users.deleteConfirm'))) {
                            deleteMutation.mutate(user.id)
                          }
                        }}
                        className="p-1.5 text-gray-400 hover:text-red-400 hover:bg-gray-800 rounded"
                        title={t('users.deleteUser')}
                        aria-label={t('users.deleteUser')}
                        disabled={user.role === 'admin'}
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
      </Card>

      <InviteModal
        isOpen={inviteModalOpen}
        onClose={() => setInviteModalOpen(false)}
        proxyHosts={proxyHosts}
      />

      <PermissionsModal
        isOpen={permissionsModalOpen}
        onClose={() => {
          setPermissionsModalOpen(false)
          setSelectedUser(null)
        }}
        user={selectedUser}
        proxyHosts={proxyHosts}
      />

      <UserDetailModal
        isOpen={detailModalOpen}
        onClose={() => {
          setDetailModalOpen(false)
          setDetailUser(null)
        }}
        user={detailUser}
        isSelf={isSelfEdit}
      />
    </div>
  )
}
