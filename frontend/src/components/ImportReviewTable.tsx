import { useState } from 'react'

interface HostPreview {
  domain_names: string
  [key: string]: unknown
}

interface Props {
  hosts: HostPreview[]
  conflicts: string[]
  errors: string[]
  onCommit: (resolutions: Record<string, string>) => Promise<void>
  onCancel: () => void
}

export default function ImportReviewTable({ hosts, conflicts, errors, onCommit, onCancel }: Props) {
  const [resolutions, setResolutions] = useState<Record<string, string>>(() => {
    const init: Record<string, string> = {}
    conflicts.forEach((d: string) => { init[d] = 'keep' })
    return init
  })
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleCommit = async () => {
    setSubmitting(true)
    setError(null)
    try {
      await onCommit(resolutions)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to commit import')
    } finally {
      setSubmitting(false)
    }
  }

  return (
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
                Domain Names
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
              return (
                <tr key={`${domain}-${idx}`} className="hover:bg-gray-900/50">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm font-medium text-white">{domain}</div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    {hasConflict ? (
                      <select
                        value={resolutions[domain]}
                        onChange={e => setResolutions({ ...resolutions, [domain]: e.target.value })}
                        className="bg-gray-900 border border-gray-700 text-white rounded px-2 py-1"
                      >
                        <option value="keep">Keep Existing</option>
                        <option value="overwrite">Overwrite</option>
                        <option value="skip">Skip</option>
                      </select>
                    ) : (
                      <span className="px-2 py-1 text-xs bg-green-900/30 text-green-400 rounded">
                        No conflict
                      </span>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
