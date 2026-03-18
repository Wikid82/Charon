import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Shield, ShieldAlert, ShieldCheck, Lock, Activity, ExternalLink, Bell } from 'lucide-react'
import { useState, useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, Outlet } from 'react-router-dom'

import { startCrowdsec, stopCrowdsec, statusCrowdsec } from '../api/crowdsec'
import { getSecurityStatus, type SecurityStatus } from '../api/security'
import { updateSetting } from '../api/settings'
import { CrowdSecKeyWarning } from '../components/CrowdSecKeyWarning'
import { PageShell } from '../components/layout/PageShell'
import { LiveLogViewer } from '../components/LiveLogViewer'
import { ConfigReloadOverlay } from '../components/LoadingStates'
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
import { useSecurityConfig, useUpdateSecurityConfig, useGenerateBreakGlassToken } from '../hooks/useSecurity'
import { toast } from '../utils/toast'

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
function SecurityPageSkeleton({ t }: { t: (key: string) => string }) {
  return (
    <PageShell
      title={t('security.title')}
      description={t('security.description')}
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
  const { t } = useTranslation()
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
  // Stable reference to prevent WebSocket reconnection loops in LiveLogViewer
  const emptySecurityFilters = useMemo(() => ({}), [])
  // Generic toggle mutation for per-service settings
  const toggleServiceMutation = useMutation({
    mutationFn: async ({ key, enabled }: { key: string; enabled: boolean }) => {
      await updateSetting(key, enabled ? 'true' : 'false', 'security', 'bool')
    },
    onMutate: async ({ key, enabled }: { key: string; enabled: boolean }) => {
      // Cancel ongoing queries to avoid race conditions
      await queryClient.cancelQueries({ queryKey: ['security-status'] })

      // Snapshot current state for rollback
      const previous = queryClient.getQueryData(['security-status'])

      // Optimistic update: parse key like "security.acl.enabled" -> section "acl"
      queryClient.setQueryData(['security-status'], (old: unknown) => {
        if (!old || typeof old !== 'object') return old

        const oldStatus = old as SecurityStatus
        const copy = { ...oldStatus }

        // Extract section from key (e.g., "security.acl.enabled" -> "acl")
        const parts = key.split('.')
        const section = parts[1]

        // CRITICAL: Spread existing section data to preserve fields like 'mode'
        // Update ONLY the enabled field, keep everything else intact
        if (section === 'acl') {
          copy.acl = { ...copy.acl, enabled }
        } else if (section === 'waf') {
          // Preserve mode field (detection/prevention)
          copy.waf = { ...copy.waf, enabled }
        } else if (section === 'rate_limit') {
          // Preserve mode field (log/block)
          copy.rate_limit = { ...copy.rate_limit, enabled }
        }

        return copy
      })

      return { previous }
    },
    onError: (_err, _vars, context: unknown) => {
      // Rollback on error
      if (context && typeof context === 'object' && 'previous' in context) {
        queryClient.setQueryData(['security-status'], context.previous)
      }
      const msg = _err instanceof Error ? _err.message : String(_err)
      toast.error(`Failed to update setting: ${msg}`)
    },
    onSuccess: () => {
      // Refresh data from server
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
    onMutate: async (enabled: boolean) => {
      if (enabled) {
        queryClient.setQueryData(['crowdsec-starting'], { isStarting: true, startedAt: Date.now() })
      }
    },
    onError: (err: unknown, enabled: boolean) => {
      queryClient.setQueryData(['crowdsec-starting'], { isStarting: false })
      const msg = err instanceof Error ? err.message : String(err)
      toast.error(enabled ? `Failed to start CrowdSec: ${msg}` : `Failed to stop CrowdSec: ${msg}`)
      // Force refresh status from backend to ensure UI matches reality
      queryClient.invalidateQueries({ queryKey: ['security-status'] })
      fetchCrowdsecStatus()
    },
    onSuccess: async (result: { lapi_ready?: boolean; enabled?: boolean } | boolean) => {
      queryClient.setQueryData(['crowdsec-starting'], { isStarting: false })
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
      return { message: t('security.threeHeadsTurn'), submessage: t('security.cerberusConfigUpdating') }
    }
    if (crowdsecPowerMutation.isPending) {
      return crowdsecPowerMutation.variables
        ? { message: t('security.summoningGuardian'), submessage: t('security.crowdsecStarting') }
        : { message: t('security.guardianRests'), submessage: t('security.crowdsecStopping') }
    }
    return { message: t('security.strengtheningGuard'), submessage: t('security.wardsActivating') }
  }

  const { message, submessage } = getMessage()

  if (isLoading) {
    return <SecurityPageSkeleton t={t} />
  }

  if (!status) {
    return (
      <PageShell
        title={t('security.title')}
        description={t('security.description')}
      >
        <Alert variant="error" title={t('common.error')}>
          {t('security.failedToLoadConfiguration')}
        </Alert>
      </PageShell>
    )
  }

  // During the crowdsecPowerMutation, use the mutation's argument as the authoritative
  // checked state. Neither crowdsecStatus (local, stale) nor status.crowdsec.enabled
  // (server, not yet invalidated) reflects the user's intent until onSuccess fires.
  const crowdsecChecked = crowdsecPowerMutation.isPending
    ? (crowdsecPowerMutation.variables ?? (crowdsecStatus?.running ?? status.crowdsec.enabled))
    : (crowdsecStatus?.running ?? status.crowdsec.enabled)

  const cerberusDisabled = !status.cerberus?.enabled
  const crowdsecToggleDisabled = cerberusDisabled || crowdsecPowerMutation.isPending
  const crowdsecControlsDisabled = cerberusDisabled || crowdsecPowerMutation.isPending

  // Header actions
  const headerActions = (
    <div className="flex items-center gap-2">
      <Button
        variant="secondary"
        onClick={() => navigate('/security/audit-logs')}
      >
        <Activity className="w-4 h-4 mr-2" />
        {t('security.auditLogs')}
      </Button>
      <Button
        variant="secondary"
        onClick={() => navigate('/settings/notifications')}
      >
        <Bell className="w-4 h-4 mr-2" />
        {t('security.notifications')}
      </Button>
      <Button
        variant="secondary"
        onClick={() => window.open('https://wikid82.github.io/charon/security', '_blank')}
      >
        <ExternalLink className="w-4 h-4 mr-2" />
        {t('common.docs')}
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
        title={t('security.title')}
        description={t('security.description')}
        actions={headerActions}
      >
        {/* Cerberus Status Header */}
        <Card className="flex items-center gap-4 p-6">
          <div className={`p-3 rounded-lg ${status.cerberus?.enabled ? 'bg-success/10' : 'bg-surface-muted'}`}>
            <ShieldCheck className={`w-8 h-8 ${status.cerberus?.enabled ? 'text-success' : 'text-content-muted'}`} />
          </div>
          <div className="flex-1">
            <div className="flex items-center gap-3">
              <h2 className="text-xl font-semibold text-content-primary">{t('security.cerberusDashboard')}</h2>
              <Badge variant={status.cerberus?.enabled ? 'success' : 'default'}>
                {status.cerberus?.enabled ? t('security.cerberusActive') : t('security.cerberusDisabled')}
              </Badge>
            </div>
            <p className="text-sm text-content-secondary mt-1">
              {status.cerberus?.enabled
                ? t('security.cerberusReadyMessage')
                : t('security.cerberusDisabledMessage')}
            </p>
          </div>
        </Card>

        {/* Cerberus Disabled Alert */}
        {!status.cerberus?.enabled && (
          <Alert variant="warning" title={t('security.featuresUnavailable')}>
            <div className="space-y-2">
              <p>
                {t('security.featuresUnavailableMessage')}
              </p>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => window.open('https://wikid82.github.io/charon/security', '_blank')}
                className="mt-2"
              >
                <ExternalLink className="w-3 h-3 mr-1.5" />
                {t('security.learnMore')}
              </Button>
            </div>
          </Alert>
        )}

        {/* CrowdSec Key Rejection Warning — suppressed during startup to avoid flashing before bouncer registration completes */}
        {status.cerberus?.enabled && !crowdsecPowerMutation.isPending && (crowdsecStatus?.running ?? status.crowdsec.enabled) && (
          <CrowdSecKeyWarning />
        )}

        {/* Admin Whitelist Section */}
        {status.cerberus?.enabled && (
          <Card>
            <CardHeader>
              <CardTitle>{t('security.adminWhitelist')}</CardTitle>
              <CardDescription>{t('security.adminWhitelistDescription')}</CardDescription>
            </CardHeader>
            <CardContent>
              <label className="text-sm text-content-secondary">{t('security.commaSeparatedCIDR')}</label>
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
                  {t('common.save')}
                </Button>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => generateBreakGlassMutation.mutate()}
                    >
                      {t('security.generateToken')}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>{t('security.generateTokenTooltip')}</p>
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
                  <Badge variant="outline" size="sm">{t('security.layer1')}</Badge>
                  <Badge variant="primary" size="sm">{t('security.ids')}</Badge>
                </div>
                <Badge variant={crowdsecPowerMutation.isPending && crowdsecPowerMutation.variables ? 'warning' : crowdsecChecked ? 'success' : 'default'}>
                  {crowdsecPowerMutation.isPending && crowdsecPowerMutation.variables ? t('security.crowdsec.starting') : crowdsecChecked ? t('common.enabled') : t('common.disabled')}
                </Badge>
              </div>
              <div className="flex items-center gap-3 mt-3">
                <div className={`p-2 rounded-lg ${crowdsecChecked ? 'bg-success/10' : 'bg-surface-muted'}`}>
                  <ShieldAlert className={`w-5 h-5 ${crowdsecChecked ? 'text-success' : 'text-content-muted'}`} />
                </div>
                <div>
                  <CardTitle className="text-base">{t('security.crowdsec')}</CardTitle>
                  <CardDescription>{t('security.crowdsecDescription')}</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="flex-1">
              <p className="text-sm text-content-muted">
                {crowdsecChecked
                  ? t('security.crowdsecProtects')
                  : t('security.crowdsecDisabledDescription')}
              </p>
              {crowdsecStatus && (
                <p className="text-xs text-content-muted mt-2">
                  {crowdsecStatus.running ? t('security.runningPid', { pid: crowdsecStatus.pid }) : t('security.processStopped')}
                </p>
              )}
            </CardContent>
            <CardFooter className="justify-between pt-4">
              <Tooltip>
                <TooltipTrigger asChild>
                  <div>
                    <Switch
                      checked={crowdsecChecked}
                      disabled={crowdsecToggleDisabled}
                      onCheckedChange={(checked) => crowdsecPowerMutation.mutate(checked)}
                      data-testid="toggle-crowdsec"
                    />
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{cerberusDisabled ? t('security.enableCerberusFirst') : t('security.toggleCrowdsec')}</p>
                </TooltipContent>
              </Tooltip>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => navigate('/security/crowdsec')}
                disabled={crowdsecControlsDisabled}
              >
                {t('common.configure')}
              </Button>
            </CardFooter>
          </Card>

          {/* ACL - Layer 2: Access Control */}
          <Card variant="interactive" className="flex flex-col">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Badge variant="outline" size="sm">{t('security.layer2')}</Badge>
                  <Badge variant="primary" size="sm">{t('security.acl')}</Badge>
                </div>
                <Badge variant={status.acl.enabled ? 'success' : 'default'}>
                  {status.acl.enabled ? t('common.enabled') : t('common.disabled')}
                </Badge>
              </div>
              <div className="flex items-center gap-3 mt-3">
                <div className={`p-2 rounded-lg ${status.acl.enabled ? 'bg-success/10' : 'bg-surface-muted'}`}>
                  <Lock className={`w-5 h-5 ${status.acl.enabled ? 'text-success' : 'text-content-muted'}`} />
                </div>
                <div>
                  <CardTitle className="text-base">{t('security.accessControl')}</CardTitle>
                  <CardDescription>{t('security.aclDescription')}</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="flex-1">
              <p className="text-sm text-content-muted">
                {t('security.aclProtects')}
              </p>
            </CardContent>
            <CardFooter className="justify-between pt-4">
              <Tooltip>
                <TooltipTrigger asChild>
                  <div>
                    <Switch
                      checked={status.acl.enabled}
                      disabled={!status.cerberus?.enabled}
                      onCheckedChange={(checked) => toggleServiceMutation.mutate({ key: 'security.acl.enabled', enabled: checked })}
                      data-testid="toggle-acl"
                    />
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{cerberusDisabled ? t('security.enableCerberusFirst') : t('security.toggleAcl')}</p>
                </TooltipContent>
              </Tooltip>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => navigate('/security/access-lists')}
              >
                {status.acl.enabled ? t('security.manageLists') : t('common.configure')}
              </Button>
            </CardFooter>
          </Card>

          {/* Coraza - Layer 3: Request Inspection */}
          <Card variant="interactive" className="flex flex-col">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Badge variant="outline" size="sm">{t('security.layer3')}</Badge>
                  <Badge variant="primary" size="sm">{t('security.waf')}</Badge>
                </div>
                <Badge variant={status.waf.enabled ? 'success' : 'default'}>
                  {status.waf.enabled ? t('common.enabled') : t('common.disabled')}
                </Badge>
              </div>
              <div className="flex items-center gap-3 mt-3">
                <div className={`p-2 rounded-lg ${status.waf.enabled ? 'bg-success/10' : 'bg-surface-muted'}`}>
                  <Shield className={`w-5 h-5 ${status.waf.enabled ? 'text-success' : 'text-content-muted'}`} />
                </div>
                <div>
                  <CardTitle className="text-base">{t('security.corazaWaf')}</CardTitle>
                  <CardDescription>{t('security.wafDescription')}</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="flex-1">
              <p className="text-sm text-content-muted">
                {status.waf.enabled
                  ? t('security.wafProtects')
                  : t('security.wafDisabledDescription')}
              </p>
            </CardContent>
            <CardFooter className="justify-between pt-4">
              <Tooltip>
                <TooltipTrigger asChild>
                  <div>
                    <Switch
                      checked={status.waf.enabled}
                      disabled={!status.cerberus?.enabled}
                      onCheckedChange={(checked) => toggleServiceMutation.mutate({ key: 'security.waf.enabled', enabled: checked })}
                      data-testid="toggle-waf"
                    />
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{cerberusDisabled ? t('security.enableCerberusFirst') : t('security.toggleWaf')}</p>
                </TooltipContent>
              </Tooltip>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => navigate('/security/waf')}
              >
                {t('common.configure')}
              </Button>
            </CardFooter>
          </Card>

          {/* Rate Limiting - Layer 4: Volume Control */}
          <Card variant="interactive" className="flex flex-col">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Badge variant="outline" size="sm">{t('security.layer4')}</Badge>
                  <Badge variant="primary" size="sm">{t('security.rate')}</Badge>
                </div>
                <Badge variant={status.rate_limit.enabled ? 'success' : 'default'}>
                  {status.rate_limit.enabled ? t('common.enabled') : t('common.disabled')}
                </Badge>
              </div>
              <div className="flex items-center gap-3 mt-3">
                <div className={`p-2 rounded-lg ${status.rate_limit.enabled ? 'bg-success/10' : 'bg-surface-muted'}`}>
                  <Activity className={`w-5 h-5 ${status.rate_limit.enabled ? 'text-success' : 'text-content-muted'}`} />
                </div>
                <div>
                  <CardTitle className="text-base">{t('security.rateLimiting')}</CardTitle>
                  <CardDescription>{t('security.rateLimitDescription')}</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="flex-1">
              <p className="text-sm text-content-muted">
                {t('security.rateLimitProtects')}
              </p>
            </CardContent>
            <CardFooter className="justify-between pt-4">
              <Tooltip>
                <TooltipTrigger asChild>
                  <div>
                    <Switch
                      checked={status.rate_limit.enabled}
                      disabled={!status.cerberus?.enabled}
                      onCheckedChange={(checked) => toggleServiceMutation.mutate({ key: 'security.rate_limit.enabled', enabled: checked })}
                      data-testid="toggle-rate-limit"
                    />
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{cerberusDisabled ? t('security.enableCerberusFirst') : t('security.toggleRateLimit')}</p>
                </TooltipContent>
              </Tooltip>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => navigate('/security/rate-limiting')}
              >
                {t('common.configure')}
              </Button>
            </CardFooter>
          </Card>
        </div>

        {/* Live Activity Section */}
        {status.cerberus?.enabled && (
          <LiveLogViewer mode="security" securityFilters={emptySecurityFilters} className="w-full" />
        )}

      </PageShell>
    </TooltipProvider>
  )
}
