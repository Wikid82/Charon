import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, useEffect, useMemo } from 'react'
import { useNavigate, Outlet } from 'react-router-dom'
import { Shield, ShieldAlert, ShieldCheck, Lock, Activity, ExternalLink, Settings } from 'lucide-react'
import { getSecurityStatus, type SecurityStatus } from '../api/security'
import { useSecurityConfig, useUpdateSecurityConfig, useGenerateBreakGlassToken } from '../hooks/useSecurity'
import { startCrowdsec, stopCrowdsec, statusCrowdsec } from '../api/crowdsec'
import { updateSetting } from '../api/settings'
import { toast } from '../utils/toast'
import { ConfigReloadOverlay } from '../components/LoadingStates'
import { LiveLogViewer } from '../components/LiveLogViewer'
import { SecurityNotificationSettingsModal } from '../components/SecurityNotificationSettingsModal'
import { PageShell } from '../components/layout/PageShell'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
  Button,
  Badge,
  Alert,
  Switch,
  Skeleton,
  Tooltip,
  TooltipTrigger,
  TooltipContent,
  TooltipProvider,
} from '../components/ui'

// Skeleton loader for security layer cards
function SecurityCardSkeleton() {
  return (
    <Card className="flex flex-col">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="space-y-2">
            <Skeleton className="h-5 w-24" />
            <Skeleton className="h-4 w-32" />
          </div>
          <Skeleton className="h-6 w-16 rounded-full" />
        </div>
      </CardHeader>
      <CardContent className="flex-1">
        <div className="space-y-3">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-3/4" />
        </div>
      </CardContent>
      <CardFooter className="justify-between pt-4">
        <Skeleton className="h-5 w-10" />
        <Skeleton className="h-8 w-20 rounded-md" />
      </CardFooter>
    </Card>
  )
}

// Loading skeleton for the entire security page
function SecurityPageSkeleton() {
  return (
    <PageShell
      title="Security"
      description="Configure security layers for your reverse proxy"
    >
      <Skeleton className="h-24 w-full rounded-lg" />
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <SecurityCardSkeleton />
        <SecurityCardSkeleton />
        <SecurityCardSkeleton />
        <SecurityCardSkeleton />
      </div>
    </PageShell>
  )
}

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
  // Stable reference to prevent WebSocket reconnection loops in LiveLogViewer
  const emptySecurityFilters = useMemo(() => ({}), [])
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
      // Update setting first
      await updateSetting('security.crowdsec.enabled', enabled ? 'true' : 'false', 'security', 'bool')

      if (enabled) {
        toast.info('Starting CrowdSec... This may take up to 30 seconds')
        const result = await startCrowdsec()

        // VERIFY: Check if it actually started
        const status = await statusCrowdsec()
        if (!status.running) {
          // Revert the setting since process didn't start
          await updateSetting('security.crowdsec.enabled', 'false', 'security', 'bool')
          throw new Error('CrowdSec process failed to start. Check server logs for details.')
        }

        return result
      } else {
        await stopCrowdsec()

        // VERIFY: Check if it actually stopped (with brief delay for cleanup)
        await new Promise(resolve => setTimeout(resolve, 500))
        const status = await statusCrowdsec()
        if (status.running) {
          throw new Error('CrowdSec process still running. Check server logs for details.')
        }

        return { enabled: false }
      }
    },
    // NO optimistic updates - wait for actual confirmation
    onError: (err: unknown, enabled: boolean) => {
      const msg = err instanceof Error ? err.message : String(err)
      toast.error(enabled ? `Failed to start CrowdSec: ${msg}` : `Failed to stop CrowdSec: ${msg}`)
      // Force refresh status from backend to ensure UI matches reality
      queryClient.invalidateQueries({ queryKey: ['security-status'] })
      fetchCrowdsecStatus()
    },
    onSuccess: async (result: { lapi_ready?: boolean; enabled?: boolean } | boolean) => {
      // Refresh all related queries to ensure consistency
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['security-status'] }),
        queryClient.invalidateQueries({ queryKey: ['settings'] }),
        fetchCrowdsecStatus(),
      ])

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
    return <SecurityPageSkeleton />
  }

  if (!status) {
    return (
      <PageShell
        title="Security"
        description="Configure security layers for your reverse proxy"
      >
        <Alert variant="error" title="Error Loading Security Status">
          Failed to load security configuration. Please try refreshing the page.
        </Alert>
      </PageShell>
    )
  }

  const cerberusDisabled = !status.cerberus?.enabled
  const crowdsecToggleDisabled = cerberusDisabled || crowdsecPowerMutation.isPending
  const crowdsecControlsDisabled = cerberusDisabled || crowdsecPowerMutation.isPending

  // Header actions
  const headerActions = (
    <div className="flex items-center gap-2">
      <Button
        variant="secondary"
        onClick={() => setShowNotificationSettings(true)}
        disabled={!status.cerberus?.enabled}
      >
        <Settings className="w-4 h-4 mr-2" />
        Notifications
      </Button>
      <Button
        variant="secondary"
        onClick={() => window.open('https://wikid82.github.io/charon/security', '_blank')}
      >
        <ExternalLink className="w-4 h-4 mr-2" />
        Docs
      </Button>
    </div>
  )



  return (
    <TooltipProvider>
      {isApplyingConfig && (
        <ConfigReloadOverlay
          message={message}
          submessage={submessage}
          type="cerberus"
        />
      )}
      <PageShell
        title="Security"
        description="Configure security layers for your reverse proxy"
        actions={headerActions}
      >
        {/* Cerberus Status Header */}
        <Card className="flex items-center gap-4 p-6">
          <div className={`p-3 rounded-lg ${status.cerberus?.enabled ? 'bg-success/10' : 'bg-surface-muted'}`}>
            <ShieldCheck className={`w-8 h-8 ${status.cerberus?.enabled ? 'text-success' : 'text-content-muted'}`} />
          </div>
          <div className="flex-1">
            <div className="flex items-center gap-3">
              <h2 className="text-xl font-semibold text-content-primary">Cerberus Dashboard</h2>
              <Badge variant={status.cerberus?.enabled ? 'success' : 'default'}>
                {status.cerberus?.enabled ? 'Active' : 'Disabled'}
              </Badge>
            </div>
            <p className="text-sm text-content-secondary mt-1">
              {status.cerberus?.enabled
                ? 'All security heads are ready for configuration'
                : 'Enable Cerberus in System Settings to activate security features'}
            </p>
          </div>
        </Card>

        {/* Cerberus Disabled Alert */}
        {!status.cerberus?.enabled && (
          <Alert variant="warning" title="Security Features Unavailable">
            <div className="space-y-2">
              <p>
                Cerberus powers CrowdSec, Coraza WAF, Access Control, and Rate Limiting.
                Enable the Cerberus toggle in System Settings to activate these features.
              </p>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => window.open('https://wikid82.github.io/charon/security', '_blank')}
                className="mt-2"
              >
                <ExternalLink className="w-3 h-3 mr-1.5" />
                Learn More
              </Button>
            </div>
          </Alert>
        )}

        {/* Admin Whitelist Section */}
        {status.cerberus?.enabled && (
          <Card>
            <CardHeader>
              <CardTitle>Admin Whitelist</CardTitle>
              <CardDescription>Configure IP addresses that bypass security checks</CardDescription>
            </CardHeader>
            <CardContent>
              <label className="text-sm text-content-secondary">Comma-separated CIDR/IPs</label>
              <div className="flex gap-2 mt-2">
                <input
                  className="flex-1 px-3 py-2 rounded-md border border-border bg-surface-elevated text-content-primary focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                  value={adminWhitelist}
                  onChange={(e) => setAdminWhitelist(e.target.value)}
                  placeholder="192.168.1.0/24, 10.0.0.1"
                />
                <Button
                  size="sm"
                  variant="primary"
                  onClick={() => updateSecurityConfigMutation.mutate({ name: 'default', admin_whitelist: adminWhitelist })}
                >
                  Save
                </Button>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => generateBreakGlassMutation.mutate()}
                    >
                      Generate Token
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>Generate a break-glass token for emergency access</p>
                  </TooltipContent>
                </Tooltip>
              </div>
            </CardContent>
          </Card>
        )}

        <Outlet />

        {/* Security Layer Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          {/* CrowdSec - Layer 1: IP Reputation */}
          <Card variant="interactive" className="flex flex-col">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Badge variant="outline" size="sm">Layer 1</Badge>
                  <Badge variant="primary" size="sm">IDS</Badge>
                </div>
                <Badge variant={(crowdsecStatus?.running ?? status.crowdsec.enabled) ? 'success' : 'default'}>
                  {(crowdsecStatus?.running ?? status.crowdsec.enabled) ? 'Enabled' : 'Disabled'}
                </Badge>
              </div>
              <div className="flex items-center gap-3 mt-3">
                <div className={`p-2 rounded-lg ${(crowdsecStatus?.running ?? status.crowdsec.enabled) ? 'bg-success/10' : 'bg-surface-muted'}`}>
                  <ShieldAlert className={`w-5 h-5 ${(crowdsecStatus?.running ?? status.crowdsec.enabled) ? 'text-success' : 'text-content-muted'}`} />
                </div>
                <div>
                  <CardTitle className="text-base">CrowdSec</CardTitle>
                  <CardDescription>IP Reputation & Threat Intelligence</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="flex-1">
              <p className="text-sm text-content-muted">
                {(crowdsecStatus?.running ?? status.crowdsec.enabled)
                  ? 'Protects against: Known attackers, botnets, brute-force'
                  : 'Intrusion Prevention System powered by community threat intelligence'}
              </p>
              {crowdsecStatus && (
                <p className="text-xs text-content-muted mt-2">
                  {crowdsecStatus.running ? `Running (PID ${crowdsecStatus.pid})` : 'Process stopped'}
                </p>
              )}
            </CardContent>
            <CardFooter className="justify-between pt-4">
              <Tooltip>
                <TooltipTrigger asChild>
                  <div>
                    <Switch
                      checked={crowdsecStatus?.running ?? status.crowdsec.enabled}
                      disabled={crowdsecToggleDisabled}
                      onChange={(e) => crowdsecPowerMutation.mutate(e.target.checked)}
                      data-testid="toggle-crowdsec"
                    />
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{cerberusDisabled ? 'Enable Cerberus first' : 'Toggle CrowdSec protection'}</p>
                </TooltipContent>
              </Tooltip>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => navigate('/security/crowdsec')}
                disabled={crowdsecControlsDisabled}
              >
                Configure
              </Button>
            </CardFooter>
          </Card>

          {/* ACL - Layer 2: Access Control */}
          <Card variant="interactive" className="flex flex-col">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Badge variant="outline" size="sm">Layer 2</Badge>
                  <Badge variant="primary" size="sm">ACL</Badge>
                </div>
                <Badge variant={status.acl.enabled ? 'success' : 'default'}>
                  {status.acl.enabled ? 'Enabled' : 'Disabled'}
                </Badge>
              </div>
              <div className="flex items-center gap-3 mt-3">
                <div className={`p-2 rounded-lg ${status.acl.enabled ? 'bg-success/10' : 'bg-surface-muted'}`}>
                  <Lock className={`w-5 h-5 ${status.acl.enabled ? 'text-success' : 'text-content-muted'}`} />
                </div>
                <div>
                  <CardTitle className="text-base">Access Control</CardTitle>
                  <CardDescription>IP & Geo-based filtering</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="flex-1">
              <p className="text-sm text-content-muted">
                Protects against: Unauthorized IPs, geo-based attacks, insider threats
              </p>
            </CardContent>
            <CardFooter className="justify-between pt-4">
              <Tooltip>
                <TooltipTrigger asChild>
                  <div>
                    <Switch
                      checked={status.acl.enabled}
                      disabled={!status.cerberus?.enabled}
                      onChange={(e) => toggleServiceMutation.mutate({ key: 'security.acl.enabled', enabled: e.target.checked })}
                      data-testid="toggle-acl"
                    />
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{cerberusDisabled ? 'Enable Cerberus first' : 'Toggle Access Control'}</p>
                </TooltipContent>
              </Tooltip>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => navigate('/security/access-lists')}
              >
                {status.acl.enabled ? 'Manage Lists' : 'Configure'}
              </Button>
            </CardFooter>
          </Card>

          {/* Coraza - Layer 3: Request Inspection */}
          <Card variant="interactive" className="flex flex-col">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Badge variant="outline" size="sm">Layer 3</Badge>
                  <Badge variant="primary" size="sm">WAF</Badge>
                </div>
                <Badge variant={status.waf.enabled ? 'success' : 'default'}>
                  {status.waf.enabled ? 'Enabled' : 'Disabled'}
                </Badge>
              </div>
              <div className="flex items-center gap-3 mt-3">
                <div className={`p-2 rounded-lg ${status.waf.enabled ? 'bg-success/10' : 'bg-surface-muted'}`}>
                  <Shield className={`w-5 h-5 ${status.waf.enabled ? 'text-success' : 'text-content-muted'}`} />
                </div>
                <div>
                  <CardTitle className="text-base">Coraza WAF</CardTitle>
                  <CardDescription>Request inspection & filtering</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="flex-1">
              <p className="text-sm text-content-muted">
                {status.waf.enabled
                  ? 'Protects against: SQL injection, XSS, RCE, zero-day exploits*'
                  : 'Web Application Firewall with OWASP Core Rule Set'}
              </p>
            </CardContent>
            <CardFooter className="justify-between pt-4">
              <Tooltip>
                <TooltipTrigger asChild>
                  <div>
                    <Switch
                      checked={status.waf.enabled}
                      disabled={!status.cerberus?.enabled}
                      onChange={(e) => toggleServiceMutation.mutate({ key: 'security.waf.enabled', enabled: e.target.checked })}
                      data-testid="toggle-waf"
                    />
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{cerberusDisabled ? 'Enable Cerberus first' : 'Toggle Coraza WAF'}</p>
                </TooltipContent>
              </Tooltip>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => navigate('/security/waf')}
              >
                Configure
              </Button>
            </CardFooter>
          </Card>

          {/* Rate Limiting - Layer 4: Volume Control */}
          <Card variant="interactive" className="flex flex-col">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Badge variant="outline" size="sm">Layer 4</Badge>
                  <Badge variant="primary" size="sm">Rate</Badge>
                </div>
                <Badge variant={status.rate_limit.enabled ? 'success' : 'default'}>
                  {status.rate_limit.enabled ? 'Enabled' : 'Disabled'}
                </Badge>
              </div>
              <div className="flex items-center gap-3 mt-3">
                <div className={`p-2 rounded-lg ${status.rate_limit.enabled ? 'bg-success/10' : 'bg-surface-muted'}`}>
                  <Activity className={`w-5 h-5 ${status.rate_limit.enabled ? 'text-success' : 'text-content-muted'}`} />
                </div>
                <div>
                  <CardTitle className="text-base">Rate Limiting</CardTitle>
                  <CardDescription>Request volume control</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="flex-1">
              <p className="text-sm text-content-muted">
                Protects against: DDoS attacks, credential stuffing, API abuse
              </p>
            </CardContent>
            <CardFooter className="justify-between pt-4">
              <Tooltip>
                <TooltipTrigger asChild>
                  <div>
                    <Switch
                      checked={status.rate_limit.enabled}
                      disabled={!status.cerberus?.enabled}
                      onChange={(e) => toggleServiceMutation.mutate({ key: 'security.rate_limit.enabled', enabled: e.target.checked })}
                      data-testid="toggle-rate-limit"
                    />
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{cerberusDisabled ? 'Enable Cerberus first' : 'Toggle Rate Limiting'}</p>
                </TooltipContent>
              </Tooltip>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => navigate('/security/rate-limiting')}
              >
                Configure
              </Button>
            </CardFooter>
          </Card>
        </div>

        {/* Live Activity Section */}
        {status.cerberus?.enabled && (
          <LiveLogViewer mode="security" securityFilters={emptySecurityFilters} className="w-full" />
        )}

        {/* Notification Settings Modal */}
        <SecurityNotificationSettingsModal
          isOpen={showNotificationSettings}
          onClose={() => setShowNotificationSettings(false)}
        />
      </PageShell>
    </TooltipProvider>
  )
}
