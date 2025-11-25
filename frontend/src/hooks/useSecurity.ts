import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import * as api from '../api/security';

// Users Hooks
export const useAuthUsers = () => {
  const queryClient = useQueryClient();

  const { data: users = [], isLoading, error } = useQuery({
    queryKey: ['auth-users'],
    queryFn: api.getAuthUsers,
  });

  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['auth-users-stats'],
    queryFn: api.getAuthUserStats,
  });

  const createMutation = useMutation({
    mutationFn: api.createAuthUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth-users'] });
      queryClient.invalidateQueries({ queryKey: ['auth-users-stats'] });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ uuid, data }: { uuid: string; data: api.UpdateAuthUserRequest }) =>
      api.updateAuthUser(uuid, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth-users'] });
      queryClient.invalidateQueries({ queryKey: ['auth-users-stats'] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: api.deleteAuthUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth-users'] });
      queryClient.invalidateQueries({ queryKey: ['auth-users-stats'] });
    },
  });

  return {
    users,
    stats,
    isLoading: isLoading || statsLoading,
    error,
    createUser: createMutation.mutateAsync,
    updateUser: updateMutation.mutateAsync,
    deleteUser: deleteMutation.mutateAsync,
  };
};

// Providers Hooks
export const useAuthProviders = () => {
  const queryClient = useQueryClient();

  const { data: providers = [], isLoading, error } = useQuery({
    queryKey: ['auth-providers'],
    queryFn: api.getAuthProviders,
  });

  const createMutation = useMutation({
    mutationFn: api.createAuthProvider,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth-providers'] });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ uuid, data }: { uuid: string; data: api.UpdateAuthProviderRequest }) =>
      api.updateAuthProvider(uuid, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth-providers'] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: api.deleteAuthProvider,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth-providers'] });
    },
  });

  return {
    providers,
    isLoading,
    error,
    createProvider: createMutation.mutateAsync,
    updateProvider: updateMutation.mutateAsync,
    deleteProvider: deleteMutation.mutateAsync,
  };
};

// Policies Hooks
export const useAuthPolicies = () => {
  const queryClient = useQueryClient();

  const { data: policies = [], isLoading, error } = useQuery({
    queryKey: ['auth-policies'],
    queryFn: api.getAuthPolicies,
  });

  const createMutation = useMutation({
    mutationFn: api.createAuthPolicy,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth-policies'] });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ uuid, data }: { uuid: string; data: api.UpdateAuthPolicyRequest }) =>
      api.updateAuthPolicy(uuid, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth-policies'] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: api.deleteAuthPolicy,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth-policies'] });
    },
  });

  return {
    policies,
    isLoading,
    error,
    createPolicy: createMutation.mutateAsync,
    updatePolicy: updateMutation.mutateAsync,
    deletePolicy: deleteMutation.mutateAsync,
  };
};
