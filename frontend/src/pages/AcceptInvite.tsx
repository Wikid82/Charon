import { useState, useEffect } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { PasswordStrengthMeter } from '../components/PasswordStrengthMeter'
import { toast } from '../utils/toast'
import { validateInvite, acceptInvite } from '../api/users'
import { Loader2, CheckCircle2, XCircle, UserCheck } from 'lucide-react'

export default function AcceptInvite() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const token = searchParams.get('token') || ''

  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [accepted, setAccepted] = useState(false)

  const {
    data: validation,
    isLoading: isValidating,
    error: validationError,
  } = useQuery({
    queryKey: ['validate-invite', token],
    queryFn: () => validateInvite(token),
    enabled: !!token,
    retry: false,
  })

  const acceptMutation = useMutation({
    mutationFn: async () => {
      return acceptInvite({ token, name, password })
    },
    onSuccess: (data) => {
      setAccepted(true)
      toast.success(`Welcome, ${data.email}! You can now log in.`)
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || 'Failed to accept invitation')
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (password !== confirmPassword) {
      toast.error('Passwords do not match')
      return
    }
    if (password.length < 8) {
      toast.error('Password must be at least 8 characters')
      return
    }
    acceptMutation.mutate()
  }

  // Redirect to login after successful acceptance
  useEffect(() => {
    if (accepted) {
      const timer = setTimeout(() => {
        navigate('/login')
      }, 3000)
      return () => clearTimeout(timer)
    }
  }, [accepted, navigate])

  if (!token) {
    return (
      <div className="min-h-screen bg-dark-bg flex items-center justify-center p-4">
        <Card className="w-full max-w-md">
          <div className="flex flex-col items-center py-8">
            <XCircle className="h-16 w-16 text-red-500 mb-4" />
            <h2 className="text-xl font-semibold text-white mb-2">Invalid Link</h2>
            <p className="text-gray-400 text-center mb-6">
              This invitation link is invalid or incomplete.
            </p>
            <Button onClick={() => navigate('/login')}>Go to Login</Button>
          </div>
        </Card>
      </div>
    )
  }

  if (isValidating) {
    return (
      <div className="min-h-screen bg-dark-bg flex items-center justify-center p-4">
        <Card className="w-full max-w-md">
          <div className="flex flex-col items-center py-8">
            <Loader2 className="h-12 w-12 animate-spin text-blue-500 mb-4" />
            <p className="text-gray-400">Validating invitation...</p>
          </div>
        </Card>
      </div>
    )
  }

  if (validationError || !validation?.valid) {
    const errorData = validationError as { response?: { data?: { error?: string } } } | undefined
    const errorMessage = errorData?.response?.data?.error || 'This invitation has expired or is invalid.'

    return (
      <div className="min-h-screen bg-dark-bg flex items-center justify-center p-4">
        <Card className="w-full max-w-md">
          <div className="flex flex-col items-center py-8">
            <XCircle className="h-16 w-16 text-red-500 mb-4" />
            <h2 className="text-xl font-semibold text-white mb-2">Invitation Invalid</h2>
            <p className="text-gray-400 text-center mb-6">{errorMessage}</p>
            <Button onClick={() => navigate('/login')}>Go to Login</Button>
          </div>
        </Card>
      </div>
    )
  }

  if (accepted) {
    return (
      <div className="min-h-screen bg-dark-bg flex items-center justify-center p-4">
        <Card className="w-full max-w-md">
          <div className="flex flex-col items-center py-8">
            <CheckCircle2 className="h-16 w-16 text-green-500 mb-4" />
            <h2 className="text-xl font-semibold text-white mb-2">Account Created!</h2>
            <p className="text-gray-400 text-center mb-6">
              Your account has been set up successfully. Redirecting to login...
            </p>
            <Loader2 className="h-6 w-6 animate-spin text-blue-500" />
          </div>
        </Card>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-dark-bg flex items-center justify-center p-4">
      <div className="w-full max-w-md space-y-4">
        <div className="flex items-center justify-center">
          <img src="/logo.png" alt="Charon" style={{ height: '100px', width: 'auto' }} />
        </div>

        <Card title="Accept Invitation">
          <div className="space-y-4">
            <div className="bg-blue-900/20 border border-blue-800 rounded-lg p-4 mb-4">
              <div className="flex items-center gap-2 text-blue-400 mb-1">
                <UserCheck className="h-4 w-4" />
                <span className="font-medium">You&apos;ve been invited!</span>
              </div>
              <p className="text-sm text-gray-300">
                Complete your account setup for <strong>{validation.email}</strong>
              </p>
            </div>

            <form onSubmit={handleSubmit} className="space-y-4">
              <Input
                label="Your Name"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="John Doe"
                required
              />

              <div className="space-y-2">
                <Input
                  label="Password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••"
                  required
                />
                <PasswordStrengthMeter password={password} />
              </div>

              <Input
                label="Confirm Password"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="••••••••"
                required
                error={
                  confirmPassword && password !== confirmPassword
                    ? 'Passwords do not match'
                    : undefined
                }
              />

              <Button
                type="submit"
                className="w-full"
                isLoading={acceptMutation.isPending}
                disabled={!name || !password || password !== confirmPassword}
              >
                Create Account
              </Button>
            </form>
          </div>
        </Card>
      </div>
    </div>
  )
}
