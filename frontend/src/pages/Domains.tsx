import { useState } from 'react'
import { useDomains } from '../hooks/useDomains'
import { Trash2, Plus, Globe } from 'lucide-react'

export default function Domains() {
  const { domains, isLoading, error, createDomain, deleteDomain } = useDomains()
  const [newDomain, setNewDomain] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newDomain.trim()) return

    setIsSubmitting(true)
    try {
      await createDomain(newDomain)
      setNewDomain('')
    } catch (err) {
      alert('Failed to create domain')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleDelete = async (uuid: string) => {
    if (confirm('Are you sure you want to delete this domain?')) {
      try {
        await deleteDomain(uuid)
      } catch (err) {
        alert('Failed to delete domain')
      }
    }
  }

  if (isLoading) return <div className="p-8 text-white">Loading...</div>
  if (error) return <div className="p-8 text-red-400">Error loading domains</div>

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-3xl font-bold text-white">Domains</h1>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {/* Add New Domain Card */}
        <div className="bg-dark-card border border-gray-800 rounded-lg p-6">
          <h3 className="text-lg font-medium text-white mb-4 flex items-center gap-2">
            <Plus size={20} />
            Add Domain
          </h3>
          <form onSubmit={handleAdd} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-400 mb-1">
                Domain Name
              </label>
              <input
                type="text"
                value={newDomain}
                onChange={(e) => setNewDomain(e.target.value)}
                placeholder="example.com"
                className="w-full bg-gray-900 border border-gray-700 rounded px-3 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <button
              type="submit"
              disabled={isSubmitting || !newDomain.trim()}
              className="w-full bg-blue-600 hover:bg-blue-700 text-white rounded py-2 font-medium transition-colors disabled:opacity-50"
            >
              {isSubmitting ? 'Adding...' : 'Add Domain'}
            </button>
          </form>
        </div>

        {/* Domain List */}
        {domains.map((domain) => (
          <div key={domain.uuid} className="bg-dark-card border border-gray-800 rounded-lg p-6 flex flex-col justify-between">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-blue-900/30 rounded-lg text-blue-400">
                  <Globe size={24} />
                </div>
                <div>
                  <h3 className="text-lg font-medium text-white">{domain.name}</h3>
                  <p className="text-sm text-gray-500">
                    Added {new Date(domain.created_at).toLocaleDateString()}
                  </p>
                </div>
              </div>
              <button
                onClick={() => handleDelete(domain.uuid)}
                className="text-gray-500 hover:text-red-400 transition-colors"
                title="Delete Domain"
              >
                <Trash2 size={20} />
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
