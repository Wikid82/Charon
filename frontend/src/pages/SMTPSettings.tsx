import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { toast } from '../utils/toast'
import { getSMTPConfig, updateSMTPConfig, testSMTPConnection, sendTestEmail } from '../api/smtp'
import type { SMTPConfigRequest } from '../api/smtp'
import { Mail, Send, CheckCircle2, XCircle, Loader2 } from 'lucide-react'

export default function SMTPSettings() {
  const queryClient = useQueryClient()
  const [host, setHost] = useState('')
  const [port, setPort] = useState(587)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [fromAddress, setFromAddress] = useState('')
  const [encryption, setEncryption] = useState<'none' | 'ssl' | 'starttls'>('starttls')
  const [testEmail, setTestEmail] = useState('')

  const { data: smtpConfig, isLoading } = useQuery({
    queryKey: ['smtp-config'],
    queryFn: getSMTPConfig,
  })

  useEffect(() => {
    if (smtpConfig) {
      setHost(smtpConfig.host || '')
      setPort(smtpConfig.port || 587)
      setUsername(smtpConfig.username || '')
      setPassword(smtpConfig.password || '')
      setFromAddress(smtpConfig.from_address || '')
      setEncryption(smtpConfig.encryption || 'starttls')
    }
  }, [smtpConfig])

  const saveMutation = useMutation({
    mutationFn: async () => {
      const config: SMTPConfigRequest = {
        host,
        port,
        username,
        password,
        from_address: fromAddress,
        encryption,
      }
      return updateSMTPConfig(config)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['smtp-config'] })
      toast.success('SMTP settings saved successfully')
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || 'Failed to save SMTP settings')
    },
  })

  const testConnectionMutation = useMutation({
    mutationFn: testSMTPConnection,
    onSuccess: (data) => {
      if (data.success) {
        toast.success(data.message || 'SMTP connection successful')
      } else {
        toast.error(data.error || 'SMTP connection failed')
      }
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || 'Failed to test SMTP connection')
    },
  })

  const sendTestEmailMutation = useMutation({
    mutationFn: async () => sendTestEmail({ to: testEmail }),
    onSuccess: (data) => {
      if (data.success) {
        toast.success(data.message || 'Test email sent successfully')
        setTestEmail('')
      } else {
        toast.error(data.error || 'Failed to send test email')
      }
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || 'Failed to send test email')
    },
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-8 w-8 animate-spin text-blue-500" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Mail className="h-6 w-6 text-blue-500" />
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Email (SMTP) Settings</h2>
      </div>

      <p className="text-sm text-gray-500 dark:text-gray-400">
        Configure SMTP settings to enable email notifications and user invitations.
      </p>

      <Card className="p-6">
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="SMTP Host"
              type="text"
              value={host}
              onChange={(e) => setHost(e.target.value)}
              placeholder="smtp.gmail.com"
            />
            <Input
              label="Port"
              type="number"
              value={port}
              onChange={(e) => setPort(parseInt(e.target.value) || 587)}
              placeholder="587"
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="Username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="your@email.com"
            />
            <Input
              label="Password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              helperText="Use app-specific password for Gmail"
            />
          </div>

          <Input
            label="From Address"
            type="email"
            value={fromAddress}
            onChange={(e) => setFromAddress(e.target.value)}
            placeholder="Charon <no-reply@example.com>"
          />

          <div className="w-full">
            <label className="block text-sm font-medium text-gray-300 mb-1.5">
              Encryption
            </label>
            <select
              value={encryption}
              onChange={(e) => setEncryption(e.target.value as 'none' | 'ssl' | 'starttls')}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
            >
              <option value="starttls">STARTTLS (Recommended)</option>
              <option value="ssl">SSL/TLS</option>
              <option value="none">None</option>
            </select>
          </div>

          <div className="flex justify-end gap-3 pt-4 border-t border-gray-700">
            <Button
              variant="secondary"
              onClick={() => testConnectionMutation.mutate()}
              isLoading={testConnectionMutation.isPending}
              disabled={!host || !fromAddress}
            >
              Test Connection
            </Button>
            <Button
              onClick={() => saveMutation.mutate()}
              isLoading={saveMutation.isPending}
            >
              Save Settings
            </Button>
          </div>
        </div>
      </Card>

      {/* Status Indicator */}
      <Card className="p-4">
        <div className="flex items-center gap-3">
          {smtpConfig?.configured ? (
            <>
              <CheckCircle2 className="h-5 w-5 text-green-500" />
              <span className="text-green-500 font-medium">SMTP Configured</span>
            </>
          ) : (
            <>
              <XCircle className="h-5 w-5 text-yellow-500" />
              <span className="text-yellow-500 font-medium">SMTP Not Configured</span>
            </>
          )}
        </div>
      </Card>

      {/* Test Email */}
      {smtpConfig?.configured && (
        <Card className="p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
            Send Test Email
          </h3>
          <div className="flex gap-3">
            <div className="flex-1">
              <Input
                type="email"
                value={testEmail}
                onChange={(e) => setTestEmail(e.target.value)}
                placeholder="recipient@example.com"
              />
            </div>
            <Button
              onClick={() => sendTestEmailMutation.mutate()}
              isLoading={sendTestEmailMutation.isPending}
              disabled={!testEmail}
            >
              <Send className="h-4 w-4 mr-2" />
              Send Test
            </Button>
          </div>
        </Card>
      )}
    </div>
  )
}
