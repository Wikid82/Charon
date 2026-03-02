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
    vi.mocked(client.post).mockResolvedValueOnce({ data: { invite_token_masked: '********', invite_url: '[REDACTED]' } })
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

  describe('previewInviteURL', () => {
    it('should call POST /users/preview-invite-url with email', async () => {
      const mockResponse = {
        preview_url: 'https://example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW',
        base_url: 'https://example.com',
        is_configured: true,
        email: 'test@example.com',
        warning: false,
        warning_message: ''
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await import('../users').then(m => m.previewInviteURL('test@example.com'))

      expect(client.post).toHaveBeenCalledWith('/users/preview-invite-url', { email: 'test@example.com' })
      expect(result).toEqual(mockResponse)
    })

    it('should return complete PreviewInviteURLResponse structure', async () => {
      const mockResponse = {
        preview_url: 'https://charon.example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW',
        base_url: 'https://charon.example.com',
        is_configured: true,
        email: 'user@test.com',
        warning: false,
        warning_message: ''
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await import('../users').then(m => m.previewInviteURL('user@test.com'))

      expect(result.preview_url).toBeDefined()
      expect(result.base_url).toBeDefined()
      expect(result.is_configured).toBeDefined()
      expect(result.email).toBeDefined()
      expect(result.warning).toBeDefined()
      expect(result.warning_message).toBeDefined()
    })

    it('should return preview_url with sample token', async () => {
      vi.mocked(client.post).mockResolvedValue({
        data: {
          preview_url: 'http://localhost:8080/accept-invite?token=SAMPLE_TOKEN_PREVIEW',
          base_url: 'http://localhost:8080',
          is_configured: false,
          email: 'test@example.com',
          warning: true,
          warning_message: 'Public URL not configured'
        }
      })

      const result = await import('../users').then(m => m.previewInviteURL('test@example.com'))

      expect(result.preview_url).toContain('SAMPLE_TOKEN_PREVIEW')
    })

    it('should return is_configured flag', async () => {
      vi.mocked(client.post).mockResolvedValue({
        data: {
          preview_url: 'https://example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW',
          base_url: 'https://example.com',
          is_configured: true,
          email: 'test@example.com',
          warning: false,
          warning_message: ''
        }
      })

      const result = await import('../users').then(m => m.previewInviteURL('test@example.com'))

      expect(result.is_configured).toBe(true)
    })

    it('should return warning flag when public URL not configured', async () => {
      vi.mocked(client.post).mockResolvedValue({
        data: {
          preview_url: 'http://localhost:8080/accept-invite?token=SAMPLE_TOKEN_PREVIEW',
          base_url: 'http://localhost:8080',
          is_configured: false,
          email: 'admin@test.com',
          warning: true,
          warning_message: 'Using default localhost URL'
        }
      })

      const result = await import('../users').then(m => m.previewInviteURL('admin@test.com'))

      expect(result.warning).toBe(true)
      expect(result.warning_message).toBe('Using default localhost URL')
    })

    it('should return the provided email in response', async () => {
      const testEmail = 'specific@email.com'
      vi.mocked(client.post).mockResolvedValue({
        data: {
          preview_url: 'https://example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW',
          base_url: 'https://example.com',
          is_configured: true,
          email: testEmail,
          warning: false,
          warning_message: ''
        }
      })

      const result = await import('../users').then(m => m.previewInviteURL(testEmail))

      expect(result.email).toBe(testEmail)
    })

    it('should handle request errors', async () => {
      vi.mocked(client.post).mockRejectedValue(new Error('Network error'))

      await expect(
        import('../users').then(m => m.previewInviteURL('test@example.com'))
      ).rejects.toThrow('Network error')
    })
  })
})
