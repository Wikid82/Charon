import { useState, useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
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
import { WebSocketStatusCard } from '../components/WebSocketStatusCard'
import { LanguageSelector } from '../components/LanguageSelector'

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
  const { t } = useTranslation()
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
      toast.success(t('systemSettings.settingsSaved'))
    },
    onError: (error: Error) => {
      toast.error(t('systemSettings.settingsFailed', { error: error.message }))
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
        label: t('systemSettings.features.cerberus'),
        tooltip: t('systemSettings.features.cerberusTooltip'),
      },
      {
        key: 'feature.crowdsec.console_enrollment',
        label: t('systemSettings.features.crowdsecConsole'),
        tooltip: t('systemSettings.features.crowdsecConsoleTooltip'),
      },
      {
        key: 'feature.uptime.enabled',
        label: t('systemSettings.features.uptimeMonitoring'),
        tooltip: t('systemSettings.features.uptimeMonitoringTooltip'),
      },
    ],
    [t]
  )

  const updateFlagMutation = useMutation({
    mutationFn: async (payload: Record<string, boolean>) => updateFeatureFlags(payload),
    onSuccess: () => {
      refetchFlags()
      toast.success(t('systemSettings.featureFlagUpdated'))
    },
    onError: (err: unknown) => {
      const msg = err instanceof Error ? err.message : String(err)
      toast.error(t('systemSettings.featureFlagFailed', { error: msg }))
    },
  })

  // CrowdSec control

  // Determine loading message
  const { message, submessage } = updateFlagMutation.isPending
    ? { message: t('systemSettings.updatingFeatures'), submessage: t('systemSettings.applyingChanges') }
    : { message: t('common.loading'), submessage: t('systemSettings.pleaseWait') }

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
          <h1 className="text-2xl font-bold text-content-primary">{t('systemSettings.title')}</h1>
        </div>

        {/* Features */}
        <Card>
          <CardHeader>
            <CardTitle>{t('systemSettings.features.title')}</CardTitle>
            <CardDescription>{t('systemSettings.features.description')}</CardDescription>
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
            <CardTitle>{t('systemSettings.general.title')}</CardTitle>
            <CardDescription>{t('systemSettings.general.description')}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="space-y-2">
              <Label htmlFor="caddy-api">{t('systemSettings.general.caddyAdminApi')}</Label>
              <Input
                id="caddy-api"
                type="text"
                value={caddyAdminAPI}
                onChange={(e) => setCaddyAdminAPI(e.target.value)}
                placeholder="http://localhost:2019"
                helperText={t('systemSettings.general.caddyAdminApiHelper')}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="ssl-provider">{t('systemSettings.general.sslProvider')}</Label>
              <Select value={sslProvider} onValueChange={setSslProvider}>
                <SelectTrigger id="ssl-provider">
                  <SelectValue placeholder={t('systemSettings.general.selectSslProvider')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="auto">{t('systemSettings.general.sslAuto')}</SelectItem>
                  <SelectItem value="letsencrypt-prod">{t('systemSettings.general.sslLetsEncryptProd')}</SelectItem>
                  <SelectItem value="letsencrypt-staging">{t('systemSettings.general.sslLetsEncryptStaging')}</SelectItem>
                  <SelectItem value="zerossl">{t('systemSettings.general.sslZeroSSL')}</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-sm text-content-muted">
                {t('systemSettings.general.sslProviderHelper')}
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="domain-behavior">{t('systemSettings.general.domainLinkBehavior')}</Label>
              <Select value={domainLinkBehavior} onValueChange={setDomainLinkBehavior}>
                <SelectTrigger id="domain-behavior">
                  <SelectValue placeholder={t('systemSettings.general.selectLinkBehavior')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="same_tab">{t('systemSettings.general.sameTab')}</SelectItem>
                  <SelectItem value="new_tab">{t('systemSettings.general.newTab')}</SelectItem>
                  <SelectItem value="new_window">{t('systemSettings.general.newWindow')}</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-sm text-content-muted">
                {t('systemSettings.general.domainLinkBehaviorHelper')}
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="language">{t('common.language')}</Label>
              <LanguageSelector />
              <p className="text-sm text-content-muted">
                {t('systemSettings.general.languageHelper')}
              </p>
            </div>
          </CardContent>
          <CardFooter className="justify-end">
            <Button
              onClick={() => saveSettingsMutation.mutate()}
              isLoading={saveSettingsMutation.isPending}
            >
              <Save className="h-4 w-4 mr-2" />
              {t('systemSettings.saveSettings')}
            </Button>
          </CardFooter>
        </Card>

        {/* System Status */}
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <Activity className="h-5 w-5 text-success" />
              <CardTitle>{t('systemSettings.systemStatus.title')}</CardTitle>
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
                  <Label variant="muted">{t('systemSettings.systemStatus.service')}</Label>
                  <p className="text-lg font-medium text-content-primary">{health.service}</p>
                </div>
                <div className="space-y-1">
                  <Label variant="muted">{t('common.status')}</Label>
                  <div className="flex items-center gap-2">
                    <Badge variant={health.status === 'healthy' ? 'success' : 'error'}>
                      {health.status === 'healthy' ? t('dashboard.healthy') : t('dashboard.unhealthy')}
                    </Badge>
                  </div>
                </div>
                <div className="space-y-1">
                  <Label variant="muted">{t('systemSettings.systemStatus.version')}</Label>
                  <p className="text-lg font-medium text-content-primary">{health.version}</p>
                </div>
                <div className="space-y-1">
                  <Label variant="muted">{t('systemSettings.systemStatus.buildTime')}</Label>
                  <p className="text-lg font-medium text-content-primary">
                    {health.build_time || t('systemSettings.systemStatus.notAvailable')}
                  </p>
                </div>
                <div className="md:col-span-2 space-y-1">
                  <Label variant="muted">{t('systemSettings.systemStatus.gitCommit')}</Label>
                  <p className="text-sm font-mono text-content-secondary bg-surface-subtle px-3 py-2 rounded-md">
                    {health.git_commit || t('systemSettings.systemStatus.notAvailable')}
                  </p>
                </div>
              </div>
            ) : (
              <Alert variant="error">
                {t('systemSettings.systemStatus.fetchError')}
              </Alert>
            )}
          </CardContent>
        </Card>

        {/* Update Check */}
        <Card>
          <CardHeader>
            <CardTitle>{t('systemSettings.updates.title')}</CardTitle>
            <CardDescription>{t('systemSettings.updates.description')}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {updateInfo && (
              <>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div className="space-y-1">
                    <Label variant="muted">{t('systemSettings.updates.currentVersion')}</Label>
                    <p className="text-lg font-medium text-content-primary">
                      {updateInfo.current_version}
                    </p>
                  </div>
                  <div className="space-y-1">
                    <Label variant="muted">{t('systemSettings.updates.latestVersion')}</Label>
                    <p className="text-lg font-medium text-content-primary">
                      {updateInfo.latest_version}
                    </p>
                  </div>
                </div>

                {updateInfo.update_available ? (
                  <Alert variant="info" title={t('systemSettings.updates.updateAvailable')}>
                    {t('systemSettings.updates.newVersionAvailable')}{' '}
                    {updateInfo.release_url && (
                      <a
                        href={updateInfo.release_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1 text-brand-500 hover:underline font-medium"
                      >
                        {t('systemSettings.updates.viewReleaseNotes')}
                        <ExternalLink className="h-3 w-3" />
                      </a>
                    )}
                  </Alert>
                ) : (
                  <Alert variant="success" title={t('systemSettings.updates.upToDate')}>
                    {t('systemSettings.updates.runningLatest')}
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
              {t('systemSettings.updates.checkForUpdates')}
            </Button>
          </CardFooter>
        </Card>

        {/* WebSocket Connection Status */}
        <WebSocketStatusCard showDetails={true} />
      </div>
    </TooltipProvider>
  )
}
