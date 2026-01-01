import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
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
      toast.success(t('smtp.settingsSaved'))
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || t('smtp.saveFailed'))
    },
  })

  const testConnectionMutation = useMutation({
    mutationFn: testSMTPConnection,
    onSuccess: (data) => {
      if (data.success) {
        toast.success(data.message || t('smtp.connectionSuccess'))
      } else {
        toast.error(data.error || t('smtp.connectionFailed'))
      }
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || t('smtp.testFailed'))
    },
  })

  const sendTestEmailMutation = useMutation({
    mutationFn: async () => sendTestEmail({ to: testEmail }),
    onSuccess: (data) => {
      if (data.success) {
        toast.success(data.message || t('smtp.testEmailSent'))
        setTestEmail('')
      } else {
        toast.error(data.error || t('smtp.testEmailFailed'))
      }
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || t('smtp.testEmailFailed'))
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
        <h2 className="text-xl font-semibold text-content-primary">{t('smtp.title')}</h2>
      </div>

      <p className="text-sm text-content-secondary">
        {t('smtp.description')}
      </p>

      {/* SMTP Configuration Form */}
      <Card>
        <CardHeader>
          <CardTitle>{t('smtp.configuration')}</CardTitle>
          <CardDescription>
            {t('smtp.configDescription')}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="smtp-host" required>{t('smtp.host')}</Label>
              <Input
                id="smtp-host"
                type="text"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder="smtp.gmail.com"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="smtp-port" required>{t('smtp.port')}</Label>
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
              <Label htmlFor="smtp-username">{t('smtp.username')}</Label>
              <Input
                id="smtp-username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="your@email.com"
                autoComplete="username"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="smtp-password">{t('smtp.password')}</Label>
              <Input
                id="smtp-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                helperText={t('smtp.passwordHelper')}
                autoComplete="current-password"
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="smtp-from" required>{t('smtp.fromAddress')}</Label>
            <Input
              id="smtp-from"
              type="email"
              value={fromAddress}
              onChange={(e) => setFromAddress(e.target.value)}
              placeholder="Charon <no-reply@example.com>"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="smtp-encryption">{t('smtp.encryption')}</Label>
            <Select value={encryption} onValueChange={(value) => setEncryption(value as 'none' | 'ssl' | 'starttls')}>
              <SelectTrigger id="smtp-encryption">
                <SelectValue placeholder={t('smtp.selectEncryption')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="starttls">{t('smtp.starttls')}</SelectItem>
                <SelectItem value="ssl">{t('smtp.sslTls')}</SelectItem>
                <SelectItem value="none">{t('smtp.none')}</SelectItem>
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
            {t('smtp.testConnection')}
          </Button>
          <Button
            onClick={() => saveMutation.mutate()}
            isLoading={saveMutation.isPending}
          >
            {t('smtp.saveSettings')}
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
                <span className="font-medium text-content-primary">{t('smtp.configured')}</span>
                <Badge variant="success" size="sm">{t('smtp.active')}</Badge>
              </>
            ) : (
              <>
                <XCircle className="h-5 w-5 text-warning" />
                <span className="font-medium text-content-primary">{t('smtp.notConfigured')}</span>
                <Badge variant="warning" size="sm">{t('smtp.inactive')}</Badge>
              </>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Test Email */}
      {smtpConfig?.configured && (
        <Card>
          <CardHeader>
            <CardTitle>{t('smtp.sendTestEmail')}</CardTitle>
            <CardDescription>
              {t('smtp.testEmailDescription')}
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
                {t('smtp.sendTest')}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Help Alert */}
      <Alert variant="info" title={t('smtp.needHelp')}>
        {t('smtp.helpText')}
      </Alert>
    </div>
  )
}
