import { describe, it, expect, vi, beforeEach } from 'vitest'
import client from '../client'
import {
  listUsers,
  getUser,
  createUser,
  inviteUser,
  updateUser,
  deleteUser,
  updateUserPermissions,
  validateInvite,
  acceptInvite,
} from '../users'

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('users api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('lists, reads, creates, updates, and deletes users', async () => {
    vi.mocked(client.get).mockResolvedValueOnce({ data: [{ id: 1, email: 'a' }] })
    const users = await listUsers()
    expect(users[0].id).toBe(1)
    expect(client.get).toHaveBeenCalledWith('/users')

    vi.mocked(client.get).mockResolvedValueOnce({ data: { id: 2 } })
    await getUser(2)
    expect(client.get).toHaveBeenCalledWith('/users/2')

    vi.mocked(client.post).mockResolvedValueOnce({ data: { id: 3 } })
    await createUser({ email: 'e', name: 'n', password: 'p' })
    expect(client.post).toHaveBeenCalledWith('/users', { email: 'e', name: 'n', password: 'p' })

    vi.mocked(client.put).mockResolvedValueOnce({ data: { message: 'ok' } })
    await updateUser(2, { enabled: false })
    expect(client.put).toHaveBeenCalledWith('/users/2', { enabled: false })

    vi.mocked(client.delete).mockResolvedValueOnce({ data: { message: 'deleted' } })
    await deleteUser(2)
    expect(client.delete).toHaveBeenCalledWith('/users/2')
  })

  it('invites users and updates permissions', async () => {
    vi.mocked(client.post).mockResolvedValueOnce({ data: { invite_token: 't' } })
    await inviteUser({ email: 'i', permission_mode: 'allow_all' })
    expect(client.post).toHaveBeenCalledWith('/users/invite', { email: 'i', permission_mode: 'allow_all' })

    vi.mocked(client.put).mockResolvedValueOnce({ data: { message: 'saved' } })
    await updateUserPermissions(1, { permission_mode: 'deny_all', permitted_hosts: [1, 2] })
    expect(client.put).toHaveBeenCalledWith('/users/1/permissions', { permission_mode: 'deny_all', permitted_hosts: [1, 2] })
  })

  it('validates and accepts invites with params', async () => {
    vi.mocked(client.get).mockResolvedValueOnce({ data: { valid: true, email: 'a' } })
    await validateInvite('token-1')
    expect(client.get).toHaveBeenCalledWith('/invite/validate', { params: { token: 'token-1' } })

    vi.mocked(client.post).mockResolvedValueOnce({ data: { message: 'accepted', email: 'a' } })
    await acceptInvite({ token: 't', name: 'n', password: 'p' })
    expect(client.post).toHaveBeenCalledWith('/invite/accept', { token: 't', name: 'n', password: 'p' })
  })
})
