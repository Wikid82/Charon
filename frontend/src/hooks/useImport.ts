import { useState, useEffect, useCallback } from 'react'
import { importAPI } from '../services/api'

interface ImportSession {
  uuid: string
  filename?: string
  state: string
  created_at: string
  updated_at: string
}

interface ImportPreview {
  hosts: any[]
  conflicts: string[]
  errors: string[]
}

export function useImport() {
  const [session, setSession] = useState<ImportSession | null>(null)
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [polling, setPolling] = useState(false)

  const checkStatus = useCallback(async () => {
    try {
      const status = await importAPI.status()
      if (status.has_pending && status.session) {
        setSession(status.session)
        if (status.session.state === 'reviewing') {
          const previewData = await importAPI.preview()
          setPreview(previewData)
        }
      } else {
        setSession(null)
        setPreview(null)
      }
    } catch (err) {
      console.error('Failed to check import status:', err)
    }
  }, [])

  useEffect(() => {
    checkStatus()
  }, [checkStatus])

  useEffect(() => {
    if (polling && session?.state === 'reviewing') {
      const interval = setInterval(checkStatus, 3000)
      return () => clearInterval(interval)
    }
  }, [polling, session?.state, checkStatus])

  const upload = async (content: string, filename?: string) => {
    try {
      setLoading(true)
      setError(null)
      const result = await importAPI.upload(content, filename)
      setSession(result.session)
      setPolling(true)
      await checkStatus()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to upload Caddyfile')
      throw err
    } finally {
      setLoading(false)
    }
  }

  const commit = async (resolutions: Record<string, string>) => {
    if (!session) throw new Error('No active session')
    
    try {
      setLoading(true)
      setError(null)
      await importAPI.commit(session.uuid, resolutions)
      setSession(null)
      setPreview(null)
      setPolling(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to commit import')
      throw err
    } finally {
      setLoading(false)
    }
  }

  const cancel = async () => {
    if (!session) return
    
    try {
      setLoading(true)
      setError(null)
      await importAPI.cancel(session.uuid)
      setSession(null)
      setPreview(null)
      setPolling(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to cancel import')
      throw err
    } finally {
      setLoading(false)
    }
  }

  return {
    session,
    preview,
    loading,
    error,
    upload,
    commit,
    cancel,
    refresh: checkStatus,
  }
}
