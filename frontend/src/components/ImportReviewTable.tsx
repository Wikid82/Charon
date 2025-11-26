import { useState } from 'react'
import { AlertTriangle, CheckCircle2 } from 'lucide-react'

interface HostPreview {
  domain_names: string
  name?: string
  forward_scheme?: string
  forward_host?: string
  forward_port?: number
  ssl_forced?: boolean
  websocket_support?: boolean
  [key: string]: unknown
}

interface ConflictDetail {
  existing: {
    forward_scheme: string
    forward_host: string
    forward_port: number
    ssl_forced: boolean
    websocket: boolean
    enabled: boolean
  }
  imported: {
    forward_scheme: string
    forward_host: string
    forward_port: number
    ssl_forced: boolean
    websocket: boolean
  }
}

interface Props {
  hosts: HostPreview[]
  conflicts: string[]
  conflictDetails?: Record<string, ConflictDetail>
  errors: string[]
  caddyfileContent?: string
  onCommit: (resolutions: Record<string, string>, names: Record<string, string>) => Promise<void>
  onCancel: () => void
}

export default function ImportReviewTable({ hosts, conflicts, conflictDetails, errors, caddyfileContent, onCommit, onCancel }: Props) {
  const [resolutions, setResolutions] = useState<Record<string, string>>(() => {
    const init: Record<string, string> = {}
    conflicts.forEach((d: string) => { init[d] = 'keep' })
    return init
  })
  const [names, setNames] = useState<Record<string, string>>(() => {
    const init: Record<string, string> = {}
    hosts.forEach((h) => {
      // Default name to domain name (first domain if comma-separated)
      init[h.domain_names] = h.name || h.domain_names.split(',')[0].trim()
    })
    return init
  })
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showSource, setShowSource] = useState(false)
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())

  const handleCommit = async () => {
    // Validate all names are filled
    const emptyNames = hosts.filter(h => !names[h.domain_names]?.trim())
    if (emptyNames.length > 0) {
      setError(`Please provide a name for all hosts. Missing: ${emptyNames.map(h => h.domain_names).join(', ')}`)
      return
    }

    setSubmitting(true)
    setError(null)
    try {
      await onCommit(resolutions, names)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to commit import')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="space-y-6">
      {caddyfileContent && (
        <div className="bg-dark-card rounded-lg border border-gray-800 overflow-hidden">
          <div className="p-4 border-b border-gray-800 flex items-center justify-between cursor-pointer" onClick={() => setShowSource(!showSource)}>
            <h2 className="text-lg font-semibold text-white">Source Caddyfile Content</h2>
            <span className="text-gray-400 text-sm">{showSource ? 'Hide' : 'Show'}</span>
          </div>
          {showSource && (
            <div className="p-4 bg-gray-900 overflow-x-auto">
              <pre className="text-xs text-gray-300 font-mono whitespace-pre-wrap">{caddyfileContent}</pre>
            </div>
          )}
        </div>
      )}

      <div className="bg-dark-card rounded-lg border border-gray-800 overflow-hidden">
        <div className="p-4 border-b border-gray-800 flex items-center justify-between">
          <h2 className="text-xl font-semibold text-white">Review Imported Hosts</h2>
          <div className="flex gap-3">
          <button
            onClick={onCancel}
            className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors"
          >
            Back
          </button>
          <button
            onClick={handleCommit}
            disabled={submitting}
            className="px-4 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors disabled:opacity-50"
          >
            {submitting ? 'Committing...' : 'Commit Import'}
          </button>
        </div>
      </div>

      {error && (
        <div className="m-4 bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded">
          {error}
        </div>
      )}

      {errors?.length > 0 && (
        <div className="m-4 bg-yellow-900/20 border border-yellow-600 text-yellow-300 px-4 py-3 rounded">
          <div className="font-medium mb-2">Issues found during parsing</div>
          <ul className="list-disc list-inside text-sm">
            {errors.map((e, i) => (
              <li key={i}>{e}</li>
            ))}
          </ul>
        </div>
      )}

      <div className="overflow-x-auto">
        <table className="w-full">
          <thead className="bg-gray-900 border-b border-gray-800">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                Name
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                Domain Names
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                Status
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                Conflict Resolution
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {hosts.map((h, idx) => {
              const domain = h.domain_names
              const hasConflict = conflicts.includes(domain)
              const isExpanded = expandedRows.has(domain)
              const details = conflictDetails?.[domain]

              return (
                <>
                  <tr key={`${domain}-${idx}`} className="hover:bg-gray-900/50">
                    <td className="px-6 py-4">
                      <input
                        type="text"
                        value={names[domain] || ''}
                        onChange={e => setNames({ ...names, [domain]: e.target.value })}
                        placeholder="Enter name"
                        className={`w-full bg-gray-900 border rounded px-3 py-1.5 text-sm text-white focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                          !names[domain]?.trim() ? 'border-red-500' : 'border-gray-700'
                        }`}
                      />
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        {hasConflict && (
                          <button
                            onClick={() => {
                              const newExpanded = new Set(expandedRows)
                              if (isExpanded) newExpanded.delete(domain)
                              else newExpanded.add(domain)
                              setExpandedRows(newExpanded)
                            }}
                            className="text-gray-400 hover:text-white"
                          >
                            {isExpanded ? '▼' : '▶'}
                          </button>
                        )}
                        <div className="text-sm font-medium text-white">{domain}</div>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      {hasConflict ? (
                        <span className="flex items-center gap-1 text-yellow-400 text-xs">
                          <AlertTriangle className="w-3 h-3" />
                          Conflict
                        </span>
                      ) : (
                        <span className="flex items-center gap-1 px-2 py-1 text-xs bg-green-900/30 text-green-400 rounded">
                          <CheckCircle2 className="w-3 h-3" />
                          New
                        </span>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      {hasConflict ? (
                        <select
                          value={resolutions[domain]}
                          onChange={e => setResolutions({ ...resolutions, [domain]: e.target.value })}
                          className="bg-gray-900 border border-gray-700 text-white rounded px-3 py-1.5 text-sm"
                        >
                          <option value="keep">Keep Existing (Skip Import)</option>
                          <option value="overwrite">Replace with Imported</option>
                        </select>
                      ) : (
                        <span className="text-gray-400 text-sm">Will be imported</span>
                      )}
                    </td>
                  </tr>

                  {hasConflict && isExpanded && details && (
                    <tr key={`${domain}-details`} className="bg-gray-900/30">
                      <td colSpan={4} className="px-6 py-4">
                        <div className="space-y-4">
                          <div className="grid grid-cols-2 gap-6">
                            {/* Existing Configuration */}
                            <div className="border border-blue-500/30 rounded-lg p-4 bg-blue-900/10">
                              <h4 className="text-sm font-semibold text-blue-400 mb-3 flex items-center gap-2">
                                <CheckCircle2 className="w-4 h-4" />
                                Current Configuration
                              </h4>
                              <dl className="space-y-2 text-sm">
                                <div className="flex justify-between">
                                  <dt className="text-gray-400">Target:</dt>
                                  <dd className="text-white font-mono">
                                    {details.existing.forward_scheme}://{details.existing.forward_host}:{details.existing.forward_port}
                                  </dd>
                                </div>
                                <div className="flex justify-between">
                                  <dt className="text-gray-400">SSL Forced:</dt>
                                  <dd className={details.existing.ssl_forced ? 'text-green-400' : 'text-gray-400'}>
                                    {details.existing.ssl_forced ? 'Yes' : 'No'}
                                  </dd>
                                </div>
                                <div className="flex justify-between">
                                  <dt className="text-gray-400">WebSocket:</dt>
                                  <dd className={details.existing.websocket ? 'text-green-400' : 'text-gray-400'}>
                                    {details.existing.websocket ? 'Enabled' : 'Disabled'}
                                  </dd>
                                </div>
                                <div className="flex justify-between">
                                  <dt className="text-gray-400">Status:</dt>
                                  <dd className={details.existing.enabled ? 'text-green-400' : 'text-red-400'}>
                                    {details.existing.enabled ? 'Enabled' : 'Disabled'}
                                  </dd>
                                </div>
                              </dl>
                            </div>

                            {/* Imported Configuration */}
                            <div className="border border-purple-500/30 rounded-lg p-4 bg-purple-900/10">
                              <h4 className="text-sm font-semibold text-purple-400 mb-3 flex items-center gap-2">
                                <AlertTriangle className="w-4 h-4" />
                                Imported Configuration
                              </h4>
                              <dl className="space-y-2 text-sm">
                                <div className="flex justify-between">
                                  <dt className="text-gray-400">Target:</dt>
                                  <dd className={`font-mono ${
                                    details.imported.forward_host !== details.existing.forward_host ||
                                    details.imported.forward_port !== details.existing.forward_port ||
                                    details.imported.forward_scheme !== details.existing.forward_scheme
                                      ? 'text-yellow-400 font-semibold'
                                      : 'text-white'
                                  }`}>
                                    {details.imported.forward_scheme}://{details.imported.forward_host}:{details.imported.forward_port}
                                  </dd>
                                </div>
                                <div className="flex justify-between">
                                  <dt className="text-gray-400">SSL Forced:</dt>
                                  <dd className={`${
                                    details.imported.ssl_forced !== details.existing.ssl_forced
                                      ? 'text-yellow-400 font-semibold'
                                      : details.imported.ssl_forced ? 'text-green-400' : 'text-gray-400'
                                  }`}>
                                    {details.imported.ssl_forced ? 'Yes' : 'No'}
                                  </dd>
                                </div>
                                <div className="flex justify-between">
                                  <dt className="text-gray-400">WebSocket:</dt>
                                  <dd className={`${
                                    details.imported.websocket !== details.existing.websocket
                                      ? 'text-yellow-400 font-semibold'
                                      : details.imported.websocket ? 'text-green-400' : 'text-gray-400'
                                  }`}>
                                    {details.imported.websocket ? 'Enabled' : 'Disabled'}
                                  </dd>
                                </div>
                                <div className="flex justify-between">
                                  <dt className="text-gray-400">Status:</dt>
                                  <dd className="text-gray-400">
                                    (Imported hosts are disabled by default)
                                  </dd>
                                </div>
                              </dl>
                            </div>
                          </div>

                          {/* Recommendation */}
                          <div className="bg-gray-800/50 rounded-lg p-3 border-l-4 border-blue-500">
                            <p className="text-sm text-gray-300">
                              <strong className="text-blue-400">💡 Recommendation:</strong>{' '}
                              {getRecommendation(details)}
                            </p>
                          </div>
                        </div>
                      </td>
                    </tr>
                  )}
                </>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
    </div>
  )
}

function getRecommendation(details: ConflictDetail): string {
  const hasTargetChange =
    details.imported.forward_host !== details.existing.forward_host ||
    details.imported.forward_port !== details.existing.forward_port ||
    details.imported.forward_scheme !== details.existing.forward_scheme

  const hasConfigChange =
    details.imported.ssl_forced !== details.existing.ssl_forced ||
    details.imported.websocket !== details.existing.websocket

  if (hasTargetChange) {
    return 'The imported configuration points to a different backend server. Choose "Replace" if you want to update the target, or "Keep Existing" if the current setup is correct.'
  }

  if (hasConfigChange) {
    return 'The imported configuration has different SSL or WebSocket settings. Choose "Replace" to update these settings, or "Keep Existing" to maintain current configuration.'
  }

  return 'The configurations are identical. You can safely keep the existing configuration.'
}
