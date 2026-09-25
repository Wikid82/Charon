import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  listUserThemes,
  createUserTheme,
  updateUserTheme,
  deleteUserTheme,
  parseUserThemeDTO,
} from '../api/themes'
import { useAuth } from './useAuth'
import type { UserTheme, CustomThemeColors } from '../context/ThemeContextValue'

export function useUserThemes() {
  const queryClient = useQueryClient()
  // /themes is behind auth; never fire it for an anonymous visitor (e.g. the login page)
  const { isAuthenticated } = useAuth()

  const { data: userThemes = [], isLoading, error } = useQuery({
    queryKey: ['user-themes'],
    queryFn: async (): Promise<UserTheme[]> => {
      const dtos = await listUserThemes()
      return dtos.map(parseUserThemeDTO)
    },
    staleTime: 1000 * 60 * 5,  // 5 minutes
    enabled: isAuthenticated,
  })

  const createMutation = useMutation({
    mutationFn: createUserTheme,
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['user-themes'] }),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: { name?: string; colors?: CustomThemeColors } }) =>
      updateUserTheme(id, payload),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['user-themes'] }),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteUserTheme,
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['user-themes'] }),
  })

  return {
    userThemes,
    isLoading,
    error,
    createTheme: (name: string, colors: CustomThemeColors) =>
      createMutation.mutateAsync({ name, colors }),
    updateTheme: (id: string, payload: { name?: string; colors?: CustomThemeColors }) =>
      updateMutation.mutateAsync({ id, payload }),
    deleteTheme: (id: string) => deleteMutation.mutateAsync(id),
    isCreating: createMutation.isPending,
    isUpdating: updateMutation.isPending,
    isDeleting: deleteMutation.isPending,
  }
}
