import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { Button } from '../components/ui/Button'
import { Label } from '../components/ui/Label'
import { Alert } from '../components/ui/Alert'
import { Checkbox } from '../components/ui/Checkbox'
import { Skeleton } from '../components/ui/Skeleton'
import { toast } from '../utils/toast'
import { getProfile, regenerateApiKey, updateProfile } from '../api/user'
import { getSettings, updateSetting } from '../api/settings'
import { Copy, RefreshCw, Shield, Mail, User, AlertTriangle, Key } from 'lucide-react'
import { PasswordStrengthMeter } from '../components/PasswordStrengthMeter'
import { isValidEmail } from '../utils/validation'
import { useAuth } from '../hooks/useAuth'

export default function Account() {
  const { t } = useTranslation()
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)

  // Profile State
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [emailValid, setEmailValid] = useState<boolean | null>(null)
  const [confirmPasswordForUpdate, setConfirmPasswordForUpdate] = useState('')
  const [showPasswordPrompt, setShowPasswordPrompt] = useState(false)
  const [pendingProfileUpdate, setPendingProfileUpdate] = useState<{name: string, email: string} | null>(null)
  const [previousEmail, setPreviousEmail] = useState('')
  const [showEmailConfirmModal, setShowEmailConfirmModal] = useState(false)

  // Certificate Email State
  const [certEmail, setCertEmail] = useState('')
  const [certEmailValid, setCertEmailValid] = useState<boolean | null>(null)
  const [useUserEmail, setUseUserEmail] = useState(true)
  const [certEmailInitialized, setCertEmailInitialized] = useState(false)

  const queryClient = useQueryClient()
  const { changePassword } = useAuth()

  const { data: profile, isLoading: isLoadingProfile } = useQuery({
    queryKey: ['profile'],
    queryFn: getProfile,
  })

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
  })

  // Initialize profile state
  useEffect(() => {
    if (profile) {
      setName(profile.name)
      setEmail(profile.email)
    }
  }, [profile])

  // Validate profile email
  useEffect(() => {
    if (email) {
      setEmailValid(isValidEmail(email))
    } else {
      setEmailValid(null)
    }
  }, [email])

  // Initialize cert email state only once, when both settings and profile are loaded
  useEffect(() => {
    if (!certEmailInitialized && settings && profile) {
      const savedEmail = settings['caddy.email']
      if (savedEmail && savedEmail !== profile.email) {
        setCertEmail(savedEmail)
        setUseUserEmail(false)
      } else {
        setCertEmail(profile.email)
        setUseUserEmail(true)
      }
      setCertEmailInitialized(true)
    }
  }, [settings, profile, certEmailInitialized])

  // Validate cert email
  useEffect(() => {
    if (certEmail && !useUserEmail) {
      setCertEmailValid(isValidEmail(certEmail))
    } else {
      setCertEmailValid(null)
    }
  }, [certEmail, useUserEmail])

  const updateProfileMutation = useMutation({
    mutationFn: updateProfile,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      toast.success(t('account.profileUpdated'))
    },
    onError: (error: Error) => {
      toast.error(t('account.profileUpdateFailed', { error: error.message }))
    },
  })

  const updateSettingMutation = useMutation({
    mutationFn: (variables: { key: string; value: string; category: string }) =>
      updateSetting(variables.key, variables.value, variables.category),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      toast.success(t('account.certEmailUpdated'))
    },
    onError: (error: Error) => {
      toast.error(t('account.certEmailUpdateFailed', { error: error.message }))
    },
  })

  const regenerateMutation = useMutation({
    mutationFn: regenerateApiKey,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      toast.success(t('account.apiKeyRegenerated'))
    },
    onError: (error: Error) => {
      toast.error(t('account.apiKeyRegenerateFailed', { error: error.message }))
    },
  })

  const handleUpdateProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!emailValid) return

    // Check if email changed
    if (email !== profile?.email) {
        setPreviousEmail(profile?.email || '')
        setPendingProfileUpdate({ name, email })
        setShowPasswordPrompt(true)
        return
    }

    updateProfileMutation.mutate({ name, email })
  }

  const handlePasswordPromptSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!pendingProfileUpdate) return

    setShowPasswordPrompt(false)

    // If email changed, we might need to ask about cert email too
    // But first, let's update the profile with the password
    updateProfileMutation.mutate({
        name: pendingProfileUpdate.name,
        email: pendingProfileUpdate.email,
        current_password: confirmPasswordForUpdate
    }, {
        onSuccess: () => {
            setConfirmPasswordForUpdate('')
            // Check if we need to prompt for cert email
            // We do this AFTER success to ensure profile is updated
            // But wait, if we do it after success, the profile email is already new.
            // The user wanted to be asked.
            // Let's ask about cert email first? No, user said "Updateing email test the popup worked as expected"
            // But "I chose to keep my certificate email as the old email and it changed anyway"
            // This implies the logic below is flawed or the backend/frontend sync is weird.

            // Let's show the cert email modal if the update was successful AND it was an email change
            setShowEmailConfirmModal(true)
        },
        onError: () => {
             setConfirmPasswordForUpdate('')
        }
    })
  }

  const confirmEmailUpdate = (updateCertEmail: boolean) => {
    setShowEmailConfirmModal(false)

    if (updateCertEmail) {
        updateSettingMutation.mutate({
            key: 'caddy.email',
            value: email,
            category: 'caddy'
        })
        setCertEmail(email)
        setUseUserEmail(true)
    } else {
        // If user chose NO, we must ensure the cert email stays as the OLD email.
        // If settings['caddy.email'] is empty, it defaults to profile email (which is now NEW).
        // So we must explicitly save the OLD email.
        const savedEmail = settings?.['caddy.email']
        if (!savedEmail && previousEmail) {
             updateSettingMutation.mutate({
                key: 'caddy.email',
                value: previousEmail,
                category: 'caddy'
            })
            // Update local state immediately
            setCertEmail(previousEmail)
            setUseUserEmail(false)
        }
    }
  }

  const handleUpdateCertEmail = (e: React.FormEvent) => {
    e.preventDefault()
    if (!useUserEmail && !certEmailValid) return

    const emailToSave = useUserEmail ? profile?.email : certEmail
    if (!emailToSave) return

    updateSettingMutation.mutate({
      key: 'caddy.email',
      value: emailToSave,
      category: 'caddy'
    })
  }

  // Compute disabled state for certificate email button
  // Button should be disabled when using custom email and it's invalid/empty  const isCertEmailButtonDisabled = useUserEmail ? false : (certEmailValid !== true)

  const handlePasswordChange = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword !== confirmPassword) {
      toast.error(t('account.passwordsDoNotMatch'))
      return
    }

    setLoading(true)
    try {
      await changePassword(oldPassword, newPassword)
      toast.success(t('account.passwordUpdated'))
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err) {
      const error = err as Error
      toast.error(error.message || t('account.passwordUpdateFailed'))
    } finally {
      setLoading(false)
    }
  }

  const copyApiKey = () => {
    if (profile?.api_key) {
      navigator.clipboard.writeText(profile.api_key)
      toast.success(t('account.apiKeyCopied'))
    }
  }

  if (isLoadingProfile) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        {[1, 2, 3, 4].map((i) => (
          <Card key={i}>
            <CardContent className="p-6 space-y-4">
              <Skeleton className="h-6 w-32" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </CardContent>
          </Card>
        ))}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <div className="p-2 bg-brand-500/10 rounded-lg">
          <User className="h-6 w-6 text-brand-500" />
        </div>
        <h1 className="text-2xl font-bold text-content-primary">{t('account.title')}</h1>
      </div>

      {/* Profile Settings */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <User className="h-5 w-5 text-brand-500" />
            <CardTitle>{t('account.profile')}</CardTitle>
          </div>
          <CardDescription>{t('account.profileDescription')}</CardDescription>
        </CardHeader>
        <form onSubmit={handleUpdateProfile}>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="profile-name" required>{t('common.name')}</Label>
              <Input
                id="profile-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="profile-email" required>{t('auth.email')}</Label>
              <Input
                id="profile-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                error={emailValid === false ? t('errors.invalidEmail') : undefined}
              />
            </div>
          </CardContent>
          <CardFooter className="justify-end">
            <Button type="submit" isLoading={updateProfileMutation.isPending} disabled={emailValid === false}>
              {t('account.saveProfile')}
            </Button>
          </CardFooter>
        </form>
      </Card>

      {/* Certificate Email Settings */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Mail className="h-5 w-5 text-info" />
            <CardTitle>{t('account.certificateEmail')}</CardTitle>
          </div>
          <CardDescription>
            {t('account.certificateEmailDescription')}
          </CardDescription>
        </CardHeader>
        <form onSubmit={handleUpdateCertEmail}>
          <CardContent className="space-y-4">
            <div className="flex items-center gap-3">
              <Checkbox
                id="useUserEmail"
                checked={useUserEmail}
                onCheckedChange={(checked) => {
                  setUseUserEmail(checked === true)
                  if (checked && profile) {
                    setCertEmail(profile.email)
                  }
                }}
              />
              <Label htmlFor="useUserEmail" className="cursor-pointer">
                {t('account.useAccountEmail', { email: profile?.email })}
              </Label>
            </div>

            {!useUserEmail && (
              <div className="space-y-2">
                <Label htmlFor="cert-email" required>{t('account.customEmail')}</Label>
                <Input
                  id="cert-email"
                  type="email"
                  value={certEmail}
                  onChange={(e) => setCertEmail(e.target.value)}
                  required={!useUserEmail}
                  error={certEmailValid === false ? t('errors.invalidEmail') : undefined}
                  errorTestId="cert-email-error"
                  aria-invalid={certEmailValid === false}
                />
              </div>
            )}
          </CardContent>
          <CardFooter className="justify-end">
            <Button
              type="submit"
              isLoading={updateSettingMutation.isPending}
              disabled={useUserEmail ? false : certEmailValid !== true}
              data-use-user-email={useUserEmail}
              data-cert-email-valid={String(certEmailValid)}
              data-compute-disabled={String(useUserEmail ? false : certEmailValid !== true)}
            >
              {t('account.saveCertificateEmail')}
            </Button>
          </CardFooter>
        </form>
      </Card>

      {/* Password Change */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Shield className="h-5 w-5 text-success" />
            <CardTitle>{t('account.changePassword')}</CardTitle>
          </div>
          <CardDescription>{t('account.changePasswordDescription')}</CardDescription>
        </CardHeader>
        <form onSubmit={handlePasswordChange}>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="current-password" required>{t('account.currentPassword')}</Label>
              <Input
                id="current-password"
                type="password"
                value={oldPassword}
                onChange={(e) => setOldPassword(e.target.value)}
                required
                autoComplete="current-password"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-password" required>{t('account.newPassword')}</Label>
              <Input
                id="new-password"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
                autoComplete="new-password"
              />
              <PasswordStrengthMeter password={newPassword} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirm-password" required>{t('account.confirmNewPassword')}</Label>
              <Input
                id="confirm-password"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
                error={confirmPassword && newPassword !== confirmPassword ? t('account.passwordsDoNotMatch') : undefined}
                autoComplete="new-password"
              />
            </div>
          </CardContent>
          <CardFooter className="justify-end">
            <Button type="submit" isLoading={loading}>
              {t('account.updatePassword')}
            </Button>
          </CardFooter>
        </form>
      </Card>

      {/* API Key */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Key className="h-5 w-5 text-warning" />
            <CardTitle>{t('account.apiKey')}</CardTitle>
          </div>
          <CardDescription>
            {t('account.apiKeyDescription')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex gap-2">
            <Input
              value={profile?.api_key || ''}
              readOnly
              className="font-mono text-sm"
            />
            <Button type="button" variant="secondary" onClick={copyApiKey} title={t('account.copyToClipboard')}>
              <Copy className="h-4 w-4" />
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => regenerateMutation.mutate()}
              isLoading={regenerateMutation.isPending}
              title={t('account.regenerateApiKey')}
            >
              <RefreshCw className="h-4 w-4" />
            </Button>
          </div>
        </CardContent>
      </Card>

      <Alert variant="warning" title={t('account.securityNotice')}>
        {t('account.securityNoticeMessage')}
      </Alert>

      {/* Password Prompt Modal */}
      {showPasswordPrompt && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <Card className="max-w-md w-full">
            <CardHeader>
              <div className="flex items-center gap-3 text-brand-500">
                <Shield className="h-6 w-6" />
                <CardTitle>{t('account.confirmPassword')}</CardTitle>
              </div>
              <CardDescription>
                {t('account.confirmPasswordDescription')}
              </CardDescription>
            </CardHeader>
            <form onSubmit={handlePasswordPromptSubmit}>
              <CardContent>
                <div className="space-y-2">
                  <Label htmlFor="confirm-current-password" required>{t('account.currentPassword')}</Label>
                  <Input
                    id="confirm-current-password"
                    type="password"
                    placeholder={t('account.enterPassword')}
                    value={confirmPasswordForUpdate}
                    onChange={(e) => setConfirmPasswordForUpdate(e.target.value)}
                    required
                    autoFocus
                  />
                </div>
              </CardContent>
              <CardFooter className="flex-col gap-3">
                <Button type="submit" className="w-full" isLoading={updateProfileMutation.isPending}>
                  {t('account.confirmAndUpdate')}
                </Button>
                <Button
                  type="button"
                  onClick={() => {
                    setShowPasswordPrompt(false)
                    setConfirmPasswordForUpdate('')
                    setPendingProfileUpdate(null)
                  }}
                  variant="ghost"
                  className="w-full"
                >
                  {t('common.cancel')}
                </Button>
              </CardFooter>
            </form>
          </Card>
        </div>
      )}

      {/* Email Update Confirmation Modal */}
      {showEmailConfirmModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <Card className="max-w-md w-full">
            <CardHeader>
              <div className="flex items-center gap-3 text-warning">
                <AlertTriangle className="h-6 w-6" />
                <CardTitle>{t('account.updateCertEmailTitle')}</CardTitle>
              </div>
              <CardDescription>
                {t('account.updateCertEmailDescription', { email })}
              </CardDescription>
            </CardHeader>
            <CardFooter className="flex-col gap-3">
              <Button onClick={() => confirmEmailUpdate(true)} className="w-full">
                {t('account.yesUpdateCertEmail')}
              </Button>
              <Button onClick={() => confirmEmailUpdate(false)} variant="secondary" className="w-full">
                {t('account.noKeepEmail', { email: previousEmail || certEmail })}
              </Button>
              <Button onClick={() => setShowEmailConfirmModal(false)} variant="ghost" className="w-full">
                {t('common.cancel')}
              </Button>
            </CardFooter>
          </Card>
        </div>
      )}
    </div>
  )
}
