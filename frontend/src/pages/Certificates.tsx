import CertificateList from '../components/CertificateList'

export default function Certificates() {
  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white mb-2">Certificates</h1>
          <p className="text-gray-400">
            View and manage SSL certificates automatically acquired by Caddy.
          </p>
        </div>
      </div>

      <CertificateList />
    </div>
  )
}
