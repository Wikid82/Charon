import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  uploadCaddyfile,
  getImportPreview,
  commitImport,
  cancelImport,
  getImportStatus,
  ImportSession,
  ImportPreview,
  ImportCommitResult,
} from '../api/import';

export const QUERY_KEY = ['import-session'];

export function useImport() {
  const queryClient = useQueryClient();
  // Track when commit has succeeded to disable preview fetching
  const [commitSucceeded, setCommitSucceeded] = useState(false);
  // Store the commit result for display in success modal
  const [commitResult, setCommitResult] = useState<ImportCommitResult | null>(null);

  // Poll for status if we think there's an active session
  const statusQuery = useQuery({
    queryKey: QUERY_KEY,
    queryFn: getImportStatus,
    refetchInterval: (query) => {
      const data = query.state.data;
      // Poll if we have a pending session in reviewing state (but not transient, as those don't change)
      if (data?.has_pending && data?.session?.state === 'reviewing') {
        return 3000;
      }
      return false;
    },
  });

  const previewQuery = useQuery({
    queryKey: ['import-preview'],
    queryFn: getImportPreview,
    // Only enable when there's an active session AND commit hasn't just succeeded
    enabled: !!statusQuery.data?.has_pending &&
      (statusQuery.data?.session?.state === 'reviewing' || statusQuery.data?.session?.state === 'pending' || statusQuery.data?.session?.state === 'transient') &&
      !commitSucceeded,
  });

  const uploadMutation = useMutation({
    mutationFn: (content: string) => uploadCaddyfile(content),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ['import-preview'] });
    },
  });

  const commitMutation = useMutation({
    mutationFn: ({ resolutions, names }: { resolutions: Record<string, string>; names: Record<string, string> }) => {
      const sessionId = statusQuery.data?.session?.id;
      if (!sessionId) throw new Error("No active session");
      return commitImport(sessionId, resolutions, names);
    },
    onSuccess: (result) => {
      // Store the commit result for display in success modal
      setCommitResult(result);
      // Mark commit as succeeded to prevent preview refetch (which would 404)
      setCommitSucceeded(true);
      // Remove preview cache entirely to prevent 404 refetch after commit
      // (the session no longer exists, so preview endpoint returns 404)
      queryClient.removeQueries({ queryKey: ['import-preview'] });
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      // Also invalidate proxy hosts as they might have changed
      queryClient.invalidateQueries({ queryKey: ['proxy-hosts'] });
    },
  });

  const cancelMutation = useMutation({
    mutationFn: () => cancelImport(),
    onSuccess: () => {
      // Remove preview cache entirely to prevent 404 refetch after cancel
      queryClient.removeQueries({ queryKey: ['import-preview'] });
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });

  const clearCommitResult = () => {
    setCommitResult(null);
    setCommitSucceeded(false);
  };

  return {
    session: statusQuery.data?.session || null,
    preview: previewQuery.data || null,
    loading: statusQuery.isLoading || uploadMutation.isPending || commitMutation.isPending || cancelMutation.isPending,
    // Only include previewQuery.error if there's an active session and commit hasn't succeeded
    // (404 expected when no session or after commit)
    error: (statusQuery.error || (previewQuery.error && statusQuery.data?.has_pending && !commitSucceeded) || uploadMutation.error || commitMutation.error || cancelMutation.error)
      ? ((statusQuery.error || (previewQuery.error && statusQuery.data?.has_pending && !commitSucceeded ? previewQuery.error : null) || uploadMutation.error || commitMutation.error || cancelMutation.error) as Error)?.message
      : null,
    commitSuccess: commitSucceeded,
    commitResult,
    clearCommitResult,
    upload: uploadMutation.mutateAsync,
    commit: (resolutions: Record<string, string>, names: Record<string, string>) =>
      commitMutation.mutateAsync({ resolutions, names }),
    cancel: cancelMutation.mutateAsync,
  };
}

export type { ImportSession, ImportPreview, ImportCommitResult };
