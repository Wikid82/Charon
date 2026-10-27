import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Gauge, Info } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { STANDARD_RATE_LIMIT } from '../api/security'
import { updateSetting } from '../api/settings'
import { ConfigReloadOverlay } from '../components/LoadingStates'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { useSecurityStatus, useSecurityConfig, useUpdateSecurityConfig } from '../hooks/useSecurity'
import { toast } from '../utils/toast'

type RateLimitField = 'requests' | 'burst' | 'window'

/** Field inputs are kept as text so an empty field is representable while the user types. */
type FieldValues = Record<RateLimitField, string>

type FieldErrors = Partial<Record<RateLimitField, string>>

/**
 * Text shown in a field for a stored value. A missing value falls back to the standard number; a stored 0
 * (or any non-positive number) shows as an empty field so the standard number appears only as a placeholder.
 */
const toFieldText = (stored: unknown, standard: number): string => {
  if (typeof stored !== 'number') return String(standard)
  return stored > 0 ? String(stored) : ''
}

/** Parses a field as a whole number greater than 0; returns null when it is empty, zero or not a whole number. */
const parsePositiveInt = (text: string): number | null => {
  const trimmed = text.trim()
  if (!/^\d+$/.test(trimmed)) return null
  const value = Number.parseInt(trimmed, 10)
  return value > 0 ? value : null
}

export default function RateLimiting() {
  const { t } = useTranslation()
  const { data: status, isLoading: statusLoading } = useSecurityStatus()
  const { data: configData, isLoading: configLoading } = useSecurityConfig()
  const updateConfigMutation = useUpdateSecurityConfig()
  const queryClient = useQueryClient()

  const [fields, setFields] = useState<FieldValues>({
    requests: String(STANDARD_RATE_LIMIT.requests),
    burst: String(STANDARD_RATE_LIMIT.burst),
    window: String(STANDARD_RATE_LIMIT.windowSec),
  })
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})

  const config = configData?.config

  // Sync local state with fetched config
  useEffect(() => {
    if (config) {
      setFields({
        requests: toFieldText(config.rate_limit_requests, STANDARD_RATE_LIMIT.requests),
        burst: toFieldText(config.rate_limit_burst, STANDARD_RATE_LIMIT.burst),
        window: toFieldText(config.rate_limit_window_sec, STANDARD_RATE_LIMIT.windowSec),
      })
    }
  }, [config])

  const updateField = (field: RateLimitField, text: string) => {
    setFields((prev) => ({ ...prev, [field]: text }))
    setFieldErrors((prev) => ({ ...prev, [field]: undefined }))
  }

  const toggleMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      await updateSetting('security.rate_limit.enabled', enabled ? 'true' : 'false', 'security', 'bool')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['securityStatus'] })
      // The enable path writes the thresholds on the backend; refresh the config so they appear.
      queryClient.invalidateQueries({ queryKey: ['securityConfig'] })
      toast.success(t('rateLimiting.settingUpdated'))
    },
    onError: (err: Error) => {
      toast.error(`${t('common.failedToUpdate')}: ${err.message}`)
    },
  })

  const handleToggle = () => {
    const newValue = !status?.rate_limit?.enabled
    toggleMutation.mutate(newValue)
  }

  const handleSave = () => {
    const requests = parsePositiveInt(fields.requests)
    const burst = parsePositiveInt(fields.burst)
    const windowSec = parsePositiveInt(fields.window)

    // Refuse zero or empty values client-side so a save never writes 0 over the thresholds.
    const invalidMessage = t('rateLimiting.invalidValue')
    const errors: FieldErrors = {
      requests: requests === null ? invalidMessage : undefined,
      burst: burst === null ? invalidMessage : undefined,
      window: windowSec === null ? invalidMessage : undefined,
    }
    setFieldErrors(errors)
    if (requests === null || burst === null || windowSec === null) return

    updateConfigMutation.mutate({
      rate_limit_requests: requests,
      rate_limit_burst: burst,
      rate_limit_window_sec: windowSec,
    })
  }

  const isApplyingConfig = toggleMutation.isPending || updateConfigMutation.isPending

  if (statusLoading || configLoading) {
    return <div className="p-8 text-center text-white">{t('common.loading')}</div>
  }

  const enabled = status?.rate_limit?.enabled ?? false

  return (
    <>
      {isApplyingConfig && (
        <ConfigReloadOverlay
          message={t('rateLimiting.adjustingGates')}
          submessage={t('rateLimiting.configurationUpdating')}
          type="cerberus"
        />
      )}
      <div className="space-y-6">
        {/* Header */}
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Gauge className="w-7 h-7 text-blue-400" />
            {t('rateLimiting.title')}
          </h1>
          <p className="text-gray-400 mt-1">
            {t('rateLimiting.description')}
          </p>
        </div>

        {/* Info Banner */}
        <div className="bg-blue-900/20 border border-blue-800/50 rounded-lg p-4">
          <div className="flex items-start gap-3">
            <Info className="h-5 w-5 text-blue-400 shrink-0 mt-0.5" />
            <div>
              <h3 className="text-sm font-semibold text-blue-300 mb-1">
                {t('rateLimiting.aboutTitle')}
              </h3>
              <p className="text-sm text-blue-200/90">
                {t('rateLimiting.aboutDescription')}
              </p>
            </div>
          </div>
        </div>

        {/* Active Settings Summary */}
        {enabled && config && (
          <Card className="bg-green-900/20 border-green-800/50">
            <div className="flex items-center gap-4">
              <div className="text-green-400 text-2xl">✓</div>
              <div>
                <h3 className="text-sm font-semibold text-green-300">{t('rateLimiting.currentlyActive')}</h3>
                <p className="text-sm text-green-200/90">
                  {t('rateLimiting.activeSummary', {
                    requests: config.rate_limit_requests,
                    burst: config.rate_limit_burst,
                    window: config.rate_limit_window_sec
                  })}
                </p>
              </div>
            </div>
          </Card>
        )}

        {/* Enable/Disable Toggle */}
        <Card>
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-lg font-semibold text-white">{t('rateLimiting.enableRateLimiting')}</h2>
              <p className="text-sm text-gray-400 mt-1">
                {enabled
                  ? t('rateLimiting.activeDescription')
                  : t('rateLimiting.disabledDescription')}
              </p>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={enabled}
                onChange={handleToggle}
                disabled={toggleMutation.isPending}
                className="sr-only peer"
                data-testid="rate-limit-toggle"
              />
              <div className="w-11 h-6 bg-gray-700 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-500 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-0.5 after:left-0.5 after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
            </label>
          </div>
        </Card>

        {/* Configuration Section - Only visible when enabled */}
        {enabled && (
          <Card>
            <h2 className="text-lg font-semibold text-white mb-4">{t('rateLimiting.configuration')}</h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <Input
                label={t('rateLimiting.requestsPerSecond')}
                type="number"
                min={1}
                max={1000}
                value={fields.requests}
                placeholder={String(STANDARD_RATE_LIMIT.requests)}
                onChange={(e) => updateField('requests', e.target.value)}
                helperText={t('rateLimiting.requestsPerSecondHelper')}
                error={fieldErrors.requests}
                errorTestId="rate-limit-rps-error"
                data-testid="rate-limit-rps"
              />
              <Input
                label={t('rateLimiting.burst')}
                type="number"
                min={1}
                max={100}
                value={fields.burst}
                placeholder={String(STANDARD_RATE_LIMIT.burst)}
                onChange={(e) => updateField('burst', e.target.value)}
                helperText={t('rateLimiting.burstHelper')}
                error={fieldErrors.burst}
                errorTestId="rate-limit-burst-error"
                data-testid="rate-limit-burst"
              />
              <Input
                label={t('rateLimiting.windowSeconds')}
                type="number"
                min={1}
                max={3600}
                value={fields.window}
                placeholder={String(STANDARD_RATE_LIMIT.windowSec)}
                onChange={(e) => updateField('window', e.target.value)}
                helperText={t('rateLimiting.windowSecondsHelper')}
                error={fieldErrors.window}
                errorTestId="rate-limit-window-error"
                data-testid="rate-limit-window"
              />
            </div>
            <div className="mt-6 flex justify-end">
              <Button
                onClick={handleSave}
                isLoading={updateConfigMutation.isPending}
                data-testid="save-rate-limit-btn"
              >
                {t('rateLimiting.saveConfiguration')}
              </Button>
            </div>
          </Card>
        )}

        {/* Guidance when disabled */}
        {!enabled && (
          <Card>
            <div className="text-center py-8">
              <div className="text-gray-500 mb-4 text-4xl">⏱️</div>
              <h3 className="text-lg font-semibold text-white mb-2">{t('rateLimiting.disabledTitle')}</h3>
              <p className="text-gray-400 mb-4">
                {t('rateLimiting.disabledMessage')}
              </p>
            </div>
          </Card>
        )}
      </div>
    </>
  )
}
