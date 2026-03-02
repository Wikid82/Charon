import client from './client'

/** User permission mode type. */
export type PermissionMode = 'allow_all' | 'deny_all'

/** User account information. */
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

/** Request payload for creating a user. */
export interface CreateUserRequest {
  email: string
  name: string
  password: string
  role?: string
  permission_mode?: PermissionMode
  permitted_hosts?: number[]
}

/** Request payload for inviting a user. */
export interface InviteUserRequest {
  email: string
  role?: string
  permission_mode?: PermissionMode
  permitted_hosts?: number[]
}

/** Response from user invitation. */
export interface InviteUserResponse {
  id: number
  uuid: string
  email: string
  role: string
  invite_token_masked: string
  invite_url?: string
  email_sent: boolean
  expires_at: string
}

/** Request payload for updating a user. */
export interface UpdateUserRequest {
  name?: string
  email?: string
  role?: string
  enabled?: boolean
}

/** Request payload for updating user permissions. */
export interface UpdateUserPermissionsRequest {
  permission_mode: PermissionMode
  permitted_hosts: number[]
}

/** Response from invite validation. */
export interface ValidateInviteResponse {
  valid: boolean
  email: string
}

/** Request payload for accepting an invitation. */
export interface AcceptInviteRequest {
  token: string
  name: string
  password: string
}

/**
 * Lists all users.
 * @returns Promise resolving to array of User objects
 * @throws {AxiosError} If the request fails
 */
export const listUsers = async (): Promise<User[]> => {
  const response = await client.get<User[]>('/users')
  return response.data
}

/**
 * Fetches a single user by ID.
 * @param id - The user ID
 * @returns Promise resolving to the User object
 * @throws {AxiosError} If the request fails or user not found
 */
export const getUser = async (id: number): Promise<User> => {
  const response = await client.get<User>(`/users/${id}`)
  return response.data
}

/**
 * Creates a new user.
 * @param data - CreateUserRequest with user details
 * @returns Promise resolving to the created User
 * @throws {AxiosError} If creation fails or email already exists
 */
export const createUser = async (data: CreateUserRequest): Promise<User> => {
  const response = await client.post<User>('/users', data)
  return response.data
}

/**
 * Invites a new user via email.
 * @param data - InviteUserRequest with invitation details
 * @returns Promise resolving to InviteUserResponse with token
 * @throws {AxiosError} If invitation fails
 */
export const inviteUser = async (data: InviteUserRequest): Promise<InviteUserResponse> => {
  const response = await client.post<InviteUserResponse>('/users/invite', data)
  return response.data
}

/**
 * Updates an existing user.
 * @param id - The user ID to update
 * @param data - UpdateUserRequest with fields to update
 * @returns Promise resolving to success message
 * @throws {AxiosError} If update fails or user not found
 */
export const updateUser = async (id: number, data: UpdateUserRequest): Promise<{ message: string }> => {
  const response = await client.put<{ message: string }>(`/users/${id}`, data)
  return response.data
}

/**
 * Deletes a user.
 * @param id - The user ID to delete
 * @returns Promise resolving to success message
 * @throws {AxiosError} If deletion fails or user not found
 */
export const deleteUser = async (id: number): Promise<{ message: string }> => {
  const response = await client.delete<{ message: string }>(`/users/${id}`)
  return response.data
}

/**
 * Updates a user's permissions.
 * @param id - The user ID to update
 * @param data - UpdateUserPermissionsRequest with new permissions
 * @returns Promise resolving to success message
 * @throws {AxiosError} If update fails or user not found
 */
export const updateUserPermissions = async (
  id: number,
  data: UpdateUserPermissionsRequest
): Promise<{ message: string }> => {
  const response = await client.put<{ message: string }>(`/users/${id}/permissions`, data)
  return response.data
}

// Public endpoints (no auth required)
/**
 * Validates an invitation token.
 * @param token - The invitation token to validate
 * @returns Promise resolving to ValidateInviteResponse
 * @throws {AxiosError} If validation fails
 */
export const validateInvite = async (token: string): Promise<ValidateInviteResponse> => {
  const response = await client.get<ValidateInviteResponse>('/invite/validate', {
    params: { token }
  })
  return response.data
}

/**
 * Accepts an invitation and creates the user account.
 * @param data - AcceptInviteRequest with token and user details
 * @returns Promise resolving to success message and email
 * @throws {AxiosError} If acceptance fails or token invalid/expired
 */
export const acceptInvite = async (data: AcceptInviteRequest): Promise<{ message: string; email: string }> => {
  const response = await client.post<{ message: string; email: string }>('/invite/accept', data)
  return response.data
}

/** Response from invite URL preview. */
export interface PreviewInviteURLResponse {
  preview_url: string
  base_url: string
  is_configured: boolean
  email: string
  warning: boolean
  warning_message: string
}

/**
 * Previews what the invite URL will look like for a given email.
 * @param email - The email to preview
 * @returns Promise resolving to PreviewInviteURLResponse
 */
export const previewInviteURL = async (email: string): Promise<PreviewInviteURLResponse> => {
  const response = await client.post<PreviewInviteURLResponse>('/users/preview-invite-url', { email })
  return response.data
}

/**
 * Resends an invitation email to a pending user.
 * @param id - The user ID to resend invite to
 * @returns Promise resolving to InviteUserResponse with new token
 */
export const resendInvite = async (id: number): Promise<InviteUserResponse> => {
  const response = await client.post<InviteUserResponse>(`/users/${id}/resend-invite`)
  return response.data
}
