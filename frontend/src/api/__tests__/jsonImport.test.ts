import { describe, it, expect, vi, beforeEach } from 'vitest';
import { cancelJSONImport } from '../jsonImport';
import client from '../client';

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

  it('forwards cancelJSONImport errors', async () => {
    const error = new Error('cancel failed');
    mockedPost.mockRejectedValue(error);

    await expect(cancelJSONImport('json-session-123')).rejects.toBe(error);
  });
});
