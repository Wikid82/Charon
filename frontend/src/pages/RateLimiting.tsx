import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Gauge, Info } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { updateSetting } from '../api/settings'
import { ConfigReloadOverlay } from '../components/LoadingStates'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { useSecurityStatus, useSecurityConfig, useUpdateSecurityConfig } from '../hooks/useSecurity'
import { toast } from '../utils/toast'

export default function RateLimiting() {
  const { t } = useTranslation()
  const { data: status, isLoading: statusLoading } = useSecurityStatus()
  const { data: configData, isLoading: configLoading } = useSecurityConfig()
  const updateConfigMutation = useUpdateSecurityConfig()
  const queryClient = useQueryClient()

  const [rps, setRps] = useState(10)
  const [burst, setBurst] = useState(5)
  const [window, setWindow] = useState(60)

  const config = configData?.config

  // Sync local state with fetched config
  useEffect(() => {
    if (config) {
      setRps(config.rate_limit_requests ?? 10)
      setBurst(config.rate_limit_burst ?? 5)
      setWindow(config.rate_limit_window_sec ?? 60)
    }
  }, [config])

  const toggleMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      await updateSetting('security.rate_limit.enabled', enabled ? 'true' : 'false', 'security', 'bool')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['securityStatus'] })
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
    updateConfigMutation.mutate({
      rate_limit_requests: rps,
      rate_limit_burst: burst,
      rate_limit_window_sec: window,
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
                value={rps}
                onChange={(e) => setRps(parseInt(e.target.value, 10) || 1)}
                helperText={t('rateLimiting.requestsPerSecondHelper')}
                data-testid="rate-limit-rps"
              />
              <Input
                label={t('rateLimiting.burst')}
                type="number"
                min={1}
                max={100}
                value={burst}
                onChange={(e) => setBurst(parseInt(e.target.value, 10) || 1)}
                helperText={t('rateLimiting.burstHelper')}
                data-testid="rate-limit-burst"
              />
              <Input
                label={t('rateLimiting.windowSeconds')}
                type="number"
                min={1}
                max={3600}
                value={window}
                onChange={(e) => setWindow(parseInt(e.target.value, 10) || 1)}
                helperText={t('rateLimiting.windowSecondsHelper')}
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
