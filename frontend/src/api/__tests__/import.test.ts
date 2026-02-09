import { describe, it, expect, vi, beforeEach } from 'vitest';
import { uploadCaddyfile, uploadCaddyfilesMulti, getImportPreview, commitImport, cancelImport, getImportStatus } from '../import';
import client from '../client';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

describe('import API', () => {
  const mockedGet = vi.mocked(client.get);
  const mockedPost = vi.mocked(client.post);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('uploadCaddyfile posts content', async () => {
    const content = 'example.com';
    const mockResponse = { preview: { hosts: [] } };
    mockedPost.mockResolvedValue({ data: mockResponse });

    const result = await uploadCaddyfile(content);
    expect(client.post).toHaveBeenCalledWith('/import/upload', { content });
    expect(result).toEqual(mockResponse);
  });

  it('uploadCaddyfilesMulti posts files', async () => {
    const files = [{ filename: 'Caddyfile', content: 'foo.com' }];
    const mockResponse = { preview: { hosts: [] } };
    mockedPost.mockResolvedValue({ data: mockResponse });

    const result = await uploadCaddyfilesMulti(files);
    expect(client.post).toHaveBeenCalledWith('/import/upload-multi', { files });
    expect(result).toEqual(mockResponse);
  });

  it('getImportPreview gets preview', async () => {
    const mockResponse = { preview: { hosts: [] } };
    mockedGet.mockResolvedValue({ data: mockResponse });

    const result = await getImportPreview();
    expect(client.get).toHaveBeenCalledWith('/import/preview');
    expect(result).toEqual(mockResponse);
  });

  it('commitImport posts commitments', async () => {
    const sessionUUID = 'uuid-123';
    const resolutions = { 'foo.com': 'keep' };
    const names = { 'foo.com': 'My Site' };
    const mockResponse = { created: 1, updated: 0, skipped: 0, errors: [] };

    mockedPost.mockResolvedValue({ data: mockResponse });

    const result = await commitImport(sessionUUID, resolutions, names);
    expect(client.post).toHaveBeenCalledWith('/import/commit', {
      session_uuid: sessionUUID,
      resolutions,
      names
    });
    expect(result).toEqual(mockResponse);
  });

  it('cancelImport posts cancel', async () => {
    mockedPost.mockResolvedValue({});

    await cancelImport();
    expect(client.post).toHaveBeenCalledWith('/import/cancel');
  });

  it('getImportStatus gets status', async () => {
    const mockResponse = { has_pending: true };
    mockedGet.mockResolvedValue({ data: mockResponse });

    const result = await getImportStatus();
    expect(client.get).toHaveBeenCalledWith('/import/status');
    expect(result).toEqual(mockResponse);
  });

  it('getImportStatus handles error', async () => {
    mockedGet.mockRejectedValue(new Error('Failed'));

    const result = await getImportStatus();
    expect(result).toEqual({ has_pending: false });
  });
});
