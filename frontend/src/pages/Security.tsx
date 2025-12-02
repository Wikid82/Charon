import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, useEffect } from 'react'
import { useNavigate, Outlet } from 'react-router-dom'
import { Shield, ShieldAlert, ShieldCheck, Lock, Activity, ExternalLink } from 'lucide-react'
import { getSecurityStatus } from '../api/security'
import { useSecurityConfig, useUpdateSecurityConfig, useGenerateBreakGlassToken } from '../hooks/useSecurity'
import { exportCrowdsecConfig, startCrowdsec, stopCrowdsec, statusCrowdsec } from '../api/crowdsec'
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
  const { data: securityConfig } = useSecurityConfig()
  const [adminWhitelist, setAdminWhitelist] = useState<string>('')
  useEffect(() => {
    if (securityConfig && securityConfig.config) {
      setAdminWhitelist(securityConfig.config.admin_whitelist || '')
    }
  }, [securityConfig])
  const updateSecurityConfigMutation = useUpdateSecurityConfig()
  const generateBreakGlassMutation = useGenerateBreakGlassToken()
  const queryClient = useQueryClient()
  const [crowdsecStatus, setCrowdsecStatus] = useState<{ running: boolean; pid?: number } | null>(null)
  // Generic toggle mutation for per-service settings
  const toggleServiceMutation = useMutation({
    mutationFn: async ({ key, enabled }: { key: string; enabled: boolean }) => {
      await updateSetting(key, enabled ? 'true' : 'false', 'security', 'bool')
    },
    onMutate: async ({ key, enabled }: { key: string; enabled: boolean }) => {
      await queryClient.cancelQueries({ queryKey: ['security-status'] })
      const previous = queryClient.getQueryData(['security-status'])
      queryClient.setQueryData(['security-status'], (old: any) => {
        if (!old) return old
        const parts = key.split('.')
        const section = parts[1]
        const field = parts[2]
        const copy = { ...old }
        if (copy[section]) {
          copy[section] = { ...copy[section], [field]: enabled }
        }
        return copy
      })
      return { previous }
    },
    onError: (_err, _vars, context: any) => {
      if (context?.previous) queryClient.setQueryData(['security-status'], context.previous)
      const msg = _err instanceof Error ? _err.message : String(_err)
      toast.error(`Failed to update setting: ${msg}`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      queryClient.invalidateQueries({ queryKey: ['security-status'] })
      toast.success('Security setting updated')
    },

  })
  const toggleCerberusMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      await updateSetting('security.cerberus.enabled', enabled ? 'true' : 'false', 'security', 'bool')
    },
    onMutate: async (enabled: boolean) => {
      await queryClient.cancelQueries({ queryKey: ['security-status'] })
      const previous = queryClient.getQueryData(['security-status'])
      if (previous) {
        queryClient.setQueryData(['security-status'], (old: any) => {
          const copy = JSON.parse(JSON.stringify(old))
          if (!copy.cerberus) copy.cerberus = {}
          copy.cerberus.enabled = enabled
          return copy
        })
      }
      return { previous }
    },
    onError: (_err, _vars, context: any) => {
      if (context?.previous) queryClient.setQueryData(['security-status'], context.previous)
    },
    // onSuccess: already set below
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      queryClient.invalidateQueries({ queryKey: ['security-status'] })
    },
  })

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

  if (isLoading) {
    return <div className="p-8 text-center">Loading security status...</div>
  }

  if (!status) {
    return <div className="p-8 text-center text-red-500">Failed to load security status</div>
  }

  // const suiteDisabled = !(status?.cerberus?.enabled ?? false)

  // Replace the previous early-return that instructed enabling via env vars.
  // If allDisabled, show a banner and continue to render the dashboard with disabled controls.
  const headerBanner = (!status.cerberus?.enabled) ? (
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
          <div/>
        <Button
          variant="secondary"
          onClick={() => window.open('https://wikid82.github.io/charon/security', '_blank')}
          className="flex items-center gap-2"
        >
          <ExternalLink className="w-4 h-4" />
          Documentation
        </Button>
      </div>

      <div className="mt-4 p-4 bg-gray-800 rounded-lg">
        <label className="text-sm text-gray-400">Admin whitelist (comma-separated CIDR/IPs)</label>
        <div className="flex gap-2 mt-2">
          <input className="flex-1 p-2 rounded bg-gray-700 text-white" value={adminWhitelist} onChange={(e) => setAdminWhitelist(e.target.value)} />
          <Button size="sm" variant="primary" onClick={() => updateSecurityConfigMutation.mutate({ name: 'default', admin_whitelist: adminWhitelist })}>Save</Button>
          <Button size="sm" variant="secondary" onClick={() => generateBreakGlassMutation.mutate()}>Generate Token</Button>
        </div>
      </div>

      <Outlet />
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
                  console.log('crowdsec onChange', e.target.checked)
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
            {crowdsecStatus && (
              <p className="text-xs text-gray-500 dark:text-gray-400">{crowdsecStatus.running ? `Running (pid ${crowdsecStatus.pid})` : 'Stopped'}</p>
            )}
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
                  <Button variant="secondary" size="sm" className="w-full" onClick={() => navigate('/security/crowdsec')}>
                    Configure
                  </Button>
                  <div className="flex gap-2 w-full">
                    <Button
                      variant="primary"
                      size="sm"
                      className="w-full"
                      onClick={() => startMutation.mutate()}
                      data-testid="crowdsec-start"
                      isLoading={startMutation.isPending}
                      disabled={!!crowdsecStatus?.running}
                    >

                      Start
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      className="w-full"
                      onClick={() => stopMutation.mutate()}
                      data-testid="crowdsec-stop"
                      isLoading={stopMutation.isPending}
                      disabled={!crowdsecStatus?.running}
                    >

                      Stop
                    </Button>
                  </div>
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
            <div className="mt-4">
              <Button
                variant="secondary"
                size="sm"
                className="w-full"
                onClick={() => navigate('/security/waf')}
              >
                Configure
              </Button>
            </div>
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
                  onClick={() => navigate('/security/access-lists')}
                >
                  Manage Lists
                </Button>
              </div>
            )}
            {!status.acl.enabled && (
              <div className="mt-4">
                <Button size="sm" variant="secondary" onClick={() => navigate('/security/access-lists')}>Configure</Button>
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
                <Button variant="secondary" size="sm" className="w-full" onClick={() => navigate('/security/rate-limiting')}>
                  Configure Limits
                </Button>
              </div>
            )}
            {!status.rate_limit.enabled && (
              <div className="mt-4">
                <Button variant="secondary" size="sm" onClick={() => navigate('/security/rate-limiting')}>Configure</Button>
              </div>
            )}
          </div>
        </Card>
      </div>
    </div>
  )
}
