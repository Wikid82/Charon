import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { Button } from '../components/ui/Button'
import { toast } from '../utils/toast'
import { getProfile, regenerateApiKey, updateProfile } from '../api/user'
import { getSettings, updateSetting } from '../api/settings'
import { Copy, RefreshCw, Shield, Mail, User, AlertTriangle } from 'lucide-react'
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
    onError: (error: any) => {
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
    onError: (error: any) => {
      toast.error(`Failed to update certificate email: ${error.message}`)
    },
  })

  const regenerateMutation = useMutation({
    mutationFn: regenerateApiKey,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      toast.success('API Key regenerated successfully')
    },
    onError: (error: any) => {
      toast.error(`Failed to regenerate API key: ${error.message}`)
    },
  })

  const handleUpdateProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!emailValid) return

    // Check if email changed and is different from current cert email
    if (email !== profile?.email) {
        setShowEmailConfirmModal(true)
        return
    }

    updateProfileMutation.mutate({ name, email })
  }

  const confirmEmailUpdate = (updateCertEmail: boolean) => {
    setShowEmailConfirmModal(false)

    // Update profile
    updateProfileMutation.mutate({ name, email }, {
        onSuccess: () => {
            if (updateCertEmail) {
                updateSettingMutation.mutate({
                    key: 'caddy.email',
                    value: email,
                    category: 'caddy'
                })
                setCertEmail(email)
                setUseUserEmail(true)
            }
        }
    })
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
    } catch (error: any) {
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
    return <div className="p-4">Loading profile...</div>
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Account Settings</h1>

      {/* Profile Settings */}
      <Card className="p-6">
        <div className="flex items-center gap-2 mb-4">
          <User className="w-5 h-5 text-blue-500" />
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Profile</h2>
        </div>
        <form onSubmit={handleUpdateProfile} className="space-y-4">
          <Input
            label="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
          <Input
            label="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            error={emailValid === false ? 'Please enter a valid email address' : undefined}
            className={emailValid === true ? 'border-green-500 focus:ring-green-500' : ''}
          />
          <div className="flex justify-end">
            <Button type="submit" isLoading={updateProfileMutation.isPending} disabled={emailValid === false}>
              Save Profile
            </Button>
          </div>
        </form>
      </Card>

      {/* Certificate Email Settings */}
      <Card className="p-6">
        <div className="flex items-center gap-2 mb-4">
          <Mail className="w-5 h-5 text-purple-500" />
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Certificate Email</h2>
        </div>
        <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
          This email is used for Let's Encrypt notifications and recovery.
        </p>
        <form onSubmit={handleUpdateCertEmail} className="space-y-4">
          <div className="flex items-center gap-2 mb-2">
            <input
              type="checkbox"
              id="useUserEmail"
              checked={useUserEmail}
              onChange={(e) => {
                setUseUserEmail(e.target.checked)
                if (e.target.checked && profile) {
                  setCertEmail(profile.email)
                }
              }}
              className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
            />
            <label htmlFor="useUserEmail" className="text-sm text-gray-700 dark:text-gray-300">
              Use my account email ({profile?.email})
            </label>
          </div>

          {!useUserEmail && (
            <Input
              label="Custom Email"
              type="email"
              value={certEmail}
              onChange={(e) => setCertEmail(e.target.value)}
              required={!useUserEmail}
              error={certEmailValid === false ? 'Please enter a valid email address' : undefined}
              className={certEmailValid === true ? 'border-green-500 focus:ring-green-500' : ''}
            />
          )}

          <div className="flex justify-end">
            <Button type="submit" isLoading={updateSettingMutation.isPending} disabled={!useUserEmail && certEmailValid === false}>
              Save Certificate Email
            </Button>
          </div>
        </form>
      </Card>

      {/* Password Change */}
      <Card className="p-6">
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5 text-green-500" />
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Change Password</h2>
        </div>
        <form onSubmit={handlePasswordChange} className="space-y-4">
          <Input
            label="Current Password"
            type="password"
            value={oldPassword}
            onChange={(e) => setOldPassword(e.target.value)}
            required
          />
          <div>
            <Input
              label="New Password"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
            />
            <PasswordStrengthMeter password={newPassword} />
          </div>
          <Input
            label="Confirm New Password"
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
            error={confirmPassword && newPassword !== confirmPassword ? 'Passwords do not match' : undefined}
          />
          <div className="flex justify-end">
            <Button type="submit" isLoading={loading}>
              Update Password
            </Button>
          </div>
        </form>
      </Card>

      {/* API Key */}
      <Card className="p-6">
        <div className="flex items-center gap-2 mb-4">
          <div className="p-1 bg-yellow-100 dark:bg-yellow-900/30 rounded">
            <span className="text-lg">🔑</span>
          </div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">API Key</h2>
        </div>
        <div className="space-y-4">
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Use this key to authenticate with the API programmatically. Keep it secret!
          </p>
          <div className="flex gap-2">
            <Input
              value={profile?.api_key || ''}
              readOnly
              className="font-mono text-sm bg-gray-50 dark:bg-gray-900"
            />
            <Button type="button" variant="secondary" onClick={copyApiKey} title="Copy to clipboard">
              <Copy className="w-4 h-4" />
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => regenerateMutation.mutate()}
              isLoading={regenerateMutation.isPending}
              title="Regenerate API Key"
            >
              <RefreshCw className="w-4 h-4" />
            </Button>
          </div>
        </div>
      </Card>

      {/* Email Update Confirmation Modal */}
      {showEmailConfirmModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full p-6">
            <div className="flex items-center gap-3 mb-4 text-yellow-600 dark:text-yellow-500">
              <AlertTriangle className="w-6 h-6" />
              <h3 className="text-lg font-bold">Update Certificate Email?</h3>
            </div>
            <p className="text-gray-600 dark:text-gray-300 mb-6">
              You are changing your account email to <strong>{email}</strong>.
              Do you want to use this new email for SSL certificates as well?
            </p>
            <div className="flex flex-col gap-3">
              <Button onClick={() => confirmEmailUpdate(true)} className="w-full">
                Yes, update certificate email too
              </Button>
              <Button onClick={() => confirmEmailUpdate(false)} variant="secondary" className="w-full">
                No, keep using {certEmail}
              </Button>
              <Button onClick={() => setShowEmailConfirmModal(false)} variant="ghost" className="w-full text-gray-500">
                Cancel
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
