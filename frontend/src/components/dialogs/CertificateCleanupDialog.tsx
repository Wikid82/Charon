import { AlertTriangle } from 'lucide-react'

interface CertificateCleanupDialogProps {
  onConfirm: (deleteCerts: boolean) => void
  onCancel: () => void
  certificates: Array<{ id: number; name: string; domain: string }>
  hostNames: string[]
  isBulk?: boolean
}

export default function CertificateCleanupDialog({
  onConfirm,
  onCancel,
  certificates,
  hostNames,
  isBulk = false
}: CertificateCleanupDialogProps) {
  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    const deleteCerts = formData.get('delete_certs') === 'on'
    onConfirm(deleteCerts)
  }

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      onClick={onCancel}
    >
      <div
        className="bg-dark-card border border-orange-900/50 rounded-lg p-6 max-w-lg w-full mx-4"
        onClick={(e) => e.stopPropagation()}
      >
        <form onSubmit={handleSubmit}>
          <div className="flex items-start gap-3 mb-4">
            <div className="flex-shrink-0 w-10 h-10 rounded-full bg-orange-900/30 flex items-center justify-center">
              <AlertTriangle className="h-5 w-5 text-orange-400" />
            </div>
            <div className="flex-1">
              <h2 className="text-xl font-bold text-white">
                Delete {isBulk ? `${hostNames.length} Proxy Hosts` : 'Proxy Host'}?
              </h2>
              <p className="text-sm text-gray-400 mt-1">
                {isBulk ? 'These hosts will be permanently deleted.' : 'This host will be permanently deleted.'}
              </p>
            </div>
          </div>

          {/* Host names */}
          <div className="bg-gray-900/50 border border-gray-800 rounded-lg p-4 mb-4">
            <p className="text-xs font-medium text-gray-400 uppercase mb-2">
              {isBulk ? 'Hosts to be deleted:' : 'Host to be deleted:'}
            </p>
            <ul className="space-y-1 max-h-32 overflow-y-auto">
              {hostNames.map((name, idx) => (
                <li key={idx} className="text-sm text-white flex items-center gap-2">
                  <span className="text-red-400">•</span>
                  <span className="font-medium">{name}</span>
                </li>
              ))}
            </ul>
          </div>

          {/* Certificate cleanup option */}
          {certificates.length > 0 && (
            <div className="bg-orange-900/10 border border-orange-800/50 rounded-lg p-4 mb-4">
              <div className="flex items-start gap-3">
                <input
                  type="checkbox"
                  id="delete_certs"
                  name="delete_certs"
                  className="mt-1 w-4 h-4 rounded border-gray-600 text-orange-500 focus:ring-orange-500 focus:ring-offset-0 bg-gray-700"
                />
                <div className="flex-1">
                  <label htmlFor="delete_certs" className="text-sm text-orange-300 font-medium cursor-pointer">
                    Also delete {certificates.length === 1 ? 'orphaned certificate' : `${certificates.length} orphaned certificates`}
                  </label>
                  <p className="text-xs text-gray-400 mt-1">
                    {certificates.length === 1
                      ? 'This custom/staging certificate will no longer be used by any hosts.'
                      : 'These custom/staging certificates will no longer be used by any hosts.'}
                  </p>
                  <ul className="mt-2 space-y-1">
                    {certificates.map((cert) => (
                      <li key={cert.id} className="text-xs text-gray-300 flex items-center gap-2">
                        <span className="text-orange-400">→</span>
                        <span className="font-medium">{cert.name || cert.domain}</span>
                        <span className="text-gray-500">({cert.domain})</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            </div>
          )}

          {/* Confirmation buttons */}
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onCancel}
              className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg font-medium transition-colors"
            >
              Delete
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
