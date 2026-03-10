import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Copy, Check, AlertTriangle, X, Eye, EyeOff } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert } from './ui/Alert'
import { Button } from './ui/Button'
import { toast } from '../utils/toast'
import { getCrowdsecKeyStatus, type CrowdSecKeyStatus } from '../api/crowdsec'

const DISMISSAL_STORAGE_KEY = 'crowdsec-key-warning-dismissed'

interface DismissedState {
  dismissed: boolean
  key?: string
}

function getDismissedState(): DismissedState {
  try {
    const stored = localStorage.getItem(DISMISSAL_STORAGE_KEY)
    if (stored) {
      return JSON.parse(stored)
    }
  } catch {
    // Ignore parse errors
  }
  return { dismissed: false }
}

function setDismissedState(fullKey: string) {
  try {
    localStorage.setItem(DISMISSAL_STORAGE_KEY, JSON.stringify({ dismissed: true, key: fullKey }))
  } catch {
    // Ignore storage errors
  }
}

export function CrowdSecKeyWarning() {
  const { t, ready } = useTranslation()
  const [copied, setCopied] = useState(false)
  const [dismissed, setDismissed] = useState(false)
  const [showKey, setShowKey] = useState(false)

  const { data: keyStatus, isLoading } = useQuery<CrowdSecKeyStatus>({
    queryKey: ['crowdsec-key-status'],
    queryFn: getCrowdsecKeyStatus,
    refetchInterval: 60000,
    retry: 1,
  })

  useEffect(() => {
    if (keyStatus?.env_key_rejected && keyStatus.full_key) {
      const storedState = getDismissedState()
      // If dismissed but for a different key, show the warning again
      if (storedState.dismissed && storedState.key !== keyStatus.full_key) {
        setDismissed(false)
      } else if (storedState.dismissed && storedState.key === keyStatus.full_key) {
        setDismissed(true)
      }
    }
  }, [keyStatus])

  const handleCopy = async () => {
    if (!keyStatus?.full_key) return

    try {
      await navigator.clipboard.writeText(keyStatus.full_key)
      setCopied(true)
      toast.success(t('security.crowdsec.keyWarning.copied'))
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error(t('security.crowdsec.copyFailed'))
    }
  }

  const handleDismiss = () => {
    if (keyStatus?.full_key) {
      setDismissedState(keyStatus.full_key)
    }
    setDismissed(true)
  }

  if (!ready || isLoading || !keyStatus?.env_key_rejected || !keyStatus?.full_key || dismissed) {
    return null
  }

  const envVarLine = `CHARON_SECURITY_CROWDSEC_API_KEY=${keyStatus.full_key}`
  const maskedKey = `CHARON_SECURITY_CROWDSEC_API_KEY=${'•'.repeat(Math.min(keyStatus.full_key.length, 40))}`

  return (
    <Alert variant="warning" className="relative">
      <div className="flex flex-col gap-3">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-warning shrink-0" />
            <h4 className="font-semibold text-content-primary">
              {t('security.crowdsec.keyWarning.title')}
            </h4>
          </div>
          <button
            type="button"
            onClick={handleDismiss}
            className="p-1 rounded-md text-content-muted hover:text-content-primary hover:bg-surface-muted transition-colors"
            aria-label={t('common.close')}
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <p className="text-sm text-content-secondary">
          {t('security.crowdsec.keyWarning.description')}
        </p>

        <div className="bg-surface-subtle border border-border rounded-md p-3">
          <p className="text-xs text-content-muted mb-2">
            {t('security.crowdsec.keyWarning.instructions')}
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 bg-surface-elevated rounded px-3 py-2 font-mono text-sm text-content-primary overflow-x-auto whitespace-nowrap">
              {showKey ? envVarLine : maskedKey}
            </code>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowKey(!showKey)}
              className="shrink-0"
              title={showKey ? 'Hide key' : 'Show key'}
            >
              {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleCopy}
              disabled={copied}
              className="shrink-0"
            >
              {copied ? (
                <>
                  <Check className="h-4 w-4 mr-1" />
                  {t('security.crowdsec.keyWarning.copied')}
                </>
              ) : (
                <>
                  <Copy className="h-4 w-4 mr-1" />
                  {t('security.crowdsec.keyWarning.copyButton')}
                </>
              )}
            </Button>
          </div>
        </div>

        <p className="text-xs text-content-muted">
          {t('security.crowdsec.keyWarning.restartNote')}
        </p>
      </div>
    </Alert>
  )
}
