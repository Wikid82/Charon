import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Shield, ShieldAlert, ShieldCheck, Lock, Activity, ExternalLink } from 'lucide-react'
import { getSecurityStatus } from '../api/security'
import { exportCrowdsecConfig } from '../api/crowdsec'
import { updateSetting } from '../api/settings'
import { Switch } from '../components/ui/Switch'
import { toast } from '../utils/toast'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'

export default function Security() {
  const navigate = useNavigate()
  const { data: status, isLoading } = useQuery({
    queryKey: ['security-status'],
    queryFn: getSecurityStatus,
  })
  const queryClient = useQueryClient()
  // Generic toggle mutation for per-service settings
  const toggleServiceMutation = useMutation({
    mutationFn: async ({ key, enabled }: { key: string; enabled: boolean }) => {
      await updateSetting(key, enabled ? 'true' : 'false', 'security', 'bool')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      queryClient.invalidateQueries({ queryKey: ['security-status'] })
      toast.success('Security setting updated')
    },
    onError: (err: unknown) => {
      const msg = err instanceof Error ? err.message : String(err)
      toast.error(`Failed to update setting: ${msg}`)
    },
  })
  const toggleCerberusMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      await updateSetting('security.cerberus.enabled', enabled ? 'true' : 'false', 'security', 'bool')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      queryClient.invalidateQueries({ queryKey: ['security-status'] })
    },
  })

  if (isLoading) {
    return <div className="p-8 text-center">Loading security status...</div>
  }

  if (!status) {
    return <div className="p-8 text-center text-red-500">Failed to load security status</div>
  }

  const allDisabled = !status?.crowdsec?.enabled && !status?.waf?.enabled && !status?.rate_limit?.enabled && !status?.acl?.enabled

  // Replace the previous early-return that instructed enabling via env vars.
  // If allDisabled, show a banner and continue to render the dashboard with disabled controls.
  const headerBanner = allDisabled ? (
    <div className="flex flex-col items-center justify-center text-center space-y-4 p-6 bg-gray-900/5 dark:bg-gray-800 rounded-lg">
      <div className="flex items-center gap-3">
        <Shield className="w-8 h-8 text-gray-400" />
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Security Suite Disabled</h2>
      </div>
      <p className="text-sm text-gray-500 dark:text-gray-400 max-w-lg">
        Charon supports advanced security features (CrowdSec, WAF, ACLs, Rate Limiting). Enable the global Cerberus toggle in System Settings and activate individual services below.
      </p>
      <Button
        variant="primary"
        onClick={() => window.open('https://wikid82.github.io/charon/security', '_blank')}
        className="flex items-center gap-2"
      >
        <ExternalLink className="w-4 h-4" />
        Documentation
      </Button>
    </div>
  ) : null



  return (
    <div className="space-y-6">
      {headerBanner}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
          <ShieldCheck className="w-8 h-8 text-green-500" />
          Security Dashboard
        </h1>
          <div className="flex items-center gap-3">
            <label className="text-sm text-gray-500 dark:text-gray-400">Enable Cerberus</label>
            <Switch
              checked={status?.cerberus?.enabled ?? false}
              onChange={(e) => toggleCerberusMutation.mutate(e.target.checked)}
              data-testid="toggle-cerberus"
            />
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="primary"
              size="sm"
              onClick={() => {
                // enable all services
                const keys = [
                  'security.crowdsec.enabled',
                  'security.waf.enabled',
                  'security.acl.enabled',
                  'security.rate_limit.enabled',
                ]
                keys.forEach(k => toggleServiceMutation.mutate({ key: k, enabled: true }))
              }}
              data-testid="enable-all-btn"
            >
              Enable All
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                const keys = [
                  'security.crowdsec.enabled',
                  'security.waf.enabled',
                  'security.acl.enabled',
                  'security.rate_limit.enabled',
                ]
                keys.forEach(k => toggleServiceMutation.mutate({ key: k, enabled: false }))
              }}
              data-testid="disable-all-btn"
            >
              Disable All
            </Button>
          </div>
        <Button
          variant="secondary"
          onClick={() => window.open('https://wikid82.github.io/charon/security', '_blank')}
          className="flex items-center gap-2"
        >
          <ExternalLink className="w-4 h-4" />
          Documentation
        </Button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {/* CrowdSec */}
        <Card className={status.crowdsec.enabled ? 'border-green-200 dark:border-green-900' : ''}>
          <div className="flex flex-row items-center justify-between pb-2">
            <h3 className="text-sm font-medium text-white">CrowdSec</h3>
            <div className="flex items-center gap-3">
              <Switch
                checked={status.crowdsec.enabled}
                disabled={!status.cerberus?.enabled}
                onChange={(e) => {
                  // pre-validate if enabling external CrowdSec without API URL
                  if (e.target.checked && status.crowdsec?.mode === 'external') {
                      toast.error('External CrowdSec mode is not supported in this release')
                      return
                    }
                  toggleServiceMutation.mutate({ key: 'security.crowdsec.enabled', enabled: e.target.checked })
                }}
                data-testid="toggle-crowdsec"
              />
              <ShieldAlert className={`w-4 h-4 ${status.crowdsec.enabled ? 'text-green-500' : 'text-gray-400'}`} />
            </div>
          </div>
          <div>
            <div className="text-2xl font-bold mb-1 text-white">
              {status.crowdsec.enabled ? 'Active' : 'Disabled'}
            </div>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              {status.crowdsec.enabled
                ? `Mode: ${status.crowdsec.mode}`
                : 'Intrusion Prevention System'}
            </p>
            {status.crowdsec.enabled && (
              <div className="mt-4 flex gap-2">
                  <Button
                    variant="secondary"
                    size="sm"
                    className="w-full"
                    onClick={() => navigate('/tasks/logs?search=crowdsec')}
                  >
                    View Logs
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    className="w-full"
                    onClick={async () => {
                      // download config
                      try {
                        const resp = await exportCrowdsecConfig()
                        const url = window.URL.createObjectURL(new Blob([resp]))
                        const a = document.createElement('a')
                        a.href = url
                        a.download = `crowdsec-config-${new Date().toISOString().slice(0,19).replace(/[:T]/g, '-')}.tar.gz`
                        document.body.appendChild(a)
                        a.click()
                        a.remove()
                        window.URL.revokeObjectURL(url)
                        toast.success('CrowdSec configuration exported')
                      } catch {
                        toast.error('Failed to export CrowdSec configuration')
                      }
                    }}
                  >
                    Export
                  </Button>
                  <Button variant="secondary" size="sm" className="w-full" onClick={() => navigate('/settings/crowdsec')}>
                    Configure
                  </Button>
                </div>
            )}
          </div>
        </Card>

        {/* WAF */}
        <Card className={status.waf.enabled ? 'border-green-200 dark:border-green-900' : ''}>
          <div className="flex flex-row items-center justify-between pb-2">
            <h3 className="text-sm font-medium text-white">WAF (Coraza)</h3>
            <div className="flex items-center gap-3">
              <Switch
                checked={status.waf.enabled}
                disabled={!status.cerberus?.enabled}
                onChange={(e) => toggleServiceMutation.mutate({ key: 'security.waf.enabled', enabled: e.target.checked })}
                data-testid="toggle-waf"
              />
              <Shield className={`w-4 h-4 ${status.waf.enabled ? 'text-green-500' : 'text-gray-400'}`} />
            </div>
          </div>
          <div>
            <div className="text-2xl font-bold mb-1 text-white">
              {status.waf.enabled ? 'Active' : 'Disabled'}
            </div>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              OWASP Core Rule Set
            </p>
          </div>
        </Card>

        {/* ACL */}
        <Card className={status.acl.enabled ? 'border-green-200 dark:border-green-900' : ''}>
          <div className="flex flex-row items-center justify-between pb-2">
            <h3 className="text-sm font-medium text-white">Access Control</h3>
            <div className="flex items-center gap-3">
              <Switch
                checked={status.acl.enabled}
                disabled={!status.cerberus?.enabled}
                onChange={(e) => toggleServiceMutation.mutate({ key: 'security.acl.enabled', enabled: e.target.checked })}
                data-testid="toggle-acl"
              />
              <Lock className={`w-4 h-4 ${status.acl.enabled ? 'text-green-500' : 'text-gray-400'}`} />
            </div>
          </div>
          <div>
            <div className="text-2xl font-bold mb-1 text-white">
              {status.acl.enabled ? 'Active' : 'Disabled'}
            </div>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              IP-based Allow/Deny Lists
            </p>
            {status.acl.enabled && (
              <div className="mt-4">
                <Button
                  variant="secondary"
                  size="sm"
                  className="w-full"
                  onClick={() => navigate('/access-lists')}
                >
                  Manage Lists
                </Button>
              </div>
            )}
            {!status.acl.enabled && (
              <div className="mt-4">
                <Button size="sm" variant="secondary" onClick={() => navigate('/access-lists')}>Configure</Button>
              </div>
            )}
          </div>
        </Card>

        {/* Rate Limiting */}
        <Card className={status.rate_limit.enabled ? 'border-green-200 dark:border-green-900' : ''}>
          <div className="flex flex-row items-center justify-between pb-2">
            <h3 className="text-sm font-medium text-white">Rate Limiting</h3>
            <div className="flex items-center gap-3">
              <Switch
                checked={status.rate_limit.enabled}
                disabled={!status.cerberus?.enabled}
                onChange={(e) => toggleServiceMutation.mutate({ key: 'security.rate_limit.enabled', enabled: e.target.checked })}
                data-testid="toggle-rate-limit"
              />
              <Activity className={`w-4 h-4 ${status.rate_limit.enabled ? 'text-green-500' : 'text-gray-400'}`} />
            </div>
          </div>
          <div>
            <div className="text-2xl font-bold mb-1 text-white">
              {status.rate_limit.enabled ? 'Active' : 'Disabled'}
            </div>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              DDoS Protection
            </p>
            {status.rate_limit.enabled && (
              <div className="mt-4">
                <Button variant="secondary" size="sm" className="w-full">
                  Configure Limits
                </Button>
              </div>
            )}
            {!status.rate_limit.enabled && (
              <div className="mt-4">
                <Button variant="secondary" size="sm" onClick={() => navigate('/settings/system')}>Configure</Button>
              </div>
            )}
          </div>
        </Card>
      </div>
    </div>
  )
}
