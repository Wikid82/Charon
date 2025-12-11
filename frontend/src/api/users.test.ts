import { describe, it, expect, vi, beforeEach } from 'vitest'
import client from './client'
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
} from './users'

vi.mock('./client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

const mockedClient = client as unknown as {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
  put: ReturnType<typeof vi.fn>
  delete: ReturnType<typeof vi.fn>
}

describe('users api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('lists and fetches users', async () => {
    mockedClient.get
      .mockResolvedValueOnce({ data: [{ id: 1, uuid: 'u1', email: 'a@example.com', name: 'A', role: 'admin', enabled: true, permission_mode: 'allow_all', created_at: '', updated_at: '' }] })
      .mockResolvedValueOnce({ data: { id: 2, uuid: 'u2', email: 'b@example.com', name: 'B', role: 'user', enabled: true, permission_mode: 'allow_all', created_at: '', updated_at: '' } })

    const users = await listUsers()
    expect(mockedClient.get).toHaveBeenCalledWith('/users')
    expect(users[0].email).toBe('a@example.com')

    const user = await getUser(2)
    expect(mockedClient.get).toHaveBeenCalledWith('/users/2')
    expect(user.uuid).toBe('u2')
  })

  it('creates, invites, updates, and deletes users', async () => {
    mockedClient.post
      .mockResolvedValueOnce({ data: { id: 3, uuid: 'u3', email: 'c@example.com', name: 'C', role: 'user', enabled: true, permission_mode: 'allow_all', created_at: '', updated_at: '' } })
      .mockResolvedValueOnce({ data: { id: 4, uuid: 'u4', email: 'invite@example.com', role: 'user', invite_token: 'token', email_sent: true, expires_at: '' } })

    mockedClient.put.mockResolvedValueOnce({ data: { message: 'updated' } })
    mockedClient.delete.mockResolvedValueOnce({ data: { message: 'deleted' } })

    const created = await createUser({ email: 'c@example.com', name: 'C', password: 'pw' })
    expect(mockedClient.post).toHaveBeenCalledWith('/users', { email: 'c@example.com', name: 'C', password: 'pw' })
    expect(created.id).toBe(3)

    const invite = await inviteUser({ email: 'invite@example.com', role: 'user' })
    expect(mockedClient.post).toHaveBeenCalledWith('/users/invite', { email: 'invite@example.com', role: 'user' })
    expect(invite.invite_token).toBe('token')

    await updateUser(3, { enabled: false })
    expect(mockedClient.put).toHaveBeenCalledWith('/users/3', { enabled: false })

    await deleteUser(3)
    expect(mockedClient.delete).toHaveBeenCalledWith('/users/3')
  })

  it('updates permissions and validates/accepts invites', async () => {
    mockedClient.put.mockResolvedValueOnce({ data: { message: 'perms updated' } })
    mockedClient.get.mockResolvedValueOnce({ data: { valid: true, email: 'invite@example.com' } })
    mockedClient.post.mockResolvedValueOnce({ data: { message: 'accepted', email: 'invite@example.com' } })

    const perms = await updateUserPermissions(5, { permission_mode: 'deny_all', permitted_hosts: [1, 2] })
    expect(mockedClient.put).toHaveBeenCalledWith('/users/5/permissions', {
      permission_mode: 'deny_all',
      permitted_hosts: [1, 2],
    })
    expect(perms.message).toBe('perms updated')

    const validation = await validateInvite('token-abc')
    expect(mockedClient.get).toHaveBeenCalledWith('/invite/validate', { params: { token: 'token-abc' } })
    expect(validation.valid).toBe(true)

    const accept = await acceptInvite({ token: 'token-abc', name: 'New', password: 'pw' })
    expect(mockedClient.post).toHaveBeenCalledWith('/invite/accept', { token: 'token-abc', name: 'New', password: 'pw' })
    expect(accept.message).toBe('accepted')
  })
})
