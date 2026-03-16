import { describe, it, expect, vi, beforeEach } from 'vitest';

import client from '../client';
import { uploadCaddyfile, uploadCaddyfilesMulti, getImportPreview, commitImport, cancelImport, getImportStatus } from '../import';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('import API', () => {
  const mockedGet = vi.mocked(client.get);
  const mockedPost = vi.mocked(client.post);
  const mockedDelete = vi.mocked(client.delete);

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

  it('uploadCaddyfilesMulti accepts empty file arrays', async () => {
    mockedPost.mockResolvedValue({ data: { preview: { hosts: [], conflicts: [], errors: [] } } });

    const result = await uploadCaddyfilesMulti([]);
    expect(client.post).toHaveBeenCalledWith('/import/upload-multi', { files: [] });
    expect(result).toEqual({ preview: { hosts: [], conflicts: [], errors: [] } });
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

  it('cancelImport deletes cancel with required session_uuid query', async () => {
    const sessionUUID = 'uuid-cancel-123';
    mockedDelete.mockResolvedValue({});

    await cancelImport(sessionUUID);

    expect(client.delete).toHaveBeenCalledTimes(1);
    expect(client.delete).toHaveBeenCalledWith('/import/cancel', {
      params: {
        session_uuid: sessionUUID,
      },
    });

    const [, requestConfig] = mockedDelete.mock.calls[0];
    expect(requestConfig).toEqual({
      params: {
        session_uuid: sessionUUID,
      },
    });
  });

  it('forwards commitImport errors', async () => {
    const error = new Error('commit failed');
    mockedPost.mockRejectedValue(error);

    await expect(commitImport('uuid-123', {}, {})).rejects.toBe(error);
  });

  it('forwards cancelImport errors', async () => {
    const error = new Error('cancel failed');
    mockedDelete.mockRejectedValue(error);

    await expect(cancelImport('uuid-cancel-123')).rejects.toBe(error);
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

  it('getImportStatus returns false on non-Error rejections', async () => {
    mockedGet.mockRejectedValue('network down');

    const result = await getImportStatus();
    expect(result).toEqual({ has_pending: false });
  });
});
