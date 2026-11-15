import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import toast from 'react-hot-toast'
import { useTranslation } from 'react-i18next'

import { wafErrorMessage } from '../api/errorBody'
import {
  getSecurityStatus,
  getManagementAccess,
  getTrustedProxies,
  resetTrustedProxies,
  saveManagementAccess,
  saveTrustedProxies,
  getSecurityConfig,
  getLoginProtectionStatus,
  updateSecurityConfig,
  generateBreakGlassToken,
  enableCerberus,
  disableCerberus,
  getDecisions,
  createDecision,
  getRuleSets,
  upsertRuleSet,
  deleteRuleSet,
  type UpsertRuleSetPayload,
  type SecurityConfigPayload,
  type CreateDecisionPayload,
} from '../api/security'

/**
 * The one query key for the security status. Every page that reads or invalidates the status uses it, so a
 * change on one page (for example the WAF rule-set source) refreshes the status everywhere.
 */
export const SECURITY_STATUS_QUERY_KEY = ['securityStatus'] as const

export function useSecurityStatus() {
  return useQuery({ queryKey: SECURITY_STATUS_QUERY_KEY, queryFn: getSecurityStatus })
}

/** Admin-only sign-in throttle status; pass `enabled: false` for non-admins so no request is made. */
export function useLoginProtectionStatus({ enabled }: { enabled: boolean }) {
  return useQuery({
    queryKey: ['loginProtectionStatus'],
    queryFn: getLoginProtectionStatus,
    enabled,
    retry: false,
  })
}

/** The one query key for the proxy-layer trust list. */
export const TRUSTED_PROXIES_QUERY_KEY = ['trustedProxies'] as const

/** The one query key for the access lists that protect Charon's own interface. */
export const MANAGEMENT_ACCESS_QUERY_KEY = ['managementAccess'] as const

/** Access lists chosen for Charon's own interface; every signed-in user may read it. */
export function useManagementAccess() {
  return useQuery({ queryKey: MANAGEMENT_ACCESS_QUERY_KEY, queryFn: getManagementAccess })
}

/** Replaces the chosen management access lists and stores the set the server now holds. */
export function useSaveManagementAccess() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (listUuids: string[]) => saveManagementAccess(listUuids),
    onSuccess: (data) => {
      qc.setQueryData(MANAGEMENT_ACCESS_QUERY_KEY, data)
    },
  })
}

/** Proxy-layer trust list; every signed-in user may read it (non-administrators get a reduced response). */
export function useTrustedProxies() {
  return useQuery({ queryKey: TRUSTED_PROXIES_QUERY_KEY, queryFn: getTrustedProxies })
}

/** Saves the trusted-proxy list and stores the list the server now applies. Refusals reach the caller's error state. */
export function useSaveTrustedProxies() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (proxies: string[]) => saveTrustedProxies(proxies),
    onSuccess: (data) => {
      qc.setQueryData(TRUSTED_PROXIES_QUERY_KEY, data)
    },
  })
}

/** Clears the saved trusted-proxy list and stores the list that applies instead. */
export function useResetTrustedProxies() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => resetTrustedProxies(),
    onSuccess: (data) => {
      qc.setQueryData(TRUSTED_PROXIES_QUERY_KEY, data)
    },
  })
}

export function useSecurityConfig() {
  return useQuery({ queryKey: ['securityConfig'], queryFn: getSecurityConfig })
}

export function useUpdateSecurityConfig() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload: SecurityConfigPayload) => updateSecurityConfig(payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityConfig'] })
      qc.invalidateQueries({ queryKey: SECURITY_STATUS_QUERY_KEY })
      toast.success('Security configuration updated')
    },
    onError: (err: Error) => {
      toast.error(`Failed to update security settings: ${err.message}`)
    },
  })
}

export function useGenerateBreakGlassToken() {
  return useMutation({ mutationFn: () => generateBreakGlassToken() })
}

export function useDecisions(limit = 50) {
  return useQuery({ queryKey: ['securityDecisions', limit], queryFn: () => getDecisions(limit) })
}

export function useCreateDecision() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload: CreateDecisionPayload) => createDecision(payload),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['securityDecisions'] }),
  })
}

export function useRuleSets() {
  return useQuery({ queryKey: ['securityRulesets'], queryFn: () => getRuleSets() })
}

export function useUpsertRuleSet() {
  const qc = useQueryClient()
  const { t } = useTranslation()
  return useMutation({
    mutationFn: (payload: UpsertRuleSetPayload) => upsertRuleSet(payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityRulesets'] })
      toast.success('Rule set saved successfully')
    },
    onError: (err: Error) => {
      toast.error(wafErrorMessage(err, t, `Failed to save rule set: ${err.message}`))
    },
  })
}

export function useDeleteRuleSet() {
  const qc = useQueryClient()
  const { t } = useTranslation()
  return useMutation({
    mutationFn: (uuid: string) => deleteRuleSet(uuid),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityRulesets'] })
      toast.success('Rule set deleted')
    },
    onError: (err: Error) => {
      toast.error(wafErrorMessage(err, t, `Failed to delete rule set: ${err.message}`))
    },
  })
}

/**
 * Chooses the rule set the WAF inspects requests with. The empty string means "Choose automatically". Saves only
 * the rule-set source (the field-level save), so no other stored setting changes.
 */
export function useUpdateWafRulesSource() {
  const qc = useQueryClient()
  const { t } = useTranslation()
  return useMutation({
    mutationFn: (source: string) =>
      updateSecurityConfig({ name: 'default', waf_rules_source: source }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityConfig'] })
      qc.invalidateQueries({ queryKey: SECURITY_STATUS_QUERY_KEY })
      toast.success(t('wafConfig.rulesSource.saved'))
    },
    onError: (err: unknown) => {
      toast.error(wafErrorMessage(err, t, t('wafConfig.rulesSource.saveFailed')))
    },
  })
}

export function useEnableCerberus() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload?: Record<string, unknown>) => enableCerberus(payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityConfig'] })
      qc.invalidateQueries({ queryKey: SECURITY_STATUS_QUERY_KEY })
      toast.success('Cerberus enabled')
    },
    onError: (err: Error) => {
      toast.error(`Failed to enable Cerberus: ${err.message}`)
    },
  })
}

export function useDisableCerberus() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload?: Record<string, unknown>) => disableCerberus(payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityConfig'] })
      qc.invalidateQueries({ queryKey: SECURITY_STATUS_QUERY_KEY })
      toast.success('Cerberus disabled')
    },
    onError: (err: Error) => {
      toast.error(`Failed to disable Cerberus: ${err.message}`)
    },
  })
}
