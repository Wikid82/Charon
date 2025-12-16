import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Label } from '../components/ui/Label'
import { Alert } from '../components/ui/Alert'
import { Badge } from '../components/ui/Badge'
import { Skeleton } from '../components/ui/Skeleton'
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../components/ui/Select'
import { toast } from '../utils/toast'
import { getSMTPConfig, updateSMTPConfig, testSMTPConnection, sendTestEmail } from '../api/smtp'
import type { SMTPConfigRequest } from '../api/smtp'
import { Mail, Send, CheckCircle2, XCircle } from 'lucide-react'

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
      <div className="space-y-6">
        <div className="flex items-center gap-3">
          <Skeleton className="h-8 w-8" />
          <Skeleton className="h-7 w-48" />
        </div>
        <Skeleton className="h-4 w-80" />
        <Card>
          <CardContent className="p-6 space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <div className="p-2 bg-brand-500/10 rounded-lg">
          <Mail className="h-6 w-6 text-brand-500" />
        </div>
        <h2 className="text-xl font-semibold text-content-primary">Email (SMTP) Settings</h2>
      </div>

      <p className="text-sm text-content-secondary">
        Configure SMTP settings to enable email notifications and user invitations.
      </p>

      {/* SMTP Configuration Form */}
      <Card>
        <CardHeader>
          <CardTitle>SMTP Configuration</CardTitle>
          <CardDescription>
            Enter your SMTP server details to enable email functionality.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="smtp-host" required>SMTP Host</Label>
              <Input
                id="smtp-host"
                type="text"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder="smtp.gmail.com"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="smtp-port" required>Port</Label>
              <Input
                id="smtp-port"
                type="number"
                value={port}
                onChange={(e) => setPort(parseInt(e.target.value) || 587)}
                placeholder="587"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="smtp-username">Username</Label>
              <Input
                id="smtp-username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="your@email.com"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="smtp-password">Password</Label>
              <Input
                id="smtp-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                helperText="Use app-specific password for Gmail"
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="smtp-from" required>From Address</Label>
            <Input
              id="smtp-from"
              type="email"
              value={fromAddress}
              onChange={(e) => setFromAddress(e.target.value)}
              placeholder="Charon <no-reply@example.com>"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="smtp-encryption">Encryption</Label>
            <Select value={encryption} onValueChange={(value) => setEncryption(value as 'none' | 'ssl' | 'starttls')}>
              <SelectTrigger id="smtp-encryption">
                <SelectValue placeholder="Select encryption" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="starttls">STARTTLS (Recommended)</SelectItem>
                <SelectItem value="ssl">SSL/TLS</SelectItem>
                <SelectItem value="none">None</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
        <CardFooter className="justify-end gap-3">
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
        </CardFooter>
      </Card>

      {/* Status Indicator */}
      <Card>
        <CardContent className="py-4">
          <div className="flex items-center gap-3">
            {smtpConfig?.configured ? (
              <>
                <CheckCircle2 className="h-5 w-5 text-success" />
                <span className="font-medium text-content-primary">SMTP Configured</span>
                <Badge variant="success" size="sm">Active</Badge>
              </>
            ) : (
              <>
                <XCircle className="h-5 w-5 text-warning" />
                <span className="font-medium text-content-primary">SMTP Not Configured</span>
                <Badge variant="warning" size="sm">Inactive</Badge>
              </>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Test Email */}
      {smtpConfig?.configured && (
        <Card>
          <CardHeader>
            <CardTitle>Send Test Email</CardTitle>
            <CardDescription>
              Send a test email to verify your SMTP configuration is working correctly.
            </CardDescription>
          </CardHeader>
          <CardContent>
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
          </CardContent>
        </Card>
      )}

      {/* Help Alert */}
      <Alert variant="info" title="Need Help?">
        If you're using Gmail, you'll need to enable 2-factor authentication and create an app-specific password.
        For other providers, check their SMTP documentation for the correct settings.
      </Alert>
    </div>
  )
}
