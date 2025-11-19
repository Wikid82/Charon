import { useState, useEffect } from 'react'
import { remoteServersAPI } from '../services/api'

export interface RemoteServer {
  uuid: string
  name: string
  provider: string
  host: string
  port: number
  username?: string
  enabled: boolean
  reachable: boolean
  last_check?: string
  created_at: string
  updated_at: string
}

export function useRemoteServers() {
  const [servers, setServers] = useState<RemoteServer[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchServers = async (enabledOnly = false) => {
    try {
      setLoading(true)
      setError(null)
      const data = await remoteServersAPI.list(enabledOnly)
      setServers(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch remote servers')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchServers()
  }, [])

  const createServer = async (data: Partial<RemoteServer>) => {
    try {
      const newServer = await remoteServersAPI.create(data)
      setServers([...servers, newServer])
      return newServer
    } catch (err) {
      throw new Error(err instanceof Error ? err.message : 'Failed to create remote server')
    }
  }

  const updateServer = async (uuid: string, data: Partial<RemoteServer>) => {
    try {
      const updatedServer = await remoteServersAPI.update(uuid, data)
      setServers(servers.map(s => s.uuid === uuid ? updatedServer : s))
      return updatedServer
    } catch (err) {
      throw new Error(err instanceof Error ? err.message : 'Failed to update remote server')
    }
  }

  const deleteServer = async (uuid: string) => {
    try {
      await remoteServersAPI.delete(uuid)
      setServers(servers.filter(s => s.uuid !== uuid))
    } catch (err) {
      throw new Error(err instanceof Error ? err.message : 'Failed to delete remote server')
    }
  }

  const testConnection = async (uuid: string) => {
    try {
      return await remoteServersAPI.test(uuid)
    } catch (err) {
      throw new Error(err instanceof Error ? err.message : 'Failed to test connection')
    }
  }

  const enabledServers = servers.filter(s => s.enabled)

  return {
    servers,
    enabledServers,
    loading,
    error,
    refresh: fetchServers,
    createServer,
    updateServer,
    deleteServer,
    testConnection,
  }
}
