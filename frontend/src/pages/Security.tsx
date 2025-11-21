import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { Button } from '../components/ui/Button'
import { toast } from '../utils/toast'
import client from '../api/client'
import { getProfile, regenerateApiKey, updateProfile } from '../api/user'
import { getSettings, updateSetting } from '../api/settings'
import { Copy, RefreshCw, Shield, Mail, User } from 'lucide-react'
import { PasswordStrengthMeter } from '../components/PasswordStrengthMeter'
import { isValidEmail } from '../utils/validation'

export default function Security() {
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)

  // Profile State
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [emailValid, setEmailValid] = useState<boolean | null>(null)

  // Certificate Email State
  const [certEmail, setCertEmail] = useState('')
  const [certEmailValid, setCertEmailValid] = useState<boolean | null>(null)
  const [useUserEmail, setUseUserEmail] = useState(true)

  const queryClient = useQueryClient()

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

  const saveCertEmailMutation = useMutation({
    mutationFn: async () => {
      const emailToSave = useUserEmail ? profile?.email : certEmail
      if (!emailToSave) return
      await updateSetting('caddy.email', emailToSave, 'caddy', 'string')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      toast.success('Certificate email updated')
    },
    onError: (error: any) => {
      toast.error(`Failed to update certificate email: ${error.message}`)
    },
  })

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword !== confirmPassword) {
      toast.error('New passwords do not match')
      return
    }

    setLoading(true)
    try {
      await client.post('/auth/change-password', {
        old_password: oldPassword,
        new_password: newPassword,
      })
      toast.success('Password updated successfully')
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Failed to update password')
    } finally {
      setLoading(false)
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success('Copied to clipboard')
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
        <Shield className="w-8 h-8" />
        Security
      </h1>

      <div className="grid gap-6">
        {/* Profile Settings */}
        <Card className="max-w-2xl p-6">
          <div className="flex items-center gap-2 mb-4">
            <User className="w-5 h-5 text-gray-500" />
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Profile Settings</h2>
          </div>
          <div className="space-y-4">
            <Input
              label="Name"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <div className="relative">
              <Input
                label="Login Email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className={emailValid === false ? 'border-red-500 focus:ring-red-500' : emailValid === true ? 'border-green-500 focus:ring-green-500' : ''}
              />
              {emailValid === false && (
                <p className="mt-1 text-xs text-red-500">Please enter a valid email address</p>
              )}
            </div>
            <Button
              onClick={() => updateProfileMutation.mutate({ name, email })}
              isLoading={updateProfileMutation.isPending}
            >
              Update Profile
            </Button>
          </div>
        </Card>

        {/* Certificate Email Configuration */}
        <Card className="max-w-2xl p-6">
          <div className="flex items-center gap-2 mb-4">
            <Mail className="w-5 h-5 text-gray-500" />
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Certificate Email</h2>
          </div>
          <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
            This email address is used to register with Let's Encrypt for SSL certificate generation and expiration notifications.
          </p>

          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="useUserEmail"
                checked={useUserEmail}
                onChange={(e) => setUseUserEmail(e.target.checked)}
                className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              <label htmlFor="useUserEmail" className="text-sm text-gray-700 dark:text-gray-300">
                Use my login email ({profile?.email})
              </label>
            </div>

            {!useUserEmail && (
              <div className="relative">
                <Input
                  label="Custom Email Address"
                  type="email"
                  value={certEmail}
                  onChange={(e) => setCertEmail(e.target.value)}
                  placeholder="certs@example.com"
                  className={certEmailValid === false ? 'border-red-500 focus:ring-red-500' : certEmailValid === true ? 'border-green-500 focus:ring-green-500' : ''}
                />
                {certEmailValid === false && (
                  <p className="mt-1 text-xs text-red-500">Please enter a valid email address</p>
                )}
              </div>
            )}

            <Button
              onClick={() => saveCertEmailMutation.mutate()}
              isLoading={saveCertEmailMutation.isPending}
            >
              Save Email Settings
            </Button>
          </div>
        </Card>

        {/* Change Password */}
        <Card className="max-w-2xl p-6">
          <h2 className="text-xl font-semibold mb-4 text-gray-900 dark:text-white">Change Password</h2>
          <form onSubmit={handleChangePassword} className="space-y-4">
            <Input
              label="Current Password"
              type="password"
              value={oldPassword}
              onChange={e => setOldPassword(e.target.value)}
              required
            />
            <div className="space-y-1">
              <Input
                label="New Password"
                type="password"
                value={newPassword}
                onChange={e => setNewPassword(e.target.value)}
                required
              />
              <PasswordStrengthMeter password={newPassword} />
            </div>
            <div className="space-y-1">
              <Input
                label="Confirm New Password"
                type="password"
                value={confirmPassword}
                onChange={e => setConfirmPassword(e.target.value)}
                required
                className={confirmPassword && newPassword !== confirmPassword ? 'border-red-500 focus:ring-red-500' : ''}
              />
              {confirmPassword && newPassword !== confirmPassword && (
                <p className="text-xs text-red-500">Passwords do not match</p>
              )}
            </div>
            <Button type="submit" isLoading={loading}>
              Update Password
            </Button>
          </form>
        </Card>

        {/* API Key */}
        <Card className="max-w-2xl p-6">
          <h2 className="text-xl font-semibold mb-4 text-gray-900 dark:text-white">API Key</h2>
          <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
            Use this key to authenticate with the API externally. Keep it secret!
          </p>

          {isLoadingProfile ? (
            <div className="animate-pulse h-10 bg-gray-200 dark:bg-gray-700 rounded" />
          ) : (
            <div className="space-y-4">
              <div className="flex gap-2">
                <Input
                  value={profile?.api_key || 'No API Key generated'}
                  readOnly
                  className="font-mono text-sm"
                />
                <Button
                  variant="secondary"
                  onClick={() => copyToClipboard(profile?.api_key || '')}
                  disabled={!profile?.api_key}
                >
                  <Copy className="w-4 h-4" />
                </Button>
              </div>
              <Button
                variant="danger"
                onClick={() => {
                  if (confirm('Are you sure? This will invalidate the old key.')) {
                    regenerateMutation.mutate()
                  }
                }}
                isLoading={regenerateMutation.isPending}
              >
                <RefreshCw className="w-4 h-4 mr-2" />
                {profile?.api_key ? 'Regenerate Key' : 'Generate Key'}
              </Button>
            </div>
          )}
        </Card>
      </div>
    </div>
  )
}
