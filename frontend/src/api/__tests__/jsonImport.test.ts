import { describe, it, expect, vi, beforeEach } from 'vitest';

import client from '../client';
import { uploadJSONExport, commitJSONImport, cancelJSONImport } from '../jsonImport';

vi.mock('../client', () => ({
  default: {
    post: vi.fn(),
  },
}));

describe('jsonImport API', () => {
  const mockedPost = vi.mocked(client.post);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('cancelJSONImport posts cancel endpoint with required session_uuid body', async () => {
    const sessionUUID = 'json-session-123';
    mockedPost.mockResolvedValue({});

    await cancelJSONImport(sessionUUID);

    expect(client.post).toHaveBeenCalledWith('/import/json/cancel', {
      session_uuid: sessionUUID,
    });
  });

  it('uploadJSONExport posts upload endpoint with content payload', async () => {
    const content = '{"proxy_hosts":[]}';
    const mockResponse = {
      session: {
        id: 'json-session-456',
        state: 'reviewing',
        source: 'json',
      },
      preview: {
        hosts: [],
        conflicts: [],
        errors: [],
      },
      conflict_details: {},
    };

    mockedPost.mockResolvedValue({ data: mockResponse });

    const result = await uploadJSONExport(content);

    expect(client.post).toHaveBeenCalledWith('/import/json/upload', { content });
    expect(result).toEqual(mockResponse);
  });

  it('commitJSONImport posts commit endpoint with session_uuid, resolutions, and names body', async () => {
    const sessionUUID = 'json-session-789';
    const resolutions = { 'json.example.com': 'replace' };
    const names = { 'json.example.com': 'JSON Example' };
    const mockResponse = {
      created: 1,
      updated: 1,
      skipped: 0,
      errors: [],
    };

    mockedPost.mockResolvedValue({ data: mockResponse });

    const result = await commitJSONImport(sessionUUID, resolutions, names);

    expect(client.post).toHaveBeenCalledWith('/import/json/commit', {
      session_uuid: sessionUUID,
      resolutions,
      names,
    });
    expect(result).toEqual(mockResponse);
  });

  it('forwards uploadJSONExport errors', async () => {
    const error = new Error('upload failed');
    mockedPost.mockRejectedValue(error);

    await expect(uploadJSONExport('{"proxy_hosts":[]}')).rejects.toBe(error);
  });

  it('forwards commitJSONImport errors', async () => {
    const error = new Error('commit failed');
    mockedPost.mockRejectedValue(error);

    await expect(commitJSONImport('json-session-123', {}, {})).rejects.toBe(error);
  });

  it('forwards cancelJSONImport errors', async () => {
    const error = new Error('cancel failed');
    mockedPost.mockRejectedValue(error);

    await expect(cancelJSONImport('json-session-123')).rejects.toBe(error);
  });
});
