import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { Button } from '../components/ui/Button'
import { toast } from '../utils/toast'
import client from '../api/client'
import { useAuth } from '../hooks/useAuth'
import { getSetupStatus } from '../api/setup'
import { ConfigReloadOverlay } from '../components/LoadingStates'

export default function Login() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [showResetInfo, setShowResetInfo] = useState(false)
  const { login } = useAuth()

  const { data: setupStatus, isLoading: isCheckingSetup } = useQuery({
    queryKey: ['setupStatus'],
    queryFn: getSetupStatus,
    retry: false,
  })

  useEffect(() => {
    if (setupStatus?.setupRequired) {
      navigate('/setup')
    }
  }, [setupStatus, navigate])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)

    try {
      const res = await client.post('/auth/login', { email, password })
      const token = (res.data as { token?: string }).token
      await login(token)
      await queryClient.invalidateQueries({ queryKey: ['setupStatus'] })
      toast.success(t('auth.loginSuccess'))
      navigate('/')
    } catch (err) {
      const error = err as Error & { response?: { data?: { error?: string } } }
      // The axios interceptor extracts error.response.data.error to error.message
      const message = error.response?.data?.error || error.message || t('auth.loginFailed')
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  if (isCheckingSetup) {
    return (
      <div className="min-h-screen bg-dark-bg flex items-center justify-center p-4">
        <div className="text-white">{t('auth.checkingSetup')}</div>
      </div>
    )
  }

  return (
    <>
      {loading && (
        <ConfigReloadOverlay
          message={t('auth.loggingIn')}
          submessage={t('auth.loggingInSub')}
          type="coin"
        />
      )}
      <div className="min-h-screen bg-dark-bg flex items-center justify-center p-4">
        <div className="w-full max-w-md space-y-4">
          <div className="flex items-center justify-center">
          <img src="/logo.png" alt="Charon" style={{ height: '150px', width: 'auto' }}/>


          </div>
          <Card className="w-full" title={t('auth.login')}>
          <form onSubmit={handleSubmit} className="space-y-6">
            <Input
              label={t('auth.email')}
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              required
              placeholder="admin@example.com"
              disabled={loading}
              autoComplete="email"
            />
            <div className="space-y-1">
              <Input
                label={t('auth.password')}
                type="password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                required
                placeholder="••••••••"
                disabled={loading}
                autoComplete="current-password"
              />
              <div className="flex justify-end">
                <button
                  type="button"
                  onClick={() => setShowResetInfo(!showResetInfo)}
                  className="text-sm text-blue-400 hover:text-blue-300"
                  disabled={loading}
                >
                  {t('auth.forgotPassword')}
                </button>
              </div>
            </div>

            {showResetInfo && (
              <div className="bg-blue-900/20 border border-blue-800 rounded-lg p-4 text-sm text-blue-200">
                <p className="mb-2 font-medium">{t('auth.resetPasswordTitle')}</p>
                <p className="mb-2">{t('auth.resetPasswordInstructions')}</p>
                <code className="block bg-black/50 p-2 rounded font-mono text-xs break-all select-all">
                  docker exec -it caddy-proxy-manager /app/backend reset-password &lt;email&gt; &lt;new-password&gt;
                </code>
              </div>
            )}

            <Button type="submit" className="w-full" isLoading={loading}>
              {t('auth.signIn')}
            </Button>
          </form>
        </Card>
        </div>
      </div>
    </>
  )
}
