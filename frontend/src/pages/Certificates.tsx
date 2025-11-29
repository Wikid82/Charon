import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, X } from 'lucide-react'
import CertificateList from '../components/CertificateList'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { uploadCertificate } from '../api/certificates'
import { toast } from '../utils/toast'

export default function Certificates() {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [name, setName] = useState('')
  const [certFile, setCertFile] = useState<File | null>(null)
  const [keyFile, setKeyFile] = useState<File | null>(null)
  const queryClient = useQueryClient()

  const uploadMutation = useMutation({
    mutationFn: async () => {
      if (!certFile || !keyFile) throw new Error('Files required')
      await uploadCertificate(name, certFile, keyFile)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['certificates'] })
      setIsModalOpen(false)
      setName('')
      setCertFile(null)
      setKeyFile(null)
      toast.success('Certificate uploaded successfully')
    },
    onError: (error: Error) => {
      toast.error(`Failed to upload certificate: ${error.message}`)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    uploadMutation.mutate()
  }

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white mb-2">Certificates</h1>
          <p className="text-gray-400">
            View and manage SSL certificates. Production Let's Encrypt certificates are auto-managed by Caddy.
          </p>
        </div>
        <Button onClick={() => setIsModalOpen(true)}>
          <Plus className="w-4 h-4 mr-2" />
          Add Certificate
        </Button>
      </div>

      <div className="mb-4 bg-blue-900/20 border border-blue-500/30 text-blue-300 px-4 py-3 rounded-lg text-sm">
        <strong>Note:</strong> You can delete custom certificates and staging certificates.
        Production Let's Encrypt certificates are automatically renewed and should not be deleted unless switching environments.
      </div>

      <CertificateList />

      {isModalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-900 border border-gray-800 rounded-lg p-6 w-full max-w-md">
            <div className="flex justify-between items-center mb-4">
              <h2 className="text-xl font-bold text-white">Upload Certificate</h2>
              <button onClick={() => setIsModalOpen(false)} className="text-gray-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleSubmit} className="space-y-4">
              <Input
                label="Friendly Name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. My Custom Cert"
                required
              />
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-1.5">
                  Certificate (PEM)
                </label>
                <input
                  type="file"
                  accept=".pem,.crt,.cer"
                  onChange={(e) => setCertFile(e.target.files?.[0] || null)}
                  className="block w-full text-sm text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-blue-600 file:text-white hover:file:bg-blue-700"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-1.5">
                  Private Key (PEM)
                </label>
                <input
                  type="file"
                  accept=".pem,.key"
                  onChange={(e) => setKeyFile(e.target.files?.[0] || null)}
                  className="block w-full text-sm text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-blue-600 file:text-white hover:file:bg-blue-700"
                  required
                />
              </div>
              <div className="flex justify-end gap-3 mt-6">
                <Button type="button" variant="secondary" onClick={() => setIsModalOpen(false)}>
                  Cancel
                </Button>
                <Button type="submit" isLoading={uploadMutation.isPending}>
                  Upload
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
