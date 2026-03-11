import { describe, it, expect, vi, beforeEach } from 'vitest';

import client from '../client';
import { securityHeadersApi } from '../securityHeaders';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('securityHeadersApi', () => {
  const mockedGet = vi.mocked(client.get);
  const mockedPost = vi.mocked(client.post);
  const mockedPut = vi.mocked(client.put);
  const mockedDelete = vi.mocked(client.delete);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('listProfiles returns profiles', async () => {
    const mockProfiles = [{ id: 1, name: 'Profile 1' }];
    mockedGet.mockResolvedValue({ data: { profiles: mockProfiles } });

    const result = await securityHeadersApi.listProfiles();
    expect(client.get).toHaveBeenCalledWith('/security/headers/profiles');
    expect(result).toEqual(mockProfiles);
  });

  it('getProfile returns a profile', async () => {
    const mockProfile = { id: 1, name: 'Profile 1' };
    mockedGet.mockResolvedValue({ data: { profile: mockProfile } });

    const result = await securityHeadersApi.getProfile(1);
    expect(client.get).toHaveBeenCalledWith('/security/headers/profiles/1');
    expect(result).toEqual(mockProfile);
  });

  it('getProfile accepts UUID string identifiers', async () => {
    const mockProfile = { id: 2, uuid: 'profile-uuid', name: 'Profile UUID' };
    mockedGet.mockResolvedValue({ data: { profile: mockProfile } });

    const result = await securityHeadersApi.getProfile('profile-uuid');
    expect(client.get).toHaveBeenCalledWith('/security/headers/profiles/profile-uuid');
    expect(result).toEqual(mockProfile);
  });

  it('createProfile creates a profile', async () => {
    const newProfile = { name: 'New Profile' };
    const mockResponse = { id: 1, ...newProfile };
    mockedPost.mockResolvedValue({ data: { profile: mockResponse } });

    const result = await securityHeadersApi.createProfile(newProfile);
    expect(client.post).toHaveBeenCalledWith('/security/headers/profiles', newProfile);
    expect(result).toEqual(mockResponse);
  });

  it('updateProfile updates a profile', async () => {
    const updates = { name: 'Updated Profile' };
    const mockResponse = { id: 1, ...updates };
    mockedPut.mockResolvedValue({ data: { profile: mockResponse } });

    const result = await securityHeadersApi.updateProfile(1, updates);
    expect(client.put).toHaveBeenCalledWith('/security/headers/profiles/1', updates);
    expect(result).toEqual(mockResponse);
  });

  it('deleteProfile deletes a profile', async () => {
    mockedDelete.mockResolvedValue({});

    await securityHeadersApi.deleteProfile(1);
    expect(client.delete).toHaveBeenCalledWith('/security/headers/profiles/1');
  });

  it('forwards API errors from listProfiles', async () => {
    const error = new Error('backend unavailable');
    mockedGet.mockRejectedValue(error);

    await expect(securityHeadersApi.listProfiles()).rejects.toBe(error);
  });

  it('getPresets returns presets', async () => {
    const mockPresets = [{ name: 'Basic' }];
    mockedGet.mockResolvedValue({ data: { presets: mockPresets } });

    const result = await securityHeadersApi.getPresets();
    expect(client.get).toHaveBeenCalledWith('/security/headers/presets');
    expect(result).toEqual(mockPresets);
  });

  it('applyPreset applies a preset', async () => {
    const request = { preset_type: 'basic', name: 'My Preset' };
    const mockResponse = { id: 1, ...request };
    mockedPost.mockResolvedValue({ data: { profile: mockResponse } });

    const result = await securityHeadersApi.applyPreset(request);
    expect(client.post).toHaveBeenCalledWith('/security/headers/presets/apply', request);
    expect(result).toEqual(mockResponse);
  });

  it('calculateScore calculates score', async () => {
    const config = { hsts_enabled: true };
    const mockResponse = { score: 90 };
    mockedPost.mockResolvedValue({ data: mockResponse });

    const result = await securityHeadersApi.calculateScore(config);
    expect(client.post).toHaveBeenCalledWith('/security/headers/score', config);
    expect(result).toEqual(mockResponse);
  });

  it('validateCSP validates CSP', async () => {
      const csp = "default-src 'self'";
      const mockResponse = { valid: true, errors: [] };
      mockedPost.mockResolvedValue({ data: mockResponse });

      const result = await securityHeadersApi.validateCSP(csp);
      expect(client.post).toHaveBeenCalledWith('/security/headers/csp/validate', { csp });
      expect(result).toEqual(mockResponse);
  });

  it('buildCSP builds CSP', async () => {
      const directives = [{ directive: 'default-src', values: ["'self'"] }];
      const mockResponse = { csp: "default-src 'self'" };
      mockedPost.mockResolvedValue({ data: mockResponse });

      const result = await securityHeadersApi.buildCSP(directives);
      expect(client.post).toHaveBeenCalledWith('/security/headers/csp/build', { directives });
      expect(result).toEqual(mockResponse);
  });
});
