import { CheckCircle, Plus, RefreshCw, SkipForward, AlertCircle, Info } from 'lucide-react'

export interface ImportSuccessModalProps {
  visible: boolean
  onClose: () => void
  onNavigateDashboard: () => void
  onNavigateHosts: () => void
  results: {
    created: number
    updated: number
    skipped: number
    errors: string[]
  } | null
}

export default function ImportSuccessModal({
  visible,
  onClose,
  onNavigateDashboard,
  onNavigateHosts,
  results,
}: ImportSuccessModalProps) {
  if (!visible || !results) return null

  const { created, updated, skipped, errors } = results
  const hasErrors = errors.length > 0
  const totalProcessed = created + updated + skipped

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center" data-testid="import-success-modal">
      <div className="absolute inset-0 bg-black/60" onClick={onClose} />
      <div className="relative bg-dark-card rounded-lg p-6 w-125 max-w-full mx-4 border border-gray-800">
        {/* Header */}
        <div className="flex items-center gap-3 mb-6">
          <div className="shrink-0 w-12 h-12 rounded-full bg-green-900/30 flex items-center justify-center">
            <CheckCircle className="h-6 w-6 text-green-400" />
          </div>
          <div>
            <h2 className="text-xl font-bold text-white">Import Completed</h2>
            <p className="text-sm text-gray-400">
              {totalProcessed} host{totalProcessed !== 1 ? 's' : ''} processed
            </p>
          </div>
        </div>

        {/* Results Summary */}
        <div className="bg-gray-900/50 border border-gray-800 rounded-lg p-4 mb-4 space-y-3">
          {created > 0 && (
            <div className="flex items-center gap-3">
              <Plus className="h-4 w-4 text-green-400" />
              <span className="text-sm text-white">
                <span className="font-medium text-green-400">{created}</span> host{created !== 1 ? 's' : ''} created
              </span>
            </div>
          )}
          {updated > 0 && (
            <div className="flex items-center gap-3">
              <RefreshCw className="h-4 w-4 text-blue-400" />
              <span className="text-sm text-white">
                <span className="font-medium text-blue-400">{updated}</span> host{updated !== 1 ? 's' : ''} updated
              </span>
            </div>
          )}
          {skipped > 0 && (
            <div className="flex items-center gap-3">
              <SkipForward className="h-4 w-4 text-gray-400" />
              <span className="text-sm text-white">
                <span className="font-medium text-gray-400">{skipped}</span> host{skipped !== 1 ? 's' : ''} skipped
              </span>
            </div>
          )}
          {totalProcessed === 0 && (
            <div className="flex items-center gap-3">
              <Info className="h-4 w-4 text-gray-400" />
              <span className="text-sm text-gray-400">No hosts were processed</span>
            </div>
          )}
        </div>

        {/* Errors Section */}
        {hasErrors && (
          <div className="bg-red-900/20 border border-red-800/50 rounded-lg p-4 mb-4">
            <div className="flex items-center gap-2 mb-2">
              <AlertCircle className="h-4 w-4 text-red-400" />
              <span className="text-sm font-medium text-red-400">
                {errors.length} error{errors.length !== 1 ? 's' : ''} encountered
              </span>
            </div>
            <ul className="space-y-1 max-h-24 overflow-y-auto">
              {errors.map((error, idx) => (
                <li key={idx} className="text-xs text-red-300 flex items-start gap-2">
                  <span className="text-red-500 mt-0.5">•</span>
                  <span>{error}</span>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Certificate Provisioning Info */}
        {created > 0 && (
          <div className="bg-blue-900/20 border border-blue-800/50 rounded-lg p-4 mb-6">
            <div className="flex items-start gap-3">
              <Info className="h-4 w-4 text-blue-400 mt-0.5 shrink-0" />
              <div>
                <p className="text-sm font-medium text-blue-300">Certificate Provisioning</p>
                <p className="text-xs text-gray-400 mt-1">
                  SSL certificates will be automatically provisioned by Let's Encrypt.
                  This typically takes 1-5 minutes per domain.
                </p>
                <p className="text-xs text-gray-400 mt-2">
                  Monitor the <span className="text-blue-400">Dashboard</span> to track certificate provisioning progress.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Action Buttons */}
        <div className="flex flex-wrap gap-3 justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors"
          >
            Close
          </button>
          <button
            onClick={onNavigateHosts}
            className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-white rounded-lg font-medium transition-colors border border-gray-700"
          >
            View Proxy Hosts
          </button>
          <button
            onClick={onNavigateDashboard}
            className="px-4 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors"
          >
            Go to Dashboard
          </button>
        </div>
      </div>
    </div>
  )
}
