import { useState } from 'react'

interface ImportReviewTableProps {
  hosts: any[]
  conflicts: string[]
  errors: string[]
  onCommit: (resolutions: Record<string, string>) => Promise<void>
  onCancel: () => void
}

export default function ImportReviewTable({ hosts, conflicts, errors, onCommit, onCancel }: ImportReviewTableProps) {
  const [resolutions, setResolutions] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const hasConflicts = conflicts.length > 0

  const handleResolutionChange = (domain: string, action: string) => {
    setResolutions({ ...resolutions, [domain]: action })
  }

  const handleCommit = async () => {
    // Ensure all conflicts have resolutions
    const unresolvedConflicts = conflicts.filter(c => !resolutions[c])
    if (unresolvedConflicts.length > 0) {
      alert(`Please resolve all conflicts: ${unresolvedConflicts.join(', ')}`)
      return
    }

    setLoading(true)
    try {
      await onCommit(resolutions)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      {/* Errors */}
      {errors.length > 0 && (
        <div className="bg-red-900/20 border border-red-500 rounded-lg p-4">
          <h3 className="text-lg font-semibold text-red-400 mb-2">Errors</h3>
          <ul className="list-disc list-inside space-y-1">
            {errors.map((error, idx) => (
              <li key={idx} className="text-sm text-red-300">{error}</li>
            ))}
          </ul>
        </div>
      )}

      {/* Conflicts */}
      {hasConflicts && (
        <div className="bg-yellow-900/20 border border-yellow-500 rounded-lg p-4">
          <h3 className="text-lg font-semibold text-yellow-400 mb-2">
            Conflicts Detected ({conflicts.length})
          </h3>
          <p className="text-sm text-gray-300 mb-4">
            The following domains already exist. Choose how to handle each conflict:
          </p>
          <div className="space-y-3">
            {conflicts.map((domain) => (
              <div key={domain} className="flex items-center justify-between bg-gray-900 p-3 rounded">
                <span className="text-white font-medium">{domain}</span>
                <select
                  value={resolutions[domain] || ''}
                  onChange={e => handleResolutionChange(domain, e.target.value)}
                  className="bg-gray-800 border border-gray-700 rounded px-3 py-1 text-white text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="">-- Choose action --</option>
                  <option value="skip">Skip (keep existing)</option>
                  <option value="overwrite">Overwrite existing</option>
                </select>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Preview Hosts */}
      <div className="bg-dark-card rounded-lg border border-gray-800 overflow-hidden">
        <div className="px-6 py-4 bg-gray-900 border-b border-gray-800">
          <h3 className="text-lg font-semibold text-white">
            Hosts to Import ({hosts.length})
          </h3>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-900 border-b border-gray-800">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                  Domain
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                  Forward To
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                  SSL
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                  Features
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {hosts.map((host, idx) => {
                const isConflict = conflicts.includes(host.domain_names)
                return (
                  <tr key={idx} className={`hover:bg-gray-900/50 ${isConflict ? 'bg-yellow-900/10' : ''}`}>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-white">{host.domain_names}</span>
                        {isConflict && (
                          <span className="px-2 py-1 text-xs bg-yellow-900/30 text-yellow-400 rounded">
                            Conflict
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-gray-300">
                        {host.forward_scheme}://{host.forward_host}:{host.forward_port}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      {host.ssl_forced && (
                        <span className="px-2 py-1 text-xs bg-green-900/30 text-green-400 rounded">
                          SSL
                        </span>
                      )}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex gap-2">
                        {host.http2_support && (
                          <span className="px-2 py-1 text-xs bg-blue-900/30 text-blue-400 rounded">
                            HTTP/2
                          </span>
                        )}
                        {host.websocket_support && (
                          <span className="px-2 py-1 text-xs bg-purple-900/30 text-purple-400 rounded">
                            WS
                          </span>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </div>

      {/* Actions */}
      <div className="flex gap-3 justify-end">
        <button
          onClick={onCancel}
          disabled={loading}
          className="px-6 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors disabled:opacity-50"
        >
          Cancel
        </button>
        <button
          onClick={handleCommit}
          disabled={loading || (hasConflicts && Object.keys(resolutions).length < conflicts.length)}
          className="px-6 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors disabled:opacity-50"
        >
          {loading ? 'Importing...' : 'Commit Import'}
        </button>
      </div>
    </div>
  )
}
