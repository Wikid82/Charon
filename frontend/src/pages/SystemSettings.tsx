import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Switch } from '../components/ui/Switch'
import { toast } from '../utils/toast'
import { getSettings, updateSetting } from '../api/settings'
import { getFeatureFlags, updateFeatureFlags } from '../api/featureFlags'
import client from '../api/client'
import { startCrowdsec, stopCrowdsec, statusCrowdsec, importCrowdsecConfig } from '../api/crowdsec'
import { Loader2, Server, RefreshCw, Save, Activity } from 'lucide-react'

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
  const [sslProvider, setSslProvider] = useState('letsencrypt')
  const [domainLinkBehavior, setDomainLinkBehavior] = useState('new_tab')
  const [cerberusEnabled, setCerberusEnabled] = useState(false)

  // Fetch Settings
  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
  })

  // Update local state when settings load
  useEffect(() => {
    if (settings) {
      if (settings['caddy.admin_api']) setCaddyAdminAPI(settings['caddy.admin_api'])
      if (settings['caddy.ssl_provider']) setSslProvider(settings['caddy.ssl_provider'])
      if (settings['ui.domain_link_behavior']) setDomainLinkBehavior(settings['ui.domain_link_behavior'])
      if (settings['security.cerberus.enabled']) setCerberusEnabled(settings['security.cerberus.enabled'] === 'true')
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
      await updateSetting('security.cerberus.enabled', cerberusEnabled ? 'true' : 'false', 'security', 'bool')
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
  const [crowdsecStatus, setCrowdsecStatus] = useState<{ running: boolean; pid?: number } | null>(null)

  const fetchCrowdsecStatus = async () => {
    try {
      const s = await statusCrowdsec()
      setCrowdsecStatus(s)
    } catch {
      setCrowdsecStatus(null)
    }
  }

  useEffect(() => { fetchCrowdsecStatus() }, [])

  const startMutation = useMutation({ mutationFn: () => startCrowdsec(), onSuccess: () => fetchCrowdsecStatus(), onError: (e: unknown) => toast.error(String(e)) })
  const stopMutation = useMutation({ mutationFn: () => stopCrowdsec(), onSuccess: () => fetchCrowdsecStatus(), onError: (e: unknown) => toast.error(String(e)) })

  const importMutation = useMutation({
    mutationFn: async (file: File) => importCrowdsecConfig(file),
    onSuccess: () => { toast.success('CrowdSec config imported'); fetchCrowdsecStatus() },
    onError: (e: unknown) => toast.error(String(e)),
  })

  const handleCrowdsecUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    if (!f) return
    importMutation.mutate(f)
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
        <Server className="w-8 h-8" />
        System Settings
      </h1>

      {/* General Configuration */}
      <Card className="p-6">
        <h2 className="text-lg font-semibold mb-4 text-gray-900 dark:text-white">General Configuration</h2>
        <div className="space-y-4">
          <Input
            label="Caddy Admin API Endpoint"
            type="text"
            value={caddyAdminAPI}
            onChange={(e) => setCaddyAdminAPI(e.target.value)}
            placeholder="http://localhost:2019"
          />
          <p className="text-sm text-gray-500 dark:text-gray-400 -mt-2">
            URL to the Caddy admin API (usually on port 2019)
          </p>

          <div className="w-full">
            <label className="block text-sm font-medium text-gray-300 mb-1.5">
              SSL Provider
            </label>
            <select
              value={sslProvider}
              onChange={(e) => setSslProvider(e.target.value)}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
            >
              <option value="letsencrypt">Let's Encrypt (Default)</option>
              <option value="zerossl">ZeroSSL</option>
            </select>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              Choose the default Certificate Authority for SSL certificates.
            </p>
          </div>

          <div className="w-full">
            <label className="block text-sm font-medium text-gray-300 mb-1.5">
              Domain Link Behavior
            </label>
            <select
              value={domainLinkBehavior}
              onChange={(e) => setDomainLinkBehavior(e.target.value)}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
            >
              <option value="same_tab">Same Tab</option>
              <option value="new_tab">New Tab (Default)</option>
              <option value="new_window">New Window</option>
            </select>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              Control how domain links open in the Proxy Hosts list.
            </p>
          </div>

          {/* Cerberus Security Toggle */}
          <div className="w-full">
            <label className="block text-sm font-medium text-gray-300 mb-1.5">
              Enable Cerberus Security
            </label>
            <div className="flex items-center gap-3">
              <Switch
                checked={cerberusEnabled}
                onChange={(e) => setCerberusEnabled(e.target.checked)}
              />
              <p className="text-sm text-gray-500 dark:text-gray-400 -mt-1">
                Optional suite that includes WAF, ACLs, Rate Limiting, and CrowdSec integration.
              </p>
            </div>
          </div>

          <div className="flex justify-end">
            <Button
              onClick={() => saveSettingsMutation.mutate()}
              isLoading={saveSettingsMutation.isPending}
            >
              <Save className="w-4 h-4 mr-2" />
              Save Settings
            </Button>
          </div>
        </div>
      </Card>

      {/* Feature Flags */}
      <Card className="p-6">
        <h2 className="text-lg font-semibold mb-4 text-gray-900 dark:text-white">Feature Flags</h2>
        <div className="space-y-4">
          {featureFlags ? (
            Object.keys(featureFlags).map((key) => (
              <div key={key} className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-gray-900 dark:text-white">{key}</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Toggle feature {key}</p>
                </div>
                <Switch
                  checked={!!featureFlags[key]}
                  onChange={(e) => updateFlagMutation.mutate({ [key]: e.target.checked })}
                />
              </div>
            ))
          ) : (
            <p className="text-sm text-gray-500">Loading feature flags...</p>
          )}
        </div>
      </Card>

      {/* System Status */}
      <Card className="p-6">
        <h2 className="text-lg font-semibold mb-4 text-gray-900 dark:text-white flex items-center gap-2">
          <Activity className="w-5 h-5" />
          System Status
        </h2>
        {isLoadingHealth ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="w-6 h-6 animate-spin text-blue-500" />
          </div>
        ) : health ? (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Service</p>
              <p className="text-lg font-medium text-gray-900 dark:text-white">{health.service}</p>
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Status</p>
              <p className="text-lg font-medium text-green-600 dark:text-green-400 capitalize">
                {health.status}
              </p>
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Version</p>
              <p className="text-lg font-medium text-gray-900 dark:text-white">{health.version}</p>
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Build Time</p>
              <p className="text-lg font-medium text-gray-900 dark:text-white">
                {health.build_time || 'N/A'}
              </p>
            </div>
            <div className="md:col-span-2">
              <p className="text-sm text-gray-500 dark:text-gray-400">Git Commit</p>
              <p className="text-sm font-mono text-gray-900 dark:text-white">
                {health.git_commit || 'N/A'}
              </p>
            </div>
          </div>
        ) : (
          <p className="text-red-500">Unable to fetch system status</p>
        )}
      </Card>

      {/* Update Check */}
      <Card className="p-6">
        <h2 className="text-lg font-semibold mb-4 text-gray-900 dark:text-white">Software Updates</h2>
        <div className="space-y-4">
          {updateInfo && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
              <div>
                <p className="text-sm text-gray-500 dark:text-gray-400">Current Version</p>
                <p className="text-lg font-medium text-gray-900 dark:text-white">
                  {updateInfo.current_version}
                </p>
              </div>
              <div>
                <p className="text-sm text-gray-500 dark:text-gray-400">Latest Version</p>
                <p className="text-lg font-medium text-gray-900 dark:text-white">
                  {updateInfo.latest_version}
                </p>
              </div>
              {updateInfo.update_available && (
                <div className="md:col-span-2">
                  <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
                    <p className="text-blue-800 dark:text-blue-300 font-medium">
                      A new version is available!
                    </p>
                    {updateInfo.release_url && (
                      <a
                        href={updateInfo.release_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-blue-600 dark:text-blue-400 hover:underline text-sm"
                      >
                        View Release Notes
                      </a>
                    )}
                  </div>
                </div>
              )}
              {!updateInfo.update_available && (
                <div className="md:col-span-2">
                  <p className="text-green-600 dark:text-green-400">
                    ✓ You are running the latest version
                  </p>
                </div>
              )}
            </div>
          )}
          <Button
            onClick={() => checkUpdates()}
            isLoading={isCheckingUpdates}
            variant="secondary"
          >
            <RefreshCw className="w-4 h-4 mr-2" />
            Check for Updates
          </Button>
        </div>
      </Card>

      {/* CrowdSec Controls */}
      <Card className="p-6">
        <h2 className="text-lg font-semibold mb-4 text-gray-900 dark:text-white">CrowdSec</h2>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-900 dark:text-white">Status</p>
              <p className="text-xs text-gray-500 dark:text-gray-400">{crowdsecStatus ? (crowdsecStatus.running ? `Running (pid ${crowdsecStatus.pid})` : 'Stopped') : 'Unknown'}</p>
            </div>
            <div className="flex items-center gap-3">
              <Button onClick={() => startMutation.mutate()} isLoading={startMutation.isPending}>Start</Button>
              <Button onClick={() => stopMutation.mutate()} isLoading={stopMutation.isPending} variant="secondary">Stop</Button>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-300 mb-1.5">Import CrowdSec Config</label>
            <input type="file" onChange={handleCrowdsecUpload} />
            <p className="text-sm text-gray-500 mt-1">Upload a tar.gz or zip with your CrowdSec configuration. Existing config will be backed up.</p>
          </div>
        </div>
      </Card>
    </div>
  )
}
