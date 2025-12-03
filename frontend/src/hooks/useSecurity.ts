import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getSecurityStatus,
  getSecurityConfig,
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
} from '../api/security'
import toast from 'react-hot-toast'

export function useSecurityStatus() {
  return useQuery({ queryKey: ['securityStatus'], queryFn: getSecurityStatus })
}

export function useSecurityConfig() {
  return useQuery({ queryKey: ['securityConfig'], queryFn: getSecurityConfig })
}

export function useUpdateSecurityConfig() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload: any) => updateSecurityConfig(payload),
    onSuccess: () => { // Specify a more accurate type for payload
      qc.invalidateQueries({ queryKey: ['securityConfig'] })
      qc.invalidateQueries({ queryKey: ['securityStatus'] })
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
    mutationFn: (payload: any) => createDecision(payload),
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
  return useMutation({
    mutationFn: (id: number) => deleteRuleSet(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityRulesets'] })
      toast.success('Rule set deleted')
    },
    onError: (err: Error) => {
      toast.error(`Failed to delete rule set: ${err.message}`)
    },
  })
}

export function useEnableCerberus() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload?: any) => enableCerberus(payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityConfig'] })
      qc.invalidateQueries({ queryKey: ['securityStatus'] })
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
    mutationFn: (payload?: any) => disableCerberus(payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['securityConfig'] })
      qc.invalidateQueries({ queryKey: ['securityStatus'] })
      toast.success('Cerberus disabled')
    },
    onError: (err: Error) => {
      toast.error(`Failed to disable Cerberus: ${err.message}`)
    },
  })
}
