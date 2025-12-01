import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getSecurityStatus, getSecurityConfig, updateSecurityConfig, generateBreakGlassToken, enableCerberus, disableCerberus } from '../api/security'
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
    onSuccess: () => {
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
