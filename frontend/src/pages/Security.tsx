import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, useEffect } from 'react'
import { useNavigate, Outlet } from 'react-router-dom'
import { Shield, ShieldAlert, ShieldCheck, Lock, Activity, ExternalLink } from 'lucide-react'
import { getSecurityStatus, type SecurityStatus } from '../api/security'
import { useSecurityConfig, useUpdateSecurityConfig, useGenerateBreakGlassToken } from '../hooks/useSecurity'
import { startCrowdsec, stopCrowdsec, statusCrowdsec } from '../api/crowdsec'
import { updateSetting } from '../api/settings'
import { Switch } from '../components/ui/Switch'
import { toast } from '../utils/toast'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { ConfigReloadOverlay } from '../components/LoadingStates'
import { LiveLogViewer } from '../components/LiveLogViewer'
import { SecurityNotificationSettingsModal } from '../components/SecurityNotificationSettingsModal'

export default function Security() {
  const navigate = useNavigate()
  const { data: status, isLoading } = useQuery({
    queryKey: ['security-status'],
    queryFn: getSecurityStatus,
  })
  const { data: securityConfig } = useSecurityConfig()
  const [adminWhitelist, setAdminWhitelist] = useState<string>('')
  const [showNotificationSettings, setShowNotificationSettings] = useState(false)
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
      queryClient.setQueryData(['security-status'], (old: unknown) => {
        if (!old || typeof old !== 'object') return old
        const parts = key.split('.')
        const section = parts[1] as keyof SecurityStatus
        const field = parts[2]
        const copy = { ...(old as SecurityStatus) }
        if (copy[section] && typeof copy[section] === 'object') {
          copy[section] = { ...copy[section], [field]: enabled } as never
        }
        return copy
      })
      return { previous }
    },
    onError: (_err, _vars, context: unknown) => {
      if (context && typeof context === 'object' && 'previous' in context) {
        queryClient.setQueryData(['security-status'], context.previous)
      }
      const msg = _err instanceof Error ? _err.message : String(_err)
      toast.error(`Failed to update setting: ${msg}`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      queryClient.invalidateQueries({ queryKey: ['security-status'] })
      toast.success('Security setting updated')
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



  const crowdsecPowerMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      await updateSetting('security.crowdsec.enabled', enabled ? 'true' : 'false', 'security', 'bool')
      if (enabled) {
        toast.info('Starting CrowdSec... This may take up to 30 seconds')
        const result = await startCrowdsec()
        return result
      } else {
        await stopCrowdsec()
        return { enabled: false }
      }
    },
    onMutate: async (enabled: boolean) => {
      await queryClient.cancelQueries({ queryKey: ['security-status'] })
      const previous = queryClient.getQueryData(['security-status'])
      queryClient.setQueryData(['security-status'], (old: unknown) => {
        if (!old || typeof old !== 'object') return old
        const copy = { ...(old as SecurityStatus) }
        if (copy.crowdsec && typeof copy.crowdsec === 'object') {
          copy.crowdsec = { ...copy.crowdsec, enabled } as never
        }
        return copy
      })
      setCrowdsecStatus(prev => prev ? { ...prev, running: enabled } : prev)
      return { previous }
    },
    onError: (err: unknown, enabled: boolean, context: unknown) => {
      if (context && typeof context === 'object' && 'previous' in context) {
        queryClient.setQueryData(['security-status'], context.previous)
      }
      const msg = err instanceof Error ? err.message : String(err)
      toast.error(enabled ? `Failed to start CrowdSec: ${msg}` : `Failed to stop CrowdSec: ${msg}`)
      fetchCrowdsecStatus()
    },
    onSuccess: async (result: { lapi_ready?: boolean; enabled?: boolean } | boolean) => {
      await fetchCrowdsecStatus()
      queryClient.invalidateQueries({ queryKey: ['security-status'] })
      queryClient.invalidateQueries({ queryKey: ['settings'] })

      if (typeof result === 'object' && result.lapi_ready === true) {
        toast.success('CrowdSec started and LAPI is ready')
      } else if (typeof result === 'object' && result.lapi_ready === false) {
        toast.warning('CrowdSec started but LAPI is still initializing. Please wait before enrolling.')
      } else if (typeof result === 'object' && result.enabled === false) {
        toast.success('CrowdSec stopped')
      } else {
        toast.success('CrowdSec started')
      }
    },
  })

  // Determine if any security operation is in progress
  const isApplyingConfig =
    toggleServiceMutation.isPending ||
    updateSecurityConfigMutation.isPending ||
    generateBreakGlassMutation.isPending ||
    crowdsecPowerMutation.isPending

  // Determine contextual message
  const getMessage = () => {
    if (toggleServiceMutation.isPending) {
      return { message: 'Three heads turn...', submessage: 'Cerberus configuration updating' }
    }
    if (crowdsecPowerMutation.isPending) {
      return crowdsecPowerMutation.variables
        ? { message: 'Summoning the guardian...', submessage: 'CrowdSec is starting' }
        : { message: 'Guardian rests...', submessage: 'CrowdSec is stopping' }
    }
    return { message: 'Strengthening the guard...', submessage: 'Protective wards activating' }
  }

  const { message, submessage } = getMessage()

  if (isLoading) {
    return <div className="p-8 text-center">Loading security status...</div>
  }

  if (!status) {
    return <div className="p-8 text-center text-red-500">Failed to load security status</div>
  }

  const cerberusDisabled = !status.cerberus?.enabled
  const crowdsecToggleDisabled = cerberusDisabled || crowdsecPowerMutation.isPending
  const crowdsecControlsDisabled = cerberusDisabled || crowdsecPowerMutation.isPending

  // const suiteDisabled = !(status?.cerberus?.enabled ?? false)

  // Replace the previous early-return that instructed enabling via env vars.
  // If allDisabled, show a banner and continue to render the dashboard with disabled controls.
  const headerBanner = (!status.cerberus?.enabled) ? (
    <div className="flex flex-col items-center justify-center text-center space-y-4 p-6 bg-gray-900/5 dark:bg-gray-800 rounded-lg">
      <div className="flex items-center gap-3">
        <Shield className="w-8 h-8 text-gray-400" />
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Cerberus Disabled</h2>
      </div>
      <p className="text-sm text-gray-500 dark:text-gray-400 max-w-lg">
        Cerberus powers CrowdSec, Coraza, ACLs, and Rate Limiting. Enable the Cerberus toggle in System Settings to awaken the guardian, then configure each head below.
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
    <>
      {isApplyingConfig && (
        <ConfigReloadOverlay
          message={message}
          submessage={submessage}
          type="cerberus"
        />
      )}
      <div className="space-y-6">
        {headerBanner}
        <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
          <ShieldCheck className="w-8 h-8 text-green-500" />
          Cerberus Dashboard
        </h1>
          <div className="flex items-center gap-2">
            <Button
              variant="secondary"
              onClick={() => setShowNotificationSettings(true)}
              disabled={!status.cerberus?.enabled}
            >
              Notification Settings
            </Button>
        <Button
          variant="secondary"
          onClick={() => window.open('https://wikid82.github.io/charon/security', '_blank')}
          className="flex items-center gap-2"
        >
          <ExternalLink className="w-4 h-4" />
          Documentation
        </Button>
          </div>
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
        {/* CrowdSec - Layer 1: IP Reputation (first line of defense) */}
        <Card className={status.crowdsec.enabled ? 'border-green-200 dark:border-green-900' : ''}>
          <div className="text-xs text-gray-400 mb-2">🛡️ Layer 1: IP Reputation</div>
          <div className="flex flex-row items-center justify-between pb-2">
            <h3 className="text-sm font-medium text-white">CrowdSec</h3>
            <div className="flex items-center gap-3">
              <Switch
                checked={status.crowdsec.enabled}
                disabled={crowdsecToggleDisabled}
                onChange={(e) => {
                  crowdsecPowerMutation.mutate(e.target.checked)
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
                ? `Protects against: Known attackers, botnets, brute-force`
                : 'Intrusion Prevention System'}
            </p>
            {crowdsecStatus && (
              <p className="text-xs text-gray-500 dark:text-gray-400">{crowdsecStatus.running ? `Running (pid ${crowdsecStatus.pid})` : 'Stopped'}</p>
            )}
            <div className="mt-4">
              <Button
                variant="secondary"
                size="sm"
                className="w-full text-xs"
                onClick={() => navigate('/security/crowdsec')}
                disabled={crowdsecControlsDisabled}
              >
                Config
              </Button>
            </div>
          </div>
        </Card>

        {/* ACL - Layer 2: Access Control (IP/Geo filtering) */}
        <Card className={status.acl.enabled ? 'border-green-200 dark:border-green-900' : ''}>
          <div className="text-xs text-gray-400 mb-2">🔒 Layer 2: Access Control</div>
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
              Protects against: Unauthorized IPs, geo-based attacks, insider threats
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

        {/* Coraza - Layer 3: Request Inspection */}
        <Card className={status.waf.enabled ? 'border-green-200 dark:border-green-900' : ''}>
          <div className="text-xs text-gray-400 mb-2">🛡️ Layer 3: Request Inspection</div>
          <div className="flex flex-row items-center justify-between pb-2">
            <h3 className="text-sm font-medium text-white">Coraza</h3>
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
              {status.waf.enabled
                ? `Protects against: SQL injection, XSS, RCE, zero-day exploits*`
                : 'Web Application Firewall'}
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

        {/* Rate Limiting - Layer 4: Volume Control */}
        <Card className={status.rate_limit.enabled ? 'border-green-200 dark:border-green-900' : ''}>
          <div className="text-xs text-gray-400 mb-2">⚡ Layer 4: Volume Control</div>
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
            <div className="flex items-center gap-2 mb-1">
              <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                status.rate_limit.enabled
                  ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                  : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
              }`}>
                {status.rate_limit.enabled ? '● Active' : '○ Disabled'}
              </span>
            </div>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Protects against: DDoS attacks, credential stuffing, API abuse
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

      {/* Live Activity Section */}
      {status.cerberus?.enabled && (
        <div className="mt-6">
          <LiveLogViewer mode="security" securityFilters={{}} className="w-full" />
        </div>
      )}

      {/* Notification Settings Modal */}
      <SecurityNotificationSettingsModal
        isOpen={showNotificationSettings}
        onClose={() => setShowNotificationSettings(false)}
      />
      </div>
    </>
  )
}
