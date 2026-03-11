import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import {
  uploadCaddyfile,
  getImportPreview,
  commitImport,
  cancelImport,
  getImportStatus,
  type ImportSession,
  type ImportPreview,
  type ImportCommitResult,
} from '../api/import';

export const QUERY_KEY = ['import-session'];

export function useImport() {
  const queryClient = useQueryClient();
  // Track when commit has succeeded to disable preview fetching
  const [commitSucceeded, setCommitSucceeded] = useState(false);
  // Store the commit result for display in success modal
  const [commitResult, setCommitResult] = useState<ImportCommitResult | null>(null);
  // Store preview data from upload response to avoid race condition
  const [uploadPreview, setUploadPreview] = useState<ImportPreview | null>(null);

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
    onSuccess: (data) => {
      // Store preview data immediately from upload response to avoid race condition
      setUploadPreview(data);
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ['import-preview'] });
    },
  });

  const commitMutation = useMutation({
    mutationFn: ({ resolutions, names }: { resolutions: Record<string, string>; names: Record<string, string> }) => {
      // Use session ID from uploadPreview (immediate) or statusQuery (background)
      const sessionId = uploadPreview?.session?.id || statusQuery.data?.session?.id;
      if (!sessionId) throw new Error("No active session");
      return commitImport(sessionId, resolutions, names);
    },
    onSuccess: (result) => {
      // Store the commit result for display in success modal
      setCommitResult(result);
      // Mark commit as succeeded to prevent preview refetch (which would 404)
      setCommitSucceeded(true);
      // Clear upload preview and remove query cache
      setUploadPreview(null);
      queryClient.removeQueries({ queryKey: ['import-preview'] });
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      // Also invalidate proxy hosts as they might have changed
      queryClient.invalidateQueries({ queryKey: ['proxy-hosts'] });
    },
  });

  const cancelMutation = useMutation({
    mutationFn: () => {
      const sessionId = uploadPreview?.session?.id || statusQuery.data?.session?.id;
      if (!sessionId) throw new Error('No active session');
      return cancelImport(sessionId);
    },
    onSuccess: () => {
      // Clear upload preview and remove query cache
      setUploadPreview(null);
      queryClient.removeQueries({ queryKey: ['import-preview'] });
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });

  const clearCommitResult = () => {
    setCommitResult(null);
    setCommitSucceeded(false);
    setUploadPreview(null);
  };

  return {
    session: statusQuery.data?.session || uploadPreview?.session || null,
    // Use uploadPreview (immediately available) or previewQuery.data (background refetch)
    preview: uploadPreview || previewQuery.data || null,
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
