import { QueryClient, type QueryKey } from '@tanstack/react-query'

interface InitialDataEntry {
  key: QueryKey
  data: unknown
}

export function createTestQueryClient(initialData: InitialDataEntry[] = []) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  })

  for (const { key, data } of initialData) client.setQueryData(key, data)
  return client
}
