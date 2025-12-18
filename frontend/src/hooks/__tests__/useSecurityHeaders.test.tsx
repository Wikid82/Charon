import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  useSecurityHeaderProfiles,
  useSecurityHeaderProfile,
  useCreateSecurityHeaderProfile,
  useUpdateSecurityHeaderProfile,
  useDeleteSecurityHeaderProfile,
  useSecurityHeaderPresets,
  useApplySecurityHeaderPreset,
  useCalculateSecurityScore,
  useValidateCSP,
  useBuildCSP,
} from '../useSecurityHeaders';
import { securityHeadersApi } from '../../api/securityHeaders';
import toast from 'react-hot-toast';

vi.mock('../../api/securityHeaders');
vi.mock('react-hot-toast');

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe('useSecurityHeaders', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('useSecurityHeaderProfiles', () => {
    it('should fetch profiles successfully', async () => {
      const mockProfiles = [
        { id: 1, name: 'Profile 1', security_score: 85 },
        { id: 2, name: 'Profile 2', security_score: 90 },
      ];

      vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);

      const { result } = renderHook(() => useSecurityHeaderProfiles(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockProfiles);
      expect(securityHeadersApi.listProfiles).toHaveBeenCalledTimes(1);
    });

    it('should handle error when fetching profiles', async () => {
      vi.mocked(securityHeadersApi.listProfiles).mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useSecurityHeaderProfiles(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isError).toBe(true));

      expect(result.current.error).toBeInstanceOf(Error);
    });
  });

  describe('useSecurityHeaderProfile', () => {
    it('should fetch a single profile', async () => {
      const mockProfile = { id: 1, name: 'Profile 1', security_score: 85 };

      vi.mocked(securityHeadersApi.getProfile).mockResolvedValue(mockProfile as any);

      const { result } = renderHook(() => useSecurityHeaderProfile(1), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockProfile);
      expect(securityHeadersApi.getProfile).toHaveBeenCalledWith(1);
    });

    it('should not fetch when id is undefined', () => {
      const { result } = renderHook(() => useSecurityHeaderProfile(undefined), {
        wrapper: createWrapper(),
      });

      expect(result.current.data).toBeUndefined();
      expect(securityHeadersApi.getProfile).not.toHaveBeenCalled();
    });
  });

  describe('useCreateSecurityHeaderProfile', () => {
    it('should create a profile successfully', async () => {
      const newProfile = { name: 'New Profile', hsts_enabled: true };
      const createdProfile = { id: 1, ...newProfile, security_score: 80 };

      vi.mocked(securityHeadersApi.createProfile).mockResolvedValue(createdProfile as any);

      const { result } = renderHook(() => useCreateSecurityHeaderProfile(), {
        wrapper: createWrapper(),
      });

      result.current.mutate(newProfile as any);

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(securityHeadersApi.createProfile).toHaveBeenCalledWith(newProfile);
      expect(toast.success).toHaveBeenCalledWith('Security header profile created successfully');
    });

    it('should handle error when creating profile', async () => {
      vi.mocked(securityHeadersApi.createProfile).mockRejectedValue(new Error('Validation error'));

      const { result } = renderHook(() => useCreateSecurityHeaderProfile(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ name: 'Test' } as any);

      await waitFor(() => expect(result.current.isError).toBe(true));

      expect(toast.error).toHaveBeenCalledWith('Failed to create profile: Validation error');
    });
  });

  describe('useUpdateSecurityHeaderProfile', () => {
    it('should update a profile successfully', async () => {
      const updateData = { name: 'Updated Profile' };
      const updatedProfile = { id: 1, ...updateData, security_score: 85 };

      vi.mocked(securityHeadersApi.updateProfile).mockResolvedValue(updatedProfile as any);

      const { result } = renderHook(() => useUpdateSecurityHeaderProfile(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ id: 1, data: updateData as any });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(securityHeadersApi.updateProfile).toHaveBeenCalledWith(1, updateData);
      expect(toast.success).toHaveBeenCalledWith('Security header profile updated successfully');
    });

    it('should handle error when updating profile', async () => {
      vi.mocked(securityHeadersApi.updateProfile).mockRejectedValue(new Error('Not found'));

      const { result } = renderHook(() => useUpdateSecurityHeaderProfile(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ id: 1, data: { name: 'Test' } as any });

      await waitFor(() => expect(result.current.isError).toBe(true));

      expect(toast.error).toHaveBeenCalledWith('Failed to update profile: Not found');
    });
  });

  describe('useDeleteSecurityHeaderProfile', () => {
    it('should delete a profile successfully', async () => {
      vi.mocked(securityHeadersApi.deleteProfile).mockResolvedValue(undefined);

      const { result } = renderHook(() => useDeleteSecurityHeaderProfile(), {
        wrapper: createWrapper(),
      });

      result.current.mutate(1);

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(securityHeadersApi.deleteProfile).toHaveBeenCalledWith(1);
      expect(toast.success).toHaveBeenCalledWith('Security header profile deleted successfully');
    });

    it('should handle error when deleting profile', async () => {
      vi.mocked(securityHeadersApi.deleteProfile).mockRejectedValue(new Error('Cannot delete preset'));

      const { result } = renderHook(() => useDeleteSecurityHeaderProfile(), {
        wrapper: createWrapper(),
      });

      result.current.mutate(1);

      await waitFor(() => expect(result.current.isError).toBe(true));

      expect(toast.error).toHaveBeenCalledWith('Failed to delete profile: Cannot delete preset');
    });
  });

  describe('useSecurityHeaderPresets', () => {
    it('should fetch presets successfully', async () => {
      const mockPresets = [
        { type: 'basic', name: 'Basic Security', score: 65 },
        { type: 'strict', name: 'Strict Security', score: 85 },
      ];

      vi.mocked(securityHeadersApi.getPresets).mockResolvedValue(mockPresets as any);

      const { result } = renderHook(() => useSecurityHeaderPresets(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockPresets);
    });
  });

  describe('useApplySecurityHeaderPreset', () => {
    it('should apply preset successfully', async () => {
      const appliedProfile = { id: 1, name: 'Basic Security', security_score: 65 };

      vi.mocked(securityHeadersApi.applyPreset).mockResolvedValue(appliedProfile as any);

      const { result } = renderHook(() => useApplySecurityHeaderPreset(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ preset_type: 'basic', name: 'Basic Security' });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(securityHeadersApi.applyPreset).toHaveBeenCalledWith({ preset_type: 'basic', name: 'Basic Security' });
      expect(toast.success).toHaveBeenCalledWith('Preset applied successfully');
    });
  });

  describe('useCalculateSecurityScore', () => {
    it('should calculate score successfully', async () => {
      const mockScore = {
        score: 85,
        max_score: 100,
        breakdown: { hsts: 25, csp: 20 },
        suggestions: ['Enable CSP'],
      };

      vi.mocked(securityHeadersApi.calculateScore).mockResolvedValue(mockScore);

      const { result } = renderHook(() => useCalculateSecurityScore(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ hsts_enabled: true } as any);

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockScore);
    });
  });

  describe('useValidateCSP', () => {
    it('should validate CSP successfully', async () => {
      const mockValidation = { valid: true, errors: [] };

      vi.mocked(securityHeadersApi.validateCSP).mockResolvedValue(mockValidation);

      const { result } = renderHook(() => useValidateCSP(), {
        wrapper: createWrapper(),
      });

      result.current.mutate("default-src 'self'");

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockValidation);
    });
  });

  describe('useBuildCSP', () => {
    it('should build CSP string successfully', async () => {
      const mockDirectives = [
        { directive: 'default-src', values: ["'self'"] },
      ];
      const mockResult = { csp: "default-src 'self'" };

      vi.mocked(securityHeadersApi.buildCSP).mockResolvedValue(mockResult);

      const { result } = renderHook(() => useBuildCSP(), {
        wrapper: createWrapper(),
      });

      result.current.mutate(mockDirectives);

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockResult);
    });
  });
});
