import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  uploadCaddyfile,
  getImportPreview,
  commitImport,
  cancelImport,
  getImportStatus,
  ImportSession,
  ImportPreview
} from '../api/import';

export const QUERY_KEY = ['import-session'];

export function useImport() {
  const queryClient = useQueryClient();

  // Poll for status if we think there's an active session
  const statusQuery = useQuery({
    queryKey: QUERY_KEY,
    queryFn: getImportStatus,
    refetchInterval: (query) => {
      const data = query.state.data;
      // Poll if we have a pending session in reviewing state
      if (data?.has_pending && data?.session?.state === 'reviewing') {
        return 3000;
      }
      return false;
    },
  });

  const previewQuery = useQuery({
    queryKey: ['import-preview'],
    queryFn: getImportPreview,
    enabled: !!statusQuery.data?.has_pending && statusQuery.data?.session?.state === 'reviewing',
  });

  const uploadMutation = useMutation({
    mutationFn: (content: string) => uploadCaddyfile(content),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ['import-preview'] });
    },
  });

  const commitMutation = useMutation({
    mutationFn: (resolutions: Record<string, string>) => commitImport(resolutions),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ['import-preview'] });
      // Also invalidate proxy hosts as they might have changed
      queryClient.invalidateQueries({ queryKey: ['proxy-hosts'] });
    },
  });

  const cancelMutation = useMutation({
    mutationFn: () => cancelImport(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ['import-preview'] });
    },
  });

  return {
    session: statusQuery.data?.session || null,
    preview: previewQuery.data || null,
    loading: statusQuery.isLoading || uploadMutation.isPending || commitMutation.isPending || cancelMutation.isPending,
    error: (statusQuery.error || previewQuery.error || uploadMutation.error || commitMutation.error || cancelMutation.error)
      ? ((statusQuery.error || previewQuery.error || uploadMutation.error || commitMutation.error || cancelMutation.error) as Error).message
      : null,
    upload: uploadMutation.mutateAsync,
    commit: commitMutation.mutateAsync,
    cancel: cancelMutation.mutateAsync,
  };
}

export type { ImportSession, ImportPreview };
