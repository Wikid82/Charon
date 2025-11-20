import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { Button } from '../components/ui/Button'
import { toast } from '../components/Toast'
import client from '../api/client'
import { useAuth } from '../context/AuthContext'

export default function Login() {
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [showResetInfo, setShowResetInfo] = useState(false)
  const { login } = useAuth()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)

    try {
      await client.post('/auth/login', { email, password })
      login()
      toast.success('Logged in successfully')
      navigate('/')
    } catch (err: any) {
      toast.error(err.response?.data?.error || 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-dark-bg flex items-center justify-center p-4">
      <Card className="w-full max-w-md" title="Login">
        <form onSubmit={handleSubmit} className="space-y-6">
          <Input
            label="Email"
            type="email"
            value={email}
            onChange={e => setEmail(e.target.value)}
            required
            placeholder="admin@example.com"
          />
          <div className="space-y-1">
            <Input
              label="Password"
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              required
              placeholder="••••••••"
            />
            <div className="flex justify-end">
              <button
                type="button"
                onClick={() => setShowResetInfo(!showResetInfo)}
                className="text-sm text-blue-400 hover:text-blue-300"
              >
                Forgot Password?
              </button>
            </div>
          </div>

          {showResetInfo && (
            <div className="bg-blue-900/20 border border-blue-800 rounded-lg p-4 text-sm text-blue-200">
              <p className="mb-2 font-medium">To reset your password:</p>
              <p className="mb-2">Run this command on your server:</p>
              <code className="block bg-black/50 p-2 rounded font-mono text-xs break-all select-all">
                docker exec -it caddy-proxy-manager /app/backend reset-password &lt;email&gt; &lt;new-password&gt;
              </code>
            </div>
          )}

          <Button type="submit" className="w-full" isLoading={loading}>
            Sign In
          </Button>
        </form>
      </Card>
    </div>
  )
}
