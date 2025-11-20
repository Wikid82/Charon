import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { Button } from '../components/ui/Button'
import { toast } from '../components/Toast'
import client from '../api/client'
import { getProfile, regenerateApiKey } from '../api/user'
import { Copy, RefreshCw, Shield } from 'lucide-react'

export default function Security() {
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const queryClient = useQueryClient()

  const { data: profile, isLoading: isLoadingProfile } = useQuery({
    queryKey: ['profile'],
    queryFn: getProfile,
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
            <Input
              label="New Password"
              type="password"
              value={newPassword}
              onChange={e => setNewPassword(e.target.value)}
              required
            />
            <Input
              label="Confirm New Password"
              type="password"
              value={confirmPassword}
              onChange={e => setConfirmPassword(e.target.value)}
              required
            />
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
