import { QueryClient, QueryKey } from '@tanstack/react-query'

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

  initialData.forEach(({ key, data }) => client.setQueryData(key, data))
  return client
}
