interface ImportBannerProps {
  session: {
    uuid: string
    filename?: string
    state: string
    created_at: string
  }
  onReview: () => void
  onCancel: () => void
}

export default function ImportBanner({ session, onReview, onCancel }: ImportBannerProps) {
  return (
    <div className="bg-blue-900/20 border border-blue-500 rounded-lg p-4 mb-6">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-semibold text-blue-400 mb-1">
            Import Session Active
          </h3>
          <p className="text-sm text-gray-300">
            {session.filename && `File: ${session.filename} • `}
            State: <span className="font-medium">{session.state}</span>
          </p>
        </div>
        <div className="flex gap-3">
          {session.state === 'reviewing' && (
            <button
              onClick={onReview}
              className="px-4 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors"
            >
              Review Changes
            </button>
          )}
          <button
            onClick={onCancel}
            className="px-4 py-2 bg-red-900/20 hover:bg-red-900/30 text-red-400 rounded-lg font-medium transition-colors"
          >
            Cancel Import
          </button>
        </div>
      </div>
    </div>
  )
}
