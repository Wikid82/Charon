import { useState, useEffect } from 'react'
import { Gauge, Info } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Card } from '../components/ui/Card'
import { useSecurityStatus, useSecurityConfig, useUpdateSecurityConfig } from '../hooks/useSecurity'
import { updateSetting } from '../api/settings'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from '../utils/toast'
import { ConfigReloadOverlay } from '../components/LoadingStates'

export default function RateLimiting() {
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
      toast.success('Rate limiting setting updated')
    },
    onError: (err: Error) => {
      toast.error(`Failed to update: ${err.message}`)
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
    return <div className="p-8 text-center text-white">Loading...</div>
  }

  const enabled = status?.rate_limit?.enabled ?? false

  return (
    <>
      {isApplyingConfig && (
        <ConfigReloadOverlay
          message="Adjusting the gates..."
          submessage="Rate limiting configuration updating"
          type="cerberus"
        />
      )}
      <div className="space-y-6">
        {/* Header */}
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Gauge className="w-7 h-7 text-blue-400" />
            Rate Limiting Configuration
          </h1>
          <p className="text-gray-400 mt-1">
            Control request rates to protect your services from abuse
          </p>
        </div>

        {/* Info Banner */}
        <div className="bg-blue-900/20 border border-blue-800/50 rounded-lg p-4">
          <div className="flex items-start gap-3">
            <Info className="h-5 w-5 text-blue-400 flex-shrink-0 mt-0.5" />
            <div>
              <h3 className="text-sm font-semibold text-blue-300 mb-1">
                About Rate Limiting
              </h3>
              <p className="text-sm text-blue-200/90">
                Rate limiting helps protect your services from abuse, brute-force attacks, and
                excessive resource consumption. Configure limits per client IP address.
              </p>
            </div>
          </div>
        </div>

        {/* Enable/Disable Toggle */}
        <Card>
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-lg font-semibold text-white">Enable Rate Limiting</h2>
              <p className="text-sm text-gray-400 mt-1">
                {enabled
                  ? 'Rate limiting is active and protecting your services'
                  : 'Enable to start limiting request rates'}
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
              <div className="w-11 h-6 bg-gray-700 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-500 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
            </label>
          </div>
        </Card>

        {/* Configuration Section - Only visible when enabled */}
        {enabled && (
          <Card>
            <h2 className="text-lg font-semibold text-white mb-4">Configuration</h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <Input
                label="Requests per Second"
                type="number"
                min={1}
                max={1000}
                value={rps}
                onChange={(e) => setRps(parseInt(e.target.value, 10) || 1)}
                helperText="Maximum requests allowed per second per client"
                data-testid="rate-limit-rps"
              />
              <Input
                label="Burst"
                type="number"
                min={1}
                max={100}
                value={burst}
                onChange={(e) => setBurst(parseInt(e.target.value, 10) || 1)}
                helperText="Allow short bursts above the rate limit"
                data-testid="rate-limit-burst"
              />
              <Input
                label="Window (seconds)"
                type="number"
                min={1}
                max={3600}
                value={window}
                onChange={(e) => setWindow(parseInt(e.target.value, 10) || 1)}
                helperText="Time window for rate calculations"
                data-testid="rate-limit-window"
              />
            </div>
            <div className="mt-6 flex justify-end">
              <Button
                onClick={handleSave}
                isLoading={updateConfigMutation.isPending}
                data-testid="save-rate-limit-btn"
              >
                Save Configuration
              </Button>
            </div>
          </Card>
        )}

        {/* Guidance when disabled */}
        {!enabled && (
          <Card>
            <div className="text-center py-8">
              <div className="text-gray-500 mb-4 text-4xl">⏱️</div>
              <h3 className="text-lg font-semibold text-white mb-2">Rate Limiting Disabled</h3>
              <p className="text-gray-400 mb-4">
                Enable rate limiting to configure request limits and protect your services
              </p>
            </div>
          </Card>
        )}
      </div>
    </>
  )
}
