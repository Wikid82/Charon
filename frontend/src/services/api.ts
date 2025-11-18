const API_BASE = '/api/v1'

interface RequestOptions {
  method?: string
  headers?: Record<string, string>
  body?: any
}

async function request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const url = `${API_BASE}${endpoint}`
  const config: RequestInit = {
    method: options.method || 'GET',
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  }

  if (options.body) {
    config.body = JSON.stringify(options.body)
  }

  const response = await fetch(url, config)

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText }))
    throw new Error(error.error || `HTTP ${response.status}`)
  }

  return response.json()
}

// Proxy Hosts API
export const proxyHostsAPI = {
  list: () => request<any[]>('/proxy-hosts'),
  get: (uuid: string) => request<any>(`/proxy-hosts/${uuid}`),
  create: (data: any) => request<any>('/proxy-hosts', { method: 'POST', body: data }),
  update: (uuid: string, data: any) => request<any>(`/proxy-hosts/${uuid}`, { method: 'PUT', body: data }),
  delete: (uuid: string) => request<void>(`/proxy-hosts/${uuid}`, { method: 'DELETE' }),
}

// Remote Servers API
export const remoteServersAPI = {
  list: (enabledOnly?: boolean) => {
    const query = enabledOnly ? '?enabled=true' : ''
    return request<any[]>(`/remote-servers${query}`)
  },
  get: (uuid: string) => request<any>(`/remote-servers/${uuid}`),
  create: (data: any) => request<any>('/remote-servers', { method: 'POST', body: data }),
  update: (uuid: string, data: any) => request<any>(`/remote-servers/${uuid}`, { method: 'PUT', body: data }),
  delete: (uuid: string) => request<void>(`/remote-servers/${uuid}`, { method: 'DELETE' }),
  test: (uuid: string) => request<any>(`/remote-servers/${uuid}/test`, { method: 'POST' }),
}

// Import API
export const importAPI = {
  status: () => request<{ has_pending: boolean; session?: any }>('/import/status'),
  preview: () => request<{ hosts: any[]; conflicts: string[]; errors: string[] }>('/import/preview'),
  upload: (content: string, filename?: string) => request<any>('/import/upload', {
    method: 'POST',
    body: { content, filename }
  }),
  commit: (sessionUUID: string, resolutions: Record<string, string>) => request<any>('/import/commit', {
    method: 'POST',
    body: { session_uuid: sessionUUID, resolutions }
  }),
  cancel: (sessionUUID: string) => request<void>(`/import/cancel?session_uuid=${sessionUUID}`, { method: 'DELETE' }),
}

// Health API
export const healthAPI = {
  check: () => request<{ status: string }>('/health'),
}
