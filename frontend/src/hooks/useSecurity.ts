import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import toast from 'react-hot-toast'
import { useTranslation } from 'react-i18next'

import { wafErrorMessage } from '../api/errorBody'
import {
  getSecurityStatus,
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
  return useMutation({
    mutationFn: (payload: UpsertRuleSetPayload) => upsertRuleSet(payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityRulesets'] })
      toast.success('Rule set saved successfully')
    },
    onError: (err: Error) => {
      toast.error(`Failed to save rule set: ${err.message}`)
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
