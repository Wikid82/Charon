import { useState } from 'react';
import { useAuthProviders } from '../../hooks/useSecurity';
import { Button } from '../../components/ui/Button';
import { Plus, Edit, Trash2, Globe } from 'lucide-react';
import toast from 'react-hot-toast';
import type { AuthProvider, CreateAuthProviderRequest, UpdateAuthProviderRequest } from '../../api/security';

interface ProviderFormData {
  name: string;
  type: 'google' | 'github' | 'oidc';
  client_id: string;
  client_secret: string;
  issuer_url: string;
  auth_url: string;
  token_url: string;
  user_info_url: string;
  scopes: string;
  display_name: string;
  enabled: boolean;
}

export default function Providers() {
  const { providers, createProvider, updateProvider, deleteProvider, isLoading } = useAuthProviders();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<AuthProvider | null>(null);
  const [formData, setFormData] = useState<ProviderFormData>({
    name: '',
    type: 'oidc',
    client_id: '',
    client_secret: '',
    issuer_url: '',
    auth_url: '',
    token_url: '',
    user_info_url: '',
    scopes: 'openid,profile,email',
    display_name: '',
    enabled: true,
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editingProvider) {
        const updateData: UpdateAuthProviderRequest = {
          name: formData.name,
          client_id: formData.client_id,
          issuer_url: formData.issuer_url,
          auth_url: formData.auth_url,
          token_url: formData.token_url,
          user_info_url: formData.user_info_url,
          scopes: formData.scopes,
          display_name: formData.display_name,
          enabled: formData.enabled,
        };
        if (formData.client_secret) {
          updateData.client_secret = formData.client_secret;
        }
        await updateProvider({ uuid: editingProvider.uuid, data: updateData });
        toast.success('Provider updated successfully');
      } else {
        const createData: CreateAuthProviderRequest = {
          name: formData.name,
          type: formData.type,
          client_id: formData.client_id,
          client_secret: formData.client_secret,
          issuer_url: formData.issuer_url,
          auth_url: formData.auth_url,
          token_url: formData.token_url,
          user_info_url: formData.user_info_url,
          scopes: formData.scopes,
          display_name: formData.display_name,
        };
        await createProvider(createData);
        toast.success('Provider created successfully');
      }
      setIsModalOpen(false);
      resetForm();
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to save provider');
    }
  };

  const handleDelete = async (uuid: string) => {
    if (confirm('Are you sure you want to delete this provider?')) {
      try {
        await deleteProvider(uuid);
        toast.success('Provider deleted successfully');
      } catch (error: any) {
        toast.error(error.response?.data?.error || 'Failed to delete provider');
      }
    }
  };

  const resetForm = () => {
    setFormData({
      name: '',
      type: 'oidc',
      client_id: '',
      client_secret: '',
      issuer_url: '',
      auth_url: '',
      token_url: '',
      user_info_url: '',
      scopes: 'openid,profile,email',
      display_name: '',
      enabled: true,
    });
    setEditingProvider(null);
  };

  const openEditModal = (provider: AuthProvider) => {
    setEditingProvider(provider);
    setFormData({
      name: provider.name,
      type: provider.type,
      client_id: provider.client_id,
      client_secret: '', // Don't populate secret
      issuer_url: provider.issuer_url || '',
      auth_url: provider.auth_url || '',
      token_url: provider.token_url || '',
      user_info_url: provider.user_info_url || '',
      scopes: provider.scopes || 'openid,profile,email',
      display_name: provider.display_name || '',
      enabled: provider.enabled,
    });
    setIsModalOpen(true);
  };

  if (isLoading) return <div>Loading...</div>;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-xl font-semibold text-white">Identity Providers</h2>
        <Button onClick={() => { resetForm(); setIsModalOpen(true); }}>
          <Plus size={16} className="mr-2" />
          Add Provider
        </Button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {providers.map((provider) => (
          <div key={provider.uuid} className="bg-dark-card rounded-lg border border-gray-800 p-6 space-y-4">
            <div className="flex justify-between items-start">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-lg bg-purple-900/30 flex items-center justify-center text-purple-400">
                  <Globe size={20} />
                </div>
                <div>
                  <h3 className="font-medium text-white">{provider.name}</h3>
                  <p className="text-xs text-gray-500 uppercase">{provider.type}</p>
                </div>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={() => openEditModal(provider)}
                  className="p-1.5 rounded-md hover:bg-gray-700 text-gray-400 hover:text-white transition-colors"
                >
                  <Edit size={16} />
                </button>
                <button
                  onClick={() => handleDelete(provider.uuid)}
                  className="p-1.5 rounded-md hover:bg-red-900/30 text-gray-400 hover:text-red-400 transition-colors"
                >
                  <Trash2 size={16} />
                </button>
              </div>
            </div>

            <div className="space-y-2 text-sm text-gray-400">
              <div className="flex justify-between">
                <span>Client ID:</span>
                <span className="font-mono text-gray-300">{provider.client_id}</span>
              </div>
              <div className="flex justify-between">
                <span>Status:</span>
                {provider.enabled ? (
                  <span className="text-green-400">Active</span>
                ) : (
                  <span className="text-red-400">Disabled</span>
                )}
              </div>
            </div>
          </div>
        ))}
        {providers.length === 0 && (
          <div className="col-span-full text-center py-12 text-gray-500 bg-dark-card rounded-lg border border-gray-800 border-dashed">
            No identity providers configured. Add one to enable external authentication.
          </div>
        )}
      </div>

      {/* Provider Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-dark-card rounded-lg border border-gray-800 max-w-2xl w-full p-6 max-h-[90vh] overflow-y-auto">
            <h3 className="text-lg font-bold text-white mb-4">
              {editingProvider ? 'Edit Provider' : 'Add Provider'}
            </h3>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-400 mb-1">Name</label>
                  <input
                    type="text"
                    required
                    value={formData.name}
                    onChange={e => setFormData({ ...formData, name: e.target.value })}
                    placeholder="Google"
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-400 mb-1">Type</label>
                  <select
                    value={formData.type}
                    onChange={e => setFormData({ ...formData, type: e.target.value as any })}
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                  >
                    <option value="oidc">Generic OIDC</option>
                    <option value="google">Google</option>
                    <option value="github">GitHub</option>
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <label className="block text-sm font-medium text-gray-400">Client ID</label>
                    <div className="group relative">
                      <button
                        type="button"
                        className="w-4 h-4 rounded-full bg-gray-700 text-gray-400 hover:bg-gray-600 flex items-center justify-center text-xs"
                        title="OAuth Client ID help"
                      >
                        ?
                      </button>
                      <div className="invisible group-hover:visible absolute bottom-6 left-1/2 transform -translate-x-1/2 bg-gray-800 text-white text-xs rounded-lg px-3 py-2 whitespace-nowrap z-10 shadow-lg">
                        The public identifier for your OAuth application.<br/>
                        Found in your provider's developer console (e.g., Google Cloud Console, GitHub Developer Settings).
                        <div className="absolute top-full left-1/2 transform -translate-x-1/2 border-4 border-transparent border-t-gray-800"></div>
                      </div>
                    </div>
                  </div>
                  <input
                    type="text"
                    required
                    value={formData.client_id}
                    onChange={e => setFormData({ ...formData, client_id: e.target.value })}
                    placeholder="e.g., 123456789.apps.googleusercontent.com"
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                  />
                </div>
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <label className="block text-sm font-medium text-gray-400">
                      {editingProvider ? 'Client Secret (leave blank to keep)' : 'Client Secret'}
                    </label>
                    <div className="group relative">
                      <button
                        type="button"
                        className="w-4 h-4 rounded-full bg-gray-700 text-gray-400 hover:bg-gray-600 flex items-center justify-center text-xs"
                        title="OAuth Client Secret help"
                      >
                        ?
                      </button>
                      <div className="invisible group-hover:visible absolute bottom-6 left-1/2 transform -translate-x-1/2 bg-gray-800 text-white text-xs rounded-lg px-3 py-2 whitespace-nowrap z-10 shadow-lg">
                        The private key for your OAuth application.<br/>
                        Keep this secret and secure! Generate/regenerate in your provider's developer console.
                        <div className="absolute top-full left-1/2 transform -translate-x-1/2 border-4 border-transparent border-t-gray-800"></div>
                      </div>
                    </div>
                  </div>
                  <input
                    type="password"
                    required={!editingProvider}
                    value={formData.client_secret}
                    onChange={e => setFormData({ ...formData, client_secret: e.target.value })}
                    placeholder="Enter your client secret"
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                  />
                </div>
              </div>

              {formData.type === 'oidc' && (
                <div>
                  <label className="block text-sm font-medium text-gray-400 mb-1">Issuer URL (Discovery)</label>
                  <input
                    type="url"
                    value={formData.issuer_url}
                    onChange={e => setFormData({ ...formData, issuer_url: e.target.value })}
                    placeholder="https://accounts.google.com"
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                  />
                </div>
              )}

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-400 mb-1">Scopes</label>
                  <input
                    type="text"
                    value={formData.scopes}
                    onChange={e => setFormData({ ...formData, scopes: e.target.value })}
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-400 mb-1">Display Name</label>
                  <input
                    type="text"
                    value={formData.display_name}
                    onChange={e => setFormData({ ...formData, display_name: e.target.value })}
                    placeholder="Sign in with Google"
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                  />
                </div>
              </div>

              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="enabled"
                  checked={formData.enabled}
                  onChange={e => setFormData({ ...formData, enabled: e.target.checked })}
                  className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
                />
                <label htmlFor="enabled" className="text-sm text-gray-400">Enabled</label>
              </div>

              <div className="flex justify-end gap-3 mt-6">
                <Button variant="ghost" onClick={() => setIsModalOpen(false)}>Cancel</Button>
                <Button type="submit">{editingProvider ? 'Save Changes' : 'Create Provider'}</Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
