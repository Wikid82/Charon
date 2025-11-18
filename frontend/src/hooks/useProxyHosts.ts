import { useState, useEffect } from 'react'
import { proxyHostsAPI } from '../services/api'

export interface ProxyHost {
  uuid: string
  domain_names: string
  forward_scheme: string
  forward_host: string
  forward_port: number
  access_list_id?: string
  certificate_id?: string
  ssl_forced: boolean
  http2_support: boolean
  hsts_enabled: boolean
  hsts_subdomains: boolean
  block_exploits: boolean
  websocket_support: boolean
  advanced_config?: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export function useProxyHosts() {
  const [hosts, setHosts] = useState<ProxyHost[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchHosts = async () => {
    try {
      setLoading(true)
      setError(null)
      const data = await proxyHostsAPI.list()
      setHosts(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch proxy hosts')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchHosts()
  }, [])

  const createHost = async (data: Partial<ProxyHost>) => {
    try {
      const newHost = await proxyHostsAPI.create(data)
      setHosts([...hosts, newHost])
      return newHost
    } catch (err) {
      throw new Error(err instanceof Error ? err.message : 'Failed to create proxy host')
    }
  }

  const updateHost = async (uuid: string, data: Partial<ProxyHost>) => {
    try {
      const updatedHost = await proxyHostsAPI.update(uuid, data)
      setHosts(hosts.map(h => h.uuid === uuid ? updatedHost : h))
      return updatedHost
    } catch (err) {
      throw new Error(err instanceof Error ? err.message : 'Failed to update proxy host')
    }
  }

  const deleteHost = async (uuid: string) => {
    try {
      await proxyHostsAPI.delete(uuid)
      setHosts(hosts.filter(h => h.uuid !== uuid))
    } catch (err) {
      throw new Error(err instanceof Error ? err.message : 'Failed to delete proxy host')
    }
  }

  return {
    hosts,
    loading,
    error,
    refresh: fetchHosts,
    createHost,
    updateHost,
    deleteHost,
  }
}
