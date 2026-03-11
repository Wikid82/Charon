import { describe, it, expect, vi, beforeEach } from 'vitest';

import client from '../client';
import { uploadNPMExport, commitNPMImport, cancelNPMImport } from '../npmImport';

vi.mock('../client', () => ({
  default: {
    post: vi.fn(),
  },
}));

describe('npmImport API', () => {
  const mockedPost = vi.mocked(client.post);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('cancelNPMImport posts cancel endpoint with required session_uuid body', async () => {
    const sessionUUID = 'npm-session-123';
    mockedPost.mockResolvedValue({});

    await cancelNPMImport(sessionUUID);

    expect(client.post).toHaveBeenCalledWith('/import/npm/cancel', {
      session_uuid: sessionUUID,
    });
  });

  it('uploadNPMExport posts upload endpoint with content payload', async () => {
    const content = '{"proxy_hosts":[]}';
    const mockResponse = {
      session: {
        id: 'npm-session-456',
        state: 'reviewing',
        source: 'npm',
      },
      preview: {
        hosts: [],
        conflicts: [],
        errors: [],
      },
      conflict_details: {},
    };

    mockedPost.mockResolvedValue({ data: mockResponse });

    const result = await uploadNPMExport(content);

    expect(client.post).toHaveBeenCalledWith('/import/npm/upload', { content });
    expect(result).toEqual(mockResponse);
  });

  it('commitNPMImport posts commit endpoint with session_uuid, resolutions, and names body', async () => {
    const sessionUUID = 'npm-session-789';
    const resolutions = { 'npm.example.com': 'replace' };
    const names = { 'npm.example.com': 'NPM Example' };
    const mockResponse = {
      created: 1,
      updated: 1,
      skipped: 0,
      errors: [],
    };

    mockedPost.mockResolvedValue({ data: mockResponse });

    const result = await commitNPMImport(sessionUUID, resolutions, names);

    expect(client.post).toHaveBeenCalledWith('/import/npm/commit', {
      session_uuid: sessionUUID,
      resolutions,
      names,
    });
    expect(result).toEqual(mockResponse);
  });

  it('forwards uploadNPMExport errors', async () => {
    const error = new Error('upload failed');
    mockedPost.mockRejectedValue(error);

    await expect(uploadNPMExport('{"proxy_hosts":[]}')).rejects.toBe(error);
  });

  it('forwards commitNPMImport errors', async () => {
    const error = new Error('commit failed');
    mockedPost.mockRejectedValue(error);

    await expect(commitNPMImport('npm-session-123', {}, {})).rejects.toBe(error);
  });

  it('forwards cancelNPMImport errors', async () => {
    const error = new Error('cancel failed');
    mockedPost.mockRejectedValue(error);

    await expect(cancelNPMImport('npm-session-123')).rejects.toBe(error);
  });
});
