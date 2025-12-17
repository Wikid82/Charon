import { useState, useEffect, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Switch } from '../components/ui/Switch'
import { Label } from '../components/ui/Label'
import { Alert } from '../components/ui/Alert'
import { Badge } from '../components/ui/Badge'
import { Skeleton } from '../components/ui/Skeleton'
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../components/ui/Select'
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from '../components/ui/Tooltip'
import { toast } from '../utils/toast'
import { getSettings, updateSetting } from '../api/settings'
import { getFeatureFlags, updateFeatureFlags } from '../api/featureFlags'
import client from '../api/client'
import { Server, RefreshCw, Save, Activity, Info, ExternalLink } from 'lucide-react'
import { ConfigReloadOverlay } from '../components/LoadingStates'

interface HealthResponse {
  status: string
  service: string
  version: string
  git_commit: string
  build_time: string
}

interface UpdateInfo {
  current_version: string
  latest_version: string
  update_available: boolean
  release_url?: string
}

export default function SystemSettings() {
  const queryClient = useQueryClient()
  const [caddyAdminAPI, setCaddyAdminAPI] = useState('http://localhost:2019')
  const [sslProvider, setSslProvider] = useState('auto')
  const [domainLinkBehavior, setDomainLinkBehavior] = useState('new_tab')

  // Fetch Settings
  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
  })

  // Update local state when settings load
  useEffect(() => {
    if (settings) {
      if (settings['caddy.admin_api']) setCaddyAdminAPI(settings['caddy.admin_api'])
      // Default to 'auto' if empty or invalid value
      if (settings['caddy.ssl_provider']) {
        const validProviders = ['auto', 'letsencrypt-staging', 'letsencrypt-prod', 'zerossl']
        const provider = settings['caddy.ssl_provider']
        setSslProvider(validProviders.includes(provider) ? provider : 'auto')
      }
      if (settings['ui.domain_link_behavior']) setDomainLinkBehavior(settings['ui.domain_link_behavior'])
    }
  }, [settings])

  // Fetch Health/System Status
  const { data: health, isLoading: isLoadingHealth } = useQuery({
    queryKey: ['health'],
    queryFn: async (): Promise<HealthResponse> => {
      const response = await client.get<HealthResponse>('/health')
      return response.data
    },
  })

  // Check for Updates
  const {
    data: updateInfo,
    refetch: checkUpdates,
    isFetching: isCheckingUpdates,
  } = useQuery({
    queryKey: ['updates'],
    queryFn: async (): Promise<UpdateInfo> => {
      const response = await client.get<UpdateInfo>('/system/updates')
      return response.data
    },
    enabled: false, // Manual trigger
  })

  const saveSettingsMutation = useMutation({
    mutationFn: async () => {
      await updateSetting('caddy.admin_api', caddyAdminAPI, 'caddy', 'string')
      await updateSetting('caddy.ssl_provider', sslProvider, 'caddy', 'string')
      await updateSetting('ui.domain_link_behavior', domainLinkBehavior, 'ui', 'string')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      toast.success('System settings saved')
    },
    onError: (error: Error) => {
      toast.error(`Failed to save settings: ${error.message}`)
    },
  })

  // Feature Flags
  const { data: featureFlags, refetch: refetchFlags } = useQuery({
    queryKey: ['feature-flags'],
    queryFn: getFeatureFlags,
  })

  const featureToggles = useMemo(
    () => [
      {
        key: 'feature.cerberus.enabled',
        label: 'Cerberus Security Suite',
        tooltip: 'Advanced security features including WAF, Access Lists, Rate Limiting, and CrowdSec.',
      },
      {
        key: 'feature.crowdsec.console_enrollment',
        label: 'CrowdSec Console Enrollment',
        tooltip: 'Allow enrolling this node with CrowdSec Console for centralized fleet management.',
      },
      {
        key: 'feature.uptime.enabled',
        label: 'Uptime Monitoring',
        tooltip: 'Monitor the availability of your proxy hosts and remote servers.',
      },
    ],
    []
  )

  const updateFlagMutation = useMutation({
    mutationFn: async (payload: Record<string, boolean>) => updateFeatureFlags(payload),
    onSuccess: () => {
      refetchFlags()
      toast.success('Feature flag updated')
    },
    onError: (err: unknown) => {
      const msg = err instanceof Error ? err.message : String(err)
      toast.error(`Failed to update flag: ${msg}`)
    },
  })

  // CrowdSec control

  // Determine loading message
  const { message, submessage } = updateFlagMutation.isPending
    ? { message: 'Updating features...', submessage: 'Applying configuration changes' }
    : { message: 'Loading...', submessage: 'Please wait' }

  // Loading skeleton for settings
  const SettingsSkeleton = () => (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Skeleton className="h-8 w-8" />
        <Skeleton className="h-8 w-48" />
      </div>
      {[1, 2, 3].map((i) => (
        <Card key={i} className="p-6">
          <div className="space-y-4">
            <Skeleton className="h-6 w-32" />
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          </div>
        </Card>
      ))}
    </div>
  )

  // Show skeleton while loading initial data
  if (!settings && !featureFlags) {
    return <SettingsSkeleton />
  }

  return (
    <TooltipProvider>
      {updateFlagMutation.isPending && (
        <ConfigReloadOverlay
          message={message}
          submessage={submessage}
          type="charon"
        />
      )}
      <div className="space-y-6">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-brand-500/10 rounded-lg">
            <Server className="h-6 w-6 text-brand-500" />
          </div>
          <h1 className="text-2xl font-bold text-content-primary">System Settings</h1>
        </div>

        {/* Features */}
        <Card>
          <CardHeader>
            <CardTitle>Features</CardTitle>
            <CardDescription>Enable or disable optional features for your Charon instance.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {featureFlags ? (
                featureToggles.map(({ key, label, tooltip }) => (
                  <div
                    key={key}
                    className="flex items-center justify-between p-4 bg-surface-subtle rounded-lg border border-border"
                  >
                    <div className="flex items-center gap-2">
                      <Label className="text-sm font-medium cursor-default">{label}</Label>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <button type="button" className="text-content-muted hover:text-content-primary">
                            <Info className="h-4 w-4" />
                          </button>
                        </TooltipTrigger>
                        <TooltipContent className="max-w-xs">
                          <p>{tooltip}</p>
                        </TooltipContent>
                      </Tooltip>
                    </div>
                    <Switch
                      aria-label={`${label} toggle`}
                      checked={!!featureFlags[key]}
                      disabled={updateFlagMutation.isPending}
                      onChange={(e) => updateFlagMutation.mutate({ [key]: e.target.checked })}
                    />
                  </div>
                ))
              ) : (
                <div className="col-span-2 space-y-3">
                  <Skeleton className="h-16 w-full" />
                  <Skeleton className="h-16 w-full" />
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {/* General Configuration */}
        <Card>
          <CardHeader>
            <CardTitle>General Configuration</CardTitle>
            <CardDescription>Configure Caddy and UI preferences.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="space-y-2">
              <Label htmlFor="caddy-api">Caddy Admin API Endpoint</Label>
              <Input
                id="caddy-api"
                type="text"
                value={caddyAdminAPI}
                onChange={(e) => setCaddyAdminAPI(e.target.value)}
                placeholder="http://localhost:2019"
                helperText="URL to the Caddy admin API (usually on port 2019)"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="ssl-provider">SSL Provider</Label>
              <Select value={sslProvider} onValueChange={setSslProvider}>
                <SelectTrigger id="ssl-provider">
                  <SelectValue placeholder="Select SSL provider" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="auto">Auto (Recommended)</SelectItem>
                  <SelectItem value="letsencrypt-prod">Let's Encrypt (Prod)</SelectItem>
                  <SelectItem value="letsencrypt-staging">Let's Encrypt (Staging)</SelectItem>
                  <SelectItem value="zerossl">ZeroSSL</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-sm text-content-muted">
                Choose the Certificate Authority. 'Auto' uses Let's Encrypt with ZeroSSL fallback.
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="domain-behavior">Domain Link Behavior</Label>
              <Select value={domainLinkBehavior} onValueChange={setDomainLinkBehavior}>
                <SelectTrigger id="domain-behavior">
                  <SelectValue placeholder="Select link behavior" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="same_tab">Same Tab</SelectItem>
                  <SelectItem value="new_tab">New Tab (Default)</SelectItem>
                  <SelectItem value="new_window">New Window</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-sm text-content-muted">
                Control how domain links open in the Proxy Hosts list.
              </p>
            </div>
          </CardContent>
          <CardFooter className="justify-end">
            <Button
              onClick={() => saveSettingsMutation.mutate()}
              isLoading={saveSettingsMutation.isPending}
            >
              <Save className="h-4 w-4 mr-2" />
              Save Settings
            </Button>
          </CardFooter>
        </Card>

        {/* System Status */}
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <Activity className="h-5 w-5 text-success" />
              <CardTitle>System Status</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            {isLoadingHealth ? (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {[1, 2, 3, 4].map((i) => (
                  <div key={i} className="space-y-2">
                    <Skeleton className="h-4 w-20" />
                    <Skeleton className="h-6 w-32" />
                  </div>
                ))}
              </div>
            ) : health ? (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div className="space-y-1">
                  <Label variant="muted">Service</Label>
                  <p className="text-lg font-medium text-content-primary">{health.service}</p>
                </div>
                <div className="space-y-1">
                  <Label variant="muted">Status</Label>
                  <div className="flex items-center gap-2">
                    <Badge variant={health.status === 'healthy' ? 'success' : 'error'}>
                      {health.status}
                    </Badge>
                  </div>
                </div>
                <div className="space-y-1">
                  <Label variant="muted">Version</Label>
                  <p className="text-lg font-medium text-content-primary">{health.version}</p>
                </div>
                <div className="space-y-1">
                  <Label variant="muted">Build Time</Label>
                  <p className="text-lg font-medium text-content-primary">
                    {health.build_time || 'N/A'}
                  </p>
                </div>
                <div className="md:col-span-2 space-y-1">
                  <Label variant="muted">Git Commit</Label>
                  <p className="text-sm font-mono text-content-secondary bg-surface-subtle px-3 py-2 rounded-md">
                    {health.git_commit || 'N/A'}
                  </p>
                </div>
              </div>
            ) : (
              <Alert variant="error">
                Unable to fetch system status. Please check your connection.
              </Alert>
            )}
          </CardContent>
        </Card>

        {/* Update Check */}
        <Card>
          <CardHeader>
            <CardTitle>Software Updates</CardTitle>
            <CardDescription>Check for new versions of Charon.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {updateInfo && (
              <>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div className="space-y-1">
                    <Label variant="muted">Current Version</Label>
                    <p className="text-lg font-medium text-content-primary">
                      {updateInfo.current_version}
                    </p>
                  </div>
                  <div className="space-y-1">
                    <Label variant="muted">Latest Version</Label>
                    <p className="text-lg font-medium text-content-primary">
                      {updateInfo.latest_version}
                    </p>
                  </div>
                </div>

                {updateInfo.update_available ? (
                  <Alert variant="info" title="Update Available">
                    A new version of Charon is available!{' '}
                    {updateInfo.release_url && (
                      <a
                        href={updateInfo.release_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1 text-brand-500 hover:underline font-medium"
                      >
                        View Release Notes
                        <ExternalLink className="h-3 w-3" />
                      </a>
                    )}
                  </Alert>
                ) : (
                  <Alert variant="success" title="Up to Date">
                    You are running the latest version of Charon.
                  </Alert>
                )}
              </>
            )}
          </CardContent>
          <CardFooter>
            <Button
              onClick={() => checkUpdates()}
              isLoading={isCheckingUpdates}
              variant="secondary"
            >
              <RefreshCw className="h-4 w-4 mr-2" />
              Check for Updates
            </Button>
          </CardFooter>
        </Card>
      </div>
    </TooltipProvider>
  )
}
