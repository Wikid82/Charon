import { useQuery } from '@tanstack/react-query'
import { Copy, Check, Key, AlertCircle } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge, type BadgeProps } from './ui/Badge'
import { Button } from './ui/Button'
import { Card, CardContent, CardHeader, CardTitle } from './ui/Card'
import { Skeleton } from './ui/Skeleton'
import { type BouncerInfo, type BouncerRegistration, getBouncerInfo, getBouncerKey } from '../api/crowdsec'
import { toast } from '../utils/toast'

/** Badge look and label for each registration state. `unknown` is neutral: the check failed, nothing is wrong. */
const REGISTRATION_BADGE: Record<
  BouncerRegistration,
  { variant: BadgeProps['variant']; labelKey: string }
> = {
  registered: { variant: 'success', labelKey: 'security.crowdsec.registered' },
  not_registered: { variant: 'error', labelKey: 'security.crowdsec.notRegistered' },
  unknown: { variant: 'secondary', labelKey: 'security.crowdsec.registrationUnknown' },
}

/** Servers before the three-state check send only `registered`, so fall back to it. */
function resolveRegistration(info: BouncerInfo): BouncerRegistration {
  if (info.registration) return info.registration
  return info.registered ? 'registered' : 'not_registered'
}

export function CrowdSecBouncerKeyDisplay() {
  const { t, ready } = useTranslation()
  const [copied, setCopied] = useState(false)
  const [isCopying, setIsCopying] = useState(false)

  const { data: info, isLoading, error } = useQuery({
    queryKey: ['crowdsec-bouncer-info'],
    queryFn: getBouncerInfo,
    refetchInterval: 30000,
    retry: 1,
  })

  const handleCopyKey = async () => {
    if (isCopying) return
    setIsCopying(true)

    try {
      const key = await getBouncerKey()
      await navigator.clipboard.writeText(key)
      setCopied(true)
      toast.success(t('security.crowdsec.keyCopied'))
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error(t('security.crowdsec.copyFailed'))
    } finally {
      setIsCopying(false)
    }
  }

  if (!ready || isLoading) {
    return (
      <Card>
        <CardHeader className="pb-2">
          <Skeleton className="h-5 w-32" />
        </CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-5 w-48" />
        </CardContent>
      </Card>
    )
  }

  if (error || !info) {
    return null
  }

  if (info.key_source === 'none') {
    return (
      <Card className="border-yellow-500/30 bg-yellow-500/5">
        <CardContent className="flex items-center gap-2 py-3">
          <AlertCircle className="h-4 w-4 text-yellow-500" />
          <span className="text-sm text-yellow-200">
            {t('security.crowdsec.noKeyConfigured')}
          </span>
        </CardContent>
      </Card>
    )
  }

  const registrationBadge = REGISTRATION_BADGE[resolveRegistration(info)]

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-base">
          <Key className="h-4 w-4" />
          {t('security.crowdsec.bouncerApiKey')}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-center justify-between gap-2">
          <code className="rounded bg-gray-900 px-3 py-1.5 font-mono text-sm text-gray-200">
            {info.key_preview}
          </code>
          <Button
            variant="secondary"
            size="sm"
            onClick={handleCopyKey}
            disabled={copied || isCopying}
          >
            {copied ? (
              <>
                <Check className="mr-1 h-3 w-3" />
                {t('common.success')}
              </>
            ) : (
              <>
                <Copy className="mr-1 h-3 w-3" />
                {t('common.copy')}
              </>
            )}
          </Button>
        </div>

        <div className="flex flex-wrap gap-2">
          <Badge variant={registrationBadge.variant}>{t(registrationBadge.labelKey)}</Badge>
          {info.key_source !== 'memory' && (
            <Badge variant="outline">
              {info.key_source === 'env_var'
                ? t('security.crowdsec.sourceEnvVar')
                : t('security.crowdsec.sourceFile')}
            </Badge>
          )}
        </div>

        {info.key_source === 'memory' && (
          <p className="text-xs text-gray-400">{t('security.crowdsec.keyHeldInMemory')}</p>
        )}

        <p className="text-xs text-gray-400">
          {t('security.crowdsec.keyStoredAt')}: <code className="text-gray-300">{info.file_path}</code>
        </p>
      </CardContent>
    </Card>
  )
}
