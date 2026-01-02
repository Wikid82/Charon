interface Props {
  session: { id: string }
  onReview: () => void
  onCancel: () => void
}

export default function ImportBanner({ session, onReview, onCancel }: Props) {
  return (
    <div className="bg-yellow-900/20 border border-yellow-600 text-yellow-300 px-4 py-3 rounded mb-6 flex items-center justify-between">
      <div>
        <div className="font-medium">Pending Import Session</div>
        <div className="text-sm text-yellow-400/80">Session ID: {session.id}</div>
      </div>
      <div className="flex gap-3">
        <button
          onClick={onReview}
          className="px-3 py-1 bg-yellow-600 hover:bg-yellow-500 text-black rounded text-sm font-medium"
        >
          Review Changes
        </button>
        <button
          onClick={onCancel}
          className="px-3 py-1 bg-gray-800 hover:bg-gray-700 text-yellow-300 border border-yellow-700 rounded text-sm font-medium"
        >
          Cancel
        </button>
      </div>
    </div>
  )
}
