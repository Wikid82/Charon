import client from './client'

export type PermissionMode = 'allow_all' | 'deny_all'

export interface User {
  id: number
  uuid: string
  email: string
  name: string
  role: 'admin' | 'user' | 'viewer'
  enabled: boolean
  last_login?: string
  invite_status?: 'pending' | 'accepted' | 'expired'
  invited_at?: string
  permission_mode: PermissionMode
  permitted_hosts?: number[]
  created_at: string
  updated_at: string
}

export interface CreateUserRequest {
  email: string
  name: string
  password: string
  role?: string
  permission_mode?: PermissionMode
  permitted_hosts?: number[]
}

export interface InviteUserRequest {
  email: string
  role?: string
  permission_mode?: PermissionMode
  permitted_hosts?: number[]
}

export interface InviteUserResponse {
  id: number
  uuid: string
  email: string
  role: string
  invite_token: string
  email_sent: boolean
  expires_at: string
}

export interface UpdateUserRequest {
  name?: string
  email?: string
  role?: string
  enabled?: boolean
}

export interface UpdateUserPermissionsRequest {
  permission_mode: PermissionMode
  permitted_hosts: number[]
}

export interface ValidateInviteResponse {
  valid: boolean
  email: string
}

export interface AcceptInviteRequest {
  token: string
  name: string
  password: string
}

export const listUsers = async (): Promise<User[]> => {
  const response = await client.get<User[]>('/users')
  return response.data
}

export const getUser = async (id: number): Promise<User> => {
  const response = await client.get<User>(`/users/${id}`)
  return response.data
}

export const createUser = async (data: CreateUserRequest): Promise<User> => {
  const response = await client.post<User>('/users', data)
  return response.data
}

export const inviteUser = async (data: InviteUserRequest): Promise<InviteUserResponse> => {
  const response = await client.post<InviteUserResponse>('/users/invite', data)
  return response.data
}

export const updateUser = async (id: number, data: UpdateUserRequest): Promise<{ message: string }> => {
  const response = await client.put<{ message: string }>(`/users/${id}`, data)
  return response.data
}

export const deleteUser = async (id: number): Promise<{ message: string }> => {
  const response = await client.delete<{ message: string }>(`/users/${id}`)
  return response.data
}

export const updateUserPermissions = async (
  id: number,
  data: UpdateUserPermissionsRequest
): Promise<{ message: string }> => {
  const response = await client.put<{ message: string }>(`/users/${id}/permissions`, data)
  return response.data
}

// Public endpoints (no auth required)
export const validateInvite = async (token: string): Promise<ValidateInviteResponse> => {
  const response = await client.get<ValidateInviteResponse>('/invite/validate', {
    params: { token }
  })
  return response.data
}

export const acceptInvite = async (data: AcceptInviteRequest): Promise<{ message: string; email: string }> => {
  const response = await client.post<{ message: string; email: string }>('/invite/accept', data)
  return response.data
}
