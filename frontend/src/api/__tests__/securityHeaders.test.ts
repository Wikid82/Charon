import { describe, it, expect, vi, beforeEach } from 'vitest';
import { securityHeadersApi } from '../securityHeaders';
import client from '../client';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('securityHeadersApi', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('listProfiles returns profiles', async () => {
    const mockProfiles = [{ id: 1, name: 'Profile 1' }];
    (client.get as any).mockResolvedValue({ data: { profiles: mockProfiles } });

    const result = await securityHeadersApi.listProfiles();
    expect(client.get).toHaveBeenCalledWith('/security/headers/profiles');
    expect(result).toEqual(mockProfiles);
  });

  it('getProfile returns a profile', async () => {
    const mockProfile = { id: 1, name: 'Profile 1' };
    (client.get as any).mockResolvedValue({ data: { profile: mockProfile } });

    const result = await securityHeadersApi.getProfile(1);
    expect(client.get).toHaveBeenCalledWith('/security/headers/profiles/1');
    expect(result).toEqual(mockProfile);
  });

  it('createProfile creates a profile', async () => {
    const newProfile = { name: 'New Profile' };
    const mockResponse = { id: 1, ...newProfile };
    (client.post as any).mockResolvedValue({ data: { profile: mockResponse } });

    const result = await securityHeadersApi.createProfile(newProfile);
    expect(client.post).toHaveBeenCalledWith('/security/headers/profiles', newProfile);
    expect(result).toEqual(mockResponse);
  });

  it('updateProfile updates a profile', async () => {
    const updates = { name: 'Updated Profile' };
    const mockResponse = { id: 1, ...updates };
    (client.put as any).mockResolvedValue({ data: { profile: mockResponse } });

    const result = await securityHeadersApi.updateProfile(1, updates);
    expect(client.put).toHaveBeenCalledWith('/security/headers/profiles/1', updates);
    expect(result).toEqual(mockResponse);
  });

  it('deleteProfile deletes a profile', async () => {
    (client.delete as any).mockResolvedValue({});

    await securityHeadersApi.deleteProfile(1);
    expect(client.delete).toHaveBeenCalledWith('/security/headers/profiles/1');
  });

  it('getPresets returns presets', async () => {
    const mockPresets = [{ name: 'Basic' }];
    (client.get as any).mockResolvedValue({ data: { presets: mockPresets } });

    const result = await securityHeadersApi.getPresets();
    expect(client.get).toHaveBeenCalledWith('/security/headers/presets');
    expect(result).toEqual(mockPresets);
  });

  it('applyPreset applies a preset', async () => {
    const request = { preset_type: 'basic', name: 'My Preset' };
    const mockResponse = { id: 1, ...request };
    (client.post as any).mockResolvedValue({ data: { profile: mockResponse } });

    const result = await securityHeadersApi.applyPreset(request);
    expect(client.post).toHaveBeenCalledWith('/security/headers/presets/apply', request);
    expect(result).toEqual(mockResponse);
  });

  it('calculateScore calculates score', async () => {
    const config = { hsts_enabled: true };
    const mockResponse = { score: 90 };
    (client.post as any).mockResolvedValue({ data: mockResponse });

    const result = await securityHeadersApi.calculateScore(config);
    expect(client.post).toHaveBeenCalledWith('/security/headers/score', config);
    expect(result).toEqual(mockResponse);
  });

  it('validateCSP validates CSP', async () => {
      const csp = "default-src 'self'";
      const mockResponse = { valid: true, errors: [] };
      (client.post as any).mockResolvedValue({ data: mockResponse });

      const result = await securityHeadersApi.validateCSP(csp);
      expect(client.post).toHaveBeenCalledWith('/security/headers/csp/validate', { csp });
      expect(result).toEqual(mockResponse);
  });

  it('buildCSP builds CSP', async () => {
      const directives = [{ directive: 'default-src', values: ["'self'"] }];
      const mockResponse = { csp: "default-src 'self'" };
      (client.post as any).mockResolvedValue({ data: mockResponse });

      const result = await securityHeadersApi.buildCSP(directives);
      expect(client.post).toHaveBeenCalledWith('/security/headers/csp/build', { directives });
      expect(result).toEqual(mockResponse);
  });
});
