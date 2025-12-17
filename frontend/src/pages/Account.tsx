import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
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

  // Initialize cert email state
  useEffect(() => {
    if (settings && profile) {
      const savedEmail = settings['caddy.email']
      if (savedEmail && savedEmail !== profile.email) {
        setCertEmail(savedEmail)
        setUseUserEmail(false)
      } else {
        setCertEmail(profile.email)
        setUseUserEmail(true)
      }
    }
  }, [settings, profile])

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
      toast.success('Profile updated successfully')
    },
    onError: (error: Error) => {
      toast.error(`Failed to update profile: ${error.message}`)
    },
  })

  const updateSettingMutation = useMutation({
    mutationFn: (variables: { key: string; value: string; category: string }) =>
      updateSetting(variables.key, variables.value, variables.category),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      toast.success('Certificate email updated')
    },
    onError: (error: Error) => {
      toast.error(`Failed to update certificate email: ${error.message}`)
    },
  })

  const regenerateMutation = useMutation({
    mutationFn: regenerateApiKey,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      toast.success('API Key regenerated successfully')
    },
    onError: (error: Error) => {
      toast.error(`Failed to regenerate API key: ${error.message}`)
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

  const handlePasswordChange = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword !== confirmPassword) {
      toast.error('New passwords do not match')
      return
    }

    setLoading(true)
    try {
      await changePassword(oldPassword, newPassword)
      toast.success('Password updated successfully')
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err) {
      const error = err as Error
      toast.error(error.message || 'Failed to update password')
    } finally {
      setLoading(false)
    }
  }

  const copyApiKey = () => {
    if (profile?.api_key) {
      navigator.clipboard.writeText(profile.api_key)
      toast.success('API Key copied to clipboard')
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
        <h1 className="text-2xl font-bold text-content-primary">Account Settings</h1>
      </div>

      {/* Profile Settings */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <User className="h-5 w-5 text-brand-500" />
            <CardTitle>Profile</CardTitle>
          </div>
          <CardDescription>Update your personal information.</CardDescription>
        </CardHeader>
        <form onSubmit={handleUpdateProfile}>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="profile-name" required>Name</Label>
              <Input
                id="profile-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="profile-email" required>Email</Label>
              <Input
                id="profile-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                error={emailValid === false ? 'Please enter a valid email address' : undefined}
              />
            </div>
          </CardContent>
          <CardFooter className="justify-end">
            <Button type="submit" isLoading={updateProfileMutation.isPending} disabled={emailValid === false}>
              Save Profile
            </Button>
          </CardFooter>
        </form>
      </Card>

      {/* Certificate Email Settings */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Mail className="h-5 w-5 text-info" />
            <CardTitle>Certificate Email</CardTitle>
          </div>
          <CardDescription>
            This email is used for Let's Encrypt notifications and recovery.
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
                Use my account email ({profile?.email})
              </Label>
            </div>

            {!useUserEmail && (
              <div className="space-y-2">
                <Label htmlFor="cert-email" required>Custom Email</Label>
                <Input
                  id="cert-email"
                  type="email"
                  value={certEmail}
                  onChange={(e) => setCertEmail(e.target.value)}
                  required={!useUserEmail}
                  error={certEmailValid === false ? 'Please enter a valid email address' : undefined}
                />
              </div>
            )}
          </CardContent>
          <CardFooter className="justify-end">
            <Button type="submit" isLoading={updateSettingMutation.isPending} disabled={!useUserEmail && certEmailValid === false}>
              Save Certificate Email
            </Button>
          </CardFooter>
        </form>
      </Card>

      {/* Password Change */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Shield className="h-5 w-5 text-success" />
            <CardTitle>Change Password</CardTitle>
          </div>
          <CardDescription>Update your account password for security.</CardDescription>
        </CardHeader>
        <form onSubmit={handlePasswordChange}>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="current-password" required>Current Password</Label>
              <Input
                id="current-password"
                type="password"
                value={oldPassword}
                onChange={(e) => setOldPassword(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-password" required>New Password</Label>
              <Input
                id="new-password"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
              />
              <PasswordStrengthMeter password={newPassword} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirm-password" required>Confirm New Password</Label>
              <Input
                id="confirm-password"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
                error={confirmPassword && newPassword !== confirmPassword ? 'Passwords do not match' : undefined}
              />
            </div>
          </CardContent>
          <CardFooter className="justify-end">
            <Button type="submit" isLoading={loading}>
              Update Password
            </Button>
          </CardFooter>
        </form>
      </Card>

      {/* API Key */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Key className="h-5 w-5 text-warning" />
            <CardTitle>API Key</CardTitle>
          </div>
          <CardDescription>
            Use this key to authenticate with the API programmatically. Keep it secret!
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex gap-2">
            <Input
              value={profile?.api_key || ''}
              readOnly
              className="font-mono text-sm"
            />
            <Button type="button" variant="secondary" onClick={copyApiKey} title="Copy to clipboard">
              <Copy className="h-4 w-4" />
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => regenerateMutation.mutate()}
              isLoading={regenerateMutation.isPending}
              title="Regenerate API Key"
            >
              <RefreshCw className="h-4 w-4" />
            </Button>
          </div>
        </CardContent>
      </Card>

      <Alert variant="warning" title="Security Notice">
        Never share your API key or password with anyone. If you believe your credentials have been compromised, regenerate your API key immediately.
      </Alert>

      {/* Password Prompt Modal */}
      {showPasswordPrompt && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <Card className="max-w-md w-full">
            <CardHeader>
              <div className="flex items-center gap-3 text-brand-500">
                <Shield className="h-6 w-6" />
                <CardTitle>Confirm Password</CardTitle>
              </div>
              <CardDescription>
                Please enter your current password to confirm these changes.
              </CardDescription>
            </CardHeader>
            <form onSubmit={handlePasswordPromptSubmit}>
              <CardContent>
                <div className="space-y-2">
                  <Label htmlFor="confirm-current-password" required>Current Password</Label>
                  <Input
                    id="confirm-current-password"
                    type="password"
                    placeholder="Enter your password"
                    value={confirmPasswordForUpdate}
                    onChange={(e) => setConfirmPasswordForUpdate(e.target.value)}
                    required
                    autoFocus
                  />
                </div>
              </CardContent>
              <CardFooter className="flex-col gap-3">
                <Button type="submit" className="w-full" isLoading={updateProfileMutation.isPending}>
                  Confirm & Update
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
                  Cancel
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
                <CardTitle>Update Certificate Email?</CardTitle>
              </div>
              <CardDescription>
                You are changing your account email to <strong className="text-content-primary">{email}</strong>.
                Do you want to use this new email for SSL certificates as well?
              </CardDescription>
            </CardHeader>
            <CardFooter className="flex-col gap-3">
              <Button onClick={() => confirmEmailUpdate(true)} className="w-full">
                Yes, update certificate email too
              </Button>
              <Button onClick={() => confirmEmailUpdate(false)} variant="secondary" className="w-full">
                No, keep using {previousEmail || certEmail}
              </Button>
              <Button onClick={() => setShowEmailConfirmModal(false)} variant="ghost" className="w-full">
                Cancel
              </Button>
            </CardFooter>
          </Card>
        </div>
      )}
    </div>
  )
}
