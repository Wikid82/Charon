import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  deleteAgent,
  getAgentProxyStatus,
  getInstallSnippets,
  getLocalNetworks,
  listAgents,
  patchAgent,
  provisionAgent,
  renameAgent,
  revokeAgent,
  rotateAgentKey,
  type ExternalProxyStatus,
  type LocalNetworks,
  type OrthrusAgent,
  type PatchAgentRequest,
  type ProvisionAgentRequest,
} from '../api/orthrus';

export const AGENTS_QUERY_KEY = ['orthrus', 'agents'] as const;

export const useAgentList = () =>
  useQuery<OrthrusAgent[]>({
    queryKey: AGENTS_QUERY_KEY,
    queryFn: listAgents,
  });

export const useProvisionAgent = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: ProvisionAgentRequest) => provisionAgent(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: AGENTS_QUERY_KEY });
    },
  });
};

export const useRenameAgent = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ uuid, name }: { uuid: string; name: string }) => renameAgent(uuid, name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: AGENTS_QUERY_KEY });
    },
  });
};

export const usePatchAgent = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ uuid, req }: { uuid: string; req: PatchAgentRequest }) =>
      patchAgent(uuid, req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: AGENTS_QUERY_KEY });
    },
  });
};

export const useRotateAgentKey = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (uuid: string) => rotateAgentKey(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: AGENTS_QUERY_KEY });
    },
  });
};

export const useDeleteAgent = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (uuid: string) => deleteAgent(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: AGENTS_QUERY_KEY });
    },
  });
};

export const useAgentProxyStatus = (agentUUID: string, enabled: boolean) =>
  useQuery<ExternalProxyStatus>({
    queryKey: ['orthrus', 'agents', agentUUID, 'proxy-status'],
    queryFn: () => getAgentProxyStatus(agentUUID),
    enabled,
    staleTime: 10_000,
  });

/** Host networks the external proxy can listen on. Only fetched while `enabled` (the dialog is open). */
export const useLocalNetworks = (enabled: boolean) =>
  useQuery<LocalNetworks>({
    queryKey: ['orthrus', 'local-networks'],
    queryFn: getLocalNetworks,
    enabled,
    staleTime: 30_000,
  });

export const useOrthrus = () => {
  const queryClient = useQueryClient();

  const {
    data: agents = [],
    isLoading: loading,
    error,
  } = useQuery<OrthrusAgent[]>({
    queryKey: AGENTS_QUERY_KEY,
    queryFn: listAgents,
  });

  const provisionMutation = useMutation({
    mutationFn: (req: ProvisionAgentRequest) => provisionAgent(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: AGENTS_QUERY_KEY });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (uuid: string) => deleteAgent(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: AGENTS_QUERY_KEY });
    },
  });

  const revokeMutation = useMutation({
    mutationFn: (uuid: string) => revokeAgent(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: AGENTS_QUERY_KEY });
    },
  });

  const snippetsMutation = useMutation({
    mutationFn: (uuid: string) => getInstallSnippets(uuid),
  });

  return {
    agents,
    loading,
    error,
    provisionAgent: provisionMutation.mutateAsync,
    deleteAgent: deleteMutation.mutate,
    revokeAgent: revokeMutation.mutate,
    getInstallSnippets: snippetsMutation.mutateAsync,
    isProvisioning: provisionMutation.isPending,
    isDeleting: deleteMutation.isPending,
    isRevoking: revokeMutation.isPending,
    isFetchingSnippets: snippetsMutation.isPending,
    provisionResult: provisionMutation.data,
  };
};
