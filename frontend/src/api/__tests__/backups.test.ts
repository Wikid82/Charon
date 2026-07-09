import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../../api/client'
import {
  getBackups,
  createBackup,
  restoreBackup,
  deleteBackup,
  uploadBackup,
  validateBackup,
  getBackupSettings,
  updateBackupSettings,
  getRemoteTargets,
  createRemoteTarget,
  updateRemoteTarget,
  deleteRemoteTarget,
  testRemoteTarget,
  testDraftRemoteTarget,
} from '../backups'

describe('backups api', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('getBackups returns list', async () => {
    const mockData = [{ filename: 'b1.zip', size: 123, time: '2025-01-01T00:00:00Z' }]
    vi.spyOn(client, 'get').mockResolvedValueOnce({ data: mockData })
    const res = await getBackups()
    expect(res).toEqual(mockData)
  })

  it('createBackup returns filename', async () => {
    vi.spyOn(client, 'post').mockResolvedValueOnce({ data: { filename: 'b2.zip' } })
    const res = await createBackup()
    expect(res).toEqual({ filename: 'b2.zip' })
  })

  it('restoreBackup posts to restore endpoint', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({})
    await restoreBackup('b3.zip')
    expect(spy).toHaveBeenCalledWith('/backups/b3.zip/restore')
  })

  it('deleteBackup deletes backup', async () => {
    const spy = vi.spyOn(client, 'delete').mockResolvedValueOnce({})
    await deleteBackup('b3.zip')
    expect(spy).toHaveBeenCalledWith('/backups/b3.zip')
  })

  it('createBackup posts encrypt/passphrase options when provided', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({ data: { filename: 'b4.zip.age', uuid: 'u1' } })
    const res = await createBackup({ encrypt: true, passphrase: 'hunter2' })
    expect(spy).toHaveBeenCalledWith('/backups', { encrypt: true, passphrase: 'hunter2' })
    expect(res).toEqual({ filename: 'b4.zip.age', uuid: 'u1' })
  })

  it('restoreBackup includes the passphrase in the body when provided', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({ data: { message: 'ok', restart_required: false } })
    await restoreBackup('b3.zip.age', 'hunter2')
    expect(spy).toHaveBeenCalledWith('/backups/b3.zip.age/restore', { passphrase: 'hunter2' })
  })

  it('uploadBackup sends a multipart FormData request', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({
      data: { filename: 'uploaded_1.zip', uuid: 'u2', legacy_format: false, message: 'ok' },
    })
    const file = new File(['content'], 'backup.zip', { type: 'application/zip' })

    const res = await uploadBackup(file, 'hunter2')

    expect(spy).toHaveBeenCalledTimes(1)
    const [url, body, config] = spy.mock.calls[0]
    expect(url).toBe('/backups/upload')
    expect(body).toBeInstanceOf(FormData)
    expect((body as FormData).get('file')).toBe(file)
    expect((body as FormData).get('passphrase')).toBe('hunter2')
    expect(config).toEqual({ headers: { 'Content-Type': 'multipart/form-data' } })
    expect(res.filename).toBe('uploaded_1.zip')
  })

  it('uploadBackup omits the passphrase field when not provided', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({
      data: { filename: 'uploaded_2.zip', legacy_format: false, message: 'ok' },
    })
    const file = new File(['content'], 'backup.zip', { type: 'application/zip' })

    await uploadBackup(file)

    const [, body] = spy.mock.calls[0]
    expect((body as FormData).has('passphrase')).toBe(false)
  })

  it('validateBackup posts without a body when no passphrase is given', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({
      data: { valid: true, format_version: 2, legacy_format: false, database_integrity: 'ok', encryption_key_required: false },
    })
    const res = await validateBackup('b1.zip')
    expect(spy).toHaveBeenCalledWith('/backups/b1.zip/validate')
    expect(res.valid).toBe(true)
  })

  it('validateBackup posts the passphrase when given', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({
      data: { valid: true, format_version: 2, legacy_format: false, database_integrity: 'ok', encryption_key_required: false },
    })
    await validateBackup('b1.zip.age', 'hunter2')
    expect(spy).toHaveBeenCalledWith('/backups/b1.zip.age/validate', { passphrase: 'hunter2' })
  })

  it('getBackupSettings fetches the settings facade', async () => {
    const settings = {
      schedule_enabled: true,
      schedule_cron: '0 3 * * *',
      retention_count: 7,
      remote_retention_count: 7,
      encryption_enabled: false,
      encryption_passphrase_set: false,
    }
    vi.spyOn(client, 'get').mockResolvedValueOnce({ data: settings })
    const res = await getBackupSettings()
    expect(res).toEqual(settings)
  })

  it('updateBackupSettings PUTs the partial payload', async () => {
    const spy = vi.spyOn(client, 'put').mockResolvedValueOnce({ data: { schedule_cron: '0 4 * * *' } })
    await updateBackupSettings({ schedule_cron: '0 4 * * *' })
    expect(spy).toHaveBeenCalledWith('/backups/settings', { schedule_cron: '0 4 * * *' })
  })

  it('getRemoteTargets fetches the target list', async () => {
    vi.spyOn(client, 'get').mockResolvedValueOnce({ data: [] })
    const res = await getRemoteTargets()
    expect(res).toEqual([])
  })

  it('createRemoteTarget POSTs name/type/config/secrets', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({ data: { uuid: 'r1' } })
    const payload = {
      name: 'NAS',
      type: 'sftp' as const,
      config: { host: 'nas.lan', port: 22, path: '/backups', username: 'charon' },
      secrets: { password: 'hunter2' },
    }
    await createRemoteTarget(payload)
    expect(spy).toHaveBeenCalledWith('/backups/remote-targets', payload)
  })

  it('updateRemoteTarget PUTs to the target uuid', async () => {
    const spy = vi.spyOn(client, 'put').mockResolvedValueOnce({ data: { uuid: 'r1' } })
    await updateRemoteTarget('r1', { name: 'Renamed' })
    expect(spy).toHaveBeenCalledWith('/backups/remote-targets/r1', { name: 'Renamed' })
  })

  it('deleteRemoteTarget DELETEs the target uuid', async () => {
    const spy = vi.spyOn(client, 'delete').mockResolvedValueOnce({})
    await deleteRemoteTarget('r1')
    expect(spy).toHaveBeenCalledWith('/backups/remote-targets/r1')
  })

  it('testRemoteTarget POSTs to the test endpoint', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({ data: { success: true, message: 'ok' } })
    const res = await testRemoteTarget('r1')
    expect(spy).toHaveBeenCalledWith('/backups/remote-targets/r1/test')
    expect(res.success).toBe(true)
  })

  it('testDraftRemoteTarget POSTs the draft config to the test-draft endpoint', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({
      data: { success: true, message: 'Host key discovered', discovered_fingerprint: 'SHA256:abc' },
    })
    const payload = { type: 'sftp' as const, config: { host: 'nas.lan', port: 22, path: '/backups', username: 'charon' } }
    const res = await testDraftRemoteTarget(payload)
    expect(spy).toHaveBeenCalledWith('/backups/remote-targets/test-draft', payload)
    expect(res.discovered_fingerprint).toBe('SHA256:abc')
  })
})
