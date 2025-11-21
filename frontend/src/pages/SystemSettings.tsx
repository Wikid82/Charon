import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { toast } from '../utils/toast'
import { getSettings, updateSetting } from '../api/settings'
import client from '../api/client'
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

  // Fetch Settings
  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
  })

  // Update local state when settings load
  useEffect(() => {
    if (settings) {
      if (settings['caddy.admin_api']) setCaddyAdminAPI(settings['caddy.admin_api'])
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
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      toast.success('System settings saved')
    },
    onError: (error: any) => {
      toast.error(`Failed to save settings: ${error.message}`)
    },
  })

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
    </div>
  )
}
