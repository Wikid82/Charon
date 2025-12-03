import type { ProxyHost } from '../api/proxyHosts'

export function formatSettingLabel(key: string) {
  switch (key) {
    case 'ssl_forced':
      return 'Force SSL'
    case 'http2_support':
      return 'HTTP/2 Support'
    case 'hsts_enabled':
      return 'HSTS Enabled'
    case 'hsts_subdomains':
      return 'HSTS Subdomains'
    case 'block_exploits':
      return 'Block Exploits'
    case 'websocket_support':
      return 'Websockets Support'
    default:
      return key
  }
}

export function settingHelpText(key: string) {
  switch (key) {
    case 'ssl_forced':
      return 'Redirect all HTTP traffic to HTTPS.'
    case 'http2_support':
      return 'Enable HTTP/2 for improved performance.'
    case 'hsts_enabled':
      return 'Send HSTS header to enforce HTTPS.'
    case 'hsts_subdomains':
      return 'Include subdomains in HSTS policy.'
    case 'block_exploits':
      return 'Add common exploit-mitigation headers and rules.'
    case 'websocket_support':
      return 'Enable websocket proxying support.'
    default:
      return ''
  }
}

export function settingKeyToField(key: string) {
  switch (key) {
    case 'ssl_forced':
      return 'ssl_forced'
    case 'http2_support':
      return 'http2_support'
    case 'hsts_enabled':
      return 'hsts_enabled'
    case 'hsts_subdomains':
      return 'hsts_subdomains'
    case 'block_exploits':
      return 'block_exploits'
    case 'websocket_support':
      return 'websocket_support'
    default:
      return key
  }
}

export async function applyBulkSettingsToHosts(options: {
  hosts: ProxyHost[]
  hostUUIDs: string[]
  keysToApply: string[]
  bulkApplySettings: Record<string, { apply: boolean; value: boolean }>
  updateHost: (uuid: string, data: Partial<ProxyHost>) => Promise<ProxyHost>
  setApplyProgress?: (p: { current: number; total: number } | null) => void
}) {
  const { hosts, hostUUIDs, keysToApply, bulkApplySettings, updateHost, setApplyProgress } = options
  let completed = 0
  let errors = 0
  setApplyProgress?.({ current: 0, total: hostUUIDs.length })

  for (const uuid of hostUUIDs) {
    const patch: Partial<ProxyHost> = {}
    for (const key of keysToApply) {
      const field = settingKeyToField(key) as keyof ProxyHost
      ;(patch as unknown as Record<string, unknown>)[field as string] = bulkApplySettings[key].value
    }

    const host = hosts.find(h => h.uuid === uuid)
    if (!host) {
      errors++
      completed++
      setApplyProgress?.({ current: completed, total: hostUUIDs.length })
      continue
    }

    const merged: Partial<ProxyHost> = { ...host, ...patch }
    try {
      await updateHost(uuid, merged)
    } catch {
      errors++
    }

    completed++
    setApplyProgress?.({ current: completed, total: hostUUIDs.length })
  }

  setApplyProgress?.(null)
  return { errors, completed }
}

export default {}
