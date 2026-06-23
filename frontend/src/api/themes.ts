import client from './client'
import type { CustomThemeColors, UserTheme } from '../context/ThemeContextValue'

// DTO as returned by the backend (colors is a JSON string, NOT a parsed object)
export interface UserThemeDTO {
  id: string
  name: string
  colors: string   // Raw JSON string — must be parsed before use
  created_at: string
  updated_at: string
}

export interface CreateThemePayload {
  name: string
  colors: CustomThemeColors
}

export interface UpdateThemePayload {
  name?: string
  colors?: CustomThemeColors
}

// Parse a backend DTO into a typed UserTheme.
// NOTE: JSON.parse is intentionally not wrapped in try/catch here.
// React Query's queryFn wrapper will catch any parse error and surface it as a
// query error state. Silent failure (returning a default) would hide data corruption.
export function parseUserThemeDTO(dto: UserThemeDTO): UserTheme {
  return {
    id: dto.id,
    name: dto.name,
    colors: JSON.parse(dto.colors) as CustomThemeColors,
    created_at: dto.created_at,
    updated_at: dto.updated_at,
  }
}

export const listUserThemes = async (): Promise<UserThemeDTO[]> => {
  const response = await client.get('/themes')
  return response.data
}

export const createUserTheme = async (payload: CreateThemePayload): Promise<UserThemeDTO> => {
  const response = await client.post('/themes', {
    name: payload.name,
    colors: JSON.stringify(payload.colors),
  })
  return response.data
}

export const updateUserTheme = async (id: string, payload: UpdateThemePayload): Promise<UserThemeDTO> => {
  const body: Record<string, unknown> = {}
  if (payload.name !== undefined) body.name = payload.name
  if (payload.colors !== undefined) body.colors = JSON.stringify(payload.colors)
  const response = await client.put(`/themes/${id}`, body)
  return response.data
}

export const deleteUserTheme = async (id: string): Promise<void> => {
  await client.delete(`/themes/${id}`)
}
