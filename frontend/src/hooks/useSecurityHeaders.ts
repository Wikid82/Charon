import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import toast from 'react-hot-toast';

import { securityHeadersApi, type CreateProfileRequest, type ApplyPresetRequest  } from '../api/securityHeaders';


export function useSecurityHeaderProfiles() {
  return useQuery({
    queryKey: ['securityHeaderProfiles'],
    queryFn: securityHeadersApi.listProfiles,
  });
}

export function useSecurityHeaderProfile(id: number | string | undefined) {
  return useQuery({
    queryKey: ['securityHeaderProfile', id],
    queryFn: () => securityHeadersApi.getProfile(id!),
    enabled: !!id,
  });
}

export function useCreateSecurityHeaderProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateProfileRequest) => securityHeadersApi.createProfile(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['securityHeaderProfiles'] });
      toast.success('Security header profile created successfully');
    },
    onError: (error: Error) => {
      toast.error(`Failed to create profile: ${error.message}`);
    },
  });
}

export function useUpdateSecurityHeaderProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: Partial<CreateProfileRequest> }) =>
      securityHeadersApi.updateProfile(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['securityHeaderProfiles'] });
      queryClient.invalidateQueries({ queryKey: ['securityHeaderProfile', variables.id] });
      toast.success('Security header profile updated successfully');
    },
    onError: (error: Error) => {
      toast.error(`Failed to update profile: ${error.message}`);
    },
  });
}

export function useDeleteSecurityHeaderProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => securityHeadersApi.deleteProfile(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['securityHeaderProfiles'] });
      toast.success('Security header profile deleted successfully');
    },
    onError: (error: Error) => {
      toast.error(`Failed to delete profile: ${error.message}`);
    },
  });
}

export function useSecurityHeaderPresets() {
  return useQuery({
    queryKey: ['securityHeaderPresets'],
    queryFn: securityHeadersApi.getPresets,
  });
}

export function useApplySecurityHeaderPreset() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: ApplyPresetRequest) => securityHeadersApi.applyPreset(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['securityHeaderProfiles'] });
      toast.success('Preset applied successfully');
    },
    onError: (error: Error) => {
      toast.error(`Failed to apply preset: ${error.message}`);
    },
  });
}

export function useCalculateSecurityScore() {
  return useMutation({
    mutationFn: (config: Partial<CreateProfileRequest>) => securityHeadersApi.calculateScore(config),
  });
}

export function useValidateCSP() {
  return useMutation({
    mutationFn: (csp: string) => securityHeadersApi.validateCSP(csp),
  });
}

export function useBuildCSP() {
  return useMutation({
    mutationFn: (directives: { directive: string; values: string[] }[]) =>
      securityHeadersApi.buildCSP(directives),
  });
}
