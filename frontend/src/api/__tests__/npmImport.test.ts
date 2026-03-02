import { describe, it, expect, vi, beforeEach } from 'vitest';
import { cancelNPMImport } from '../npmImport';
import client from '../client';

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

  it('forwards cancelNPMImport errors', async () => {
    const error = new Error('cancel failed');
    mockedPost.mockRejectedValue(error);

    await expect(cancelNPMImport('npm-session-123')).rejects.toBe(error);
  });
});
