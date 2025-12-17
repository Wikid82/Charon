import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, ShieldCheck } from 'lucide-react'
import CertificateList from '../components/CertificateList'
import { uploadCertificate } from '../api/certificates'
import { toast } from '../utils/toast'
import { PageShell } from '../components/layout/PageShell'
import {
  Button,
  Input,
  Alert,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Label,
} from '../components/ui'

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

  // Header actions
  const headerActions = (
    <Button onClick={() => setIsModalOpen(true)}>
      <Plus className="w-4 h-4 mr-2" />
      Add Certificate
    </Button>
  )

  return (
    <PageShell
      title="SSL Certificates"
      description="Manage SSL/TLS certificates for your proxy hosts"
      actions={headerActions}
    >
      <Alert variant="info" icon={ShieldCheck}>
        <strong>Note:</strong> You can delete custom certificates and staging certificates.
        Production Let&apos;s Encrypt certificates are automatically renewed and should not be deleted unless switching environments.
      </Alert>

      <CertificateList />

      {/* Upload Certificate Dialog */}
      <Dialog open={isModalOpen} onOpenChange={setIsModalOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Upload Certificate</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit} className="space-y-4 py-4">
            <Input
              label="Friendly Name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. My Custom Cert"
              required
            />
            <div>
              <Label htmlFor="cert-file">Certificate (PEM)</Label>
              <input
                id="cert-file"
                type="file"
                accept=".pem,.crt,.cer"
                onChange={(e) => setCertFile(e.target.files?.[0] || null)}
                className="mt-1.5 block w-full text-sm text-content-secondary file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-brand-500 file:text-white hover:file:bg-brand-600 cursor-pointer"
                required
              />
            </div>
            <div>
              <Label htmlFor="key-file">Private Key (PEM)</Label>
              <input
                id="key-file"
                type="file"
                accept=".pem,.key"
                onChange={(e) => setKeyFile(e.target.files?.[0] || null)}
                className="mt-1.5 block w-full text-sm text-content-secondary file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-brand-500 file:text-white hover:file:bg-brand-600 cursor-pointer"
                required
              />
            </div>
            <DialogFooter className="pt-4">
              <Button type="button" variant="secondary" onClick={() => setIsModalOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" isLoading={uploadMutation.isPending}>
                Upload
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </PageShell>
  )
}
