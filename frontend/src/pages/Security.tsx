import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Shield, ShieldAlert, ShieldCheck, Lock, Activity, ExternalLink } from 'lucide-react'
import { getSecurityStatus } from '../api/security'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'

export default function Security() {
  const navigate = useNavigate()
  const { data: status, isLoading } = useQuery({
    queryKey: ['security-status'],
    queryFn: getSecurityStatus,
  })

  if (isLoading) {
    return <div className="p-8 text-center">Loading security status...</div>
  }

  if (!status) {
    return <div className="p-8 text-center text-red-500">Failed to load security status</div>
  }

  const allDisabled = !status.crowdsec.enabled && !status.waf.enabled && !status.rate_limit.enabled && !status.acl.enabled

  if (allDisabled) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] text-center space-y-6">
        <div className="bg-gray-100 dark:bg-gray-800 p-6 rounded-full">
          <Shield className="w-16 h-16 text-gray-400" />
        </div>
        <div className="max-w-md space-y-2">
          <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Security Services Not Enabled</h2>
          <p className="text-gray-500 dark:text-gray-400">
            CaddyProxyManager+ supports advanced security features like CrowdSec, WAF, ACLs, and Rate Limiting.
            These are optional and can be enabled via environment variables.
          </p>
        </div>
        <Button
          variant="primary"
          onClick={() => window.open('https://wikid82.github.io/cpmp/docs/security.html', '_blank')}
          className="flex items-center gap-2"
        >
          <ExternalLink className="w-4 h-4" />
          View Implementation Guide
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
          <ShieldCheck className="w-8 h-8 text-green-500" />
          Security Dashboard
        </h1>
        <Button
          variant="secondary"
          onClick={() => window.open('https://wikid82.github.io/cpmp/docs/security.html', '_blank')}
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
            <ShieldAlert className={`w-4 h-4 ${status.crowdsec.enabled ? 'text-green-500' : 'text-gray-400'}`} />
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
              <div className="mt-4">
                <Button
                  variant="secondary"
                  size="sm"
                  className="w-full"
                  onClick={() => navigate('/tasks/logs?search=crowdsec')}
                >
                  View Logs
                </Button>
              </div>
            )}
          </div>
        </Card>

        {/* WAF */}
        <Card className={status.waf.enabled ? 'border-green-200 dark:border-green-900' : ''}>
          <div className="flex flex-row items-center justify-between pb-2">
            <h3 className="text-sm font-medium text-white">WAF (Coraza)</h3>
            <Shield className={`w-4 h-4 ${status.waf.enabled ? 'text-green-500' : 'text-gray-400'}`} />
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
            <Lock className={`w-4 h-4 ${status.acl.enabled ? 'text-green-500' : 'text-gray-400'}`} />
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
                <Button variant="secondary" size="sm" className="w-full">
                  Manage Lists
                </Button>
              </div>
            )}
          </div>
        </Card>

        {/* Rate Limiting */}
        <Card className={status.rate_limit.enabled ? 'border-green-200 dark:border-green-900' : ''}>
          <div className="flex flex-row items-center justify-between pb-2">
            <h3 className="text-sm font-medium text-white">Rate Limiting</h3>
            <Activity className={`w-4 h-4 ${status.rate_limit.enabled ? 'text-green-500' : 'text-gray-400'}`} />
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
          </div>
        </Card>
      </div>
    </div>
  )
}
