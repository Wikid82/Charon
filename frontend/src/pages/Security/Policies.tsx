import { useState } from 'react';
import { useAuthPolicies } from '../../hooks/useSecurity';
import { Button } from '../../components/ui/Button';
import { Plus, Edit, Trash2, ShieldCheck, Users, Globe } from 'lucide-react';
import toast from 'react-hot-toast';
import type { AuthPolicy, CreateAuthPolicyRequest, UpdateAuthPolicyRequest } from '../../api/security';

interface PolicyFormData {
  name: string;
  description: string;
  allowed_roles: string;
  allowed_users: string;
  allowed_domains: string;
  require_mfa: boolean;
  session_timeout: number;
}

export default function Policies() {
  const { policies, createPolicy, updatePolicy, deletePolicy, isLoading } = useAuthPolicies();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingPolicy, setEditingPolicy] = useState<AuthPolicy | null>(null);
  const [formData, setFormData] = useState<PolicyFormData>({
    name: '',
    description: '',
    allowed_roles: '',
    allowed_users: '',
    allowed_domains: '',
    require_mfa: false,
    session_timeout: 0,
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editingPolicy) {
        const updateData: UpdateAuthPolicyRequest = {
          name: formData.name,
          description: formData.description,
          allowed_roles: formData.allowed_roles,
          allowed_users: formData.allowed_users,
          allowed_domains: formData.allowed_domains,
          require_mfa: formData.require_mfa,
          session_timeout: formData.session_timeout,
        };
        await updatePolicy({ uuid: editingPolicy.uuid, data: updateData });
        toast.success('Policy updated successfully');
      } else {
        const createData: CreateAuthPolicyRequest = {
          name: formData.name,
          description: formData.description,
          allowed_roles: formData.allowed_roles,
          allowed_users: formData.allowed_users,
          allowed_domains: formData.allowed_domains,
          require_mfa: formData.require_mfa,
          session_timeout: formData.session_timeout,
        };
        await createPolicy(createData);
        toast.success('Policy created successfully');
      }
      setIsModalOpen(false);
      resetForm();
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || 'Failed to save policy');
    }
  };

  const handleDelete = async (uuid: string) => {
    if (confirm('Are you sure you want to delete this policy?')) {
      try {
        await deletePolicy(uuid);
        toast.success('Policy deleted successfully');
      } catch (error: unknown) {
        const err = error as { response?: { data?: { error?: string } } };
        toast.error(err.response?.data?.error || 'Failed to delete policy');
      }
    }
  };

  const resetForm = () => {
    setFormData({
      name: '',
      description: '',
      allowed_roles: '',
      allowed_users: '',
      allowed_domains: '',
      require_mfa: false,
      session_timeout: 0,
    });
    setEditingPolicy(null);
  };

  const openEditModal = (policy: AuthPolicy) => {
    setEditingPolicy(policy);
    setFormData({
      name: policy.name,
      description: policy.description || '',
      allowed_roles: policy.allowed_roles || '',
      allowed_users: policy.allowed_users || '',
      allowed_domains: policy.allowed_domains || '',
      require_mfa: policy.require_mfa || false,
      session_timeout: policy.session_timeout || 0,
    });
    setIsModalOpen(true);
  };

  if (isLoading) return <div>Loading...</div>;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-xl font-semibold text-white">Access Policies</h2>
        <Button onClick={() => { resetForm(); setIsModalOpen(true); }}>
          <Plus size={16} className="mr-2" />
          Add Policy
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-4">
        {policies.map((policy) => {
          return (
            <div key={policy.uuid} className="bg-dark-card rounded-lg border border-gray-800 p-6 flex flex-col md:flex-row justify-between gap-6">
              <div className="space-y-2 flex-1">
                <div className="flex items-center gap-3">
                  <h3 className="font-medium text-white text-lg">{policy.name}</h3>
                  {policy.require_mfa && (
                    <span className="px-2 py-0.5 rounded text-xs bg-blue-900/30 text-blue-400 border border-blue-900/50 flex items-center gap-1">
                      <ShieldCheck size={12} /> MFA Required
                    </span>
                  )}
                </div>
                <p className="text-sm text-gray-400">{policy.description || 'No description provided.'}</p>

                <div className="flex flex-wrap gap-4 mt-4">
                  {policy.allowed_roles && (
                    <div className="flex items-center gap-2 text-sm text-gray-400">
                      <ShieldCheck size={16} className="text-gray-500" />
                      <span>Roles: <span className="text-gray-300">{policy.allowed_roles}</span></span>
                    </div>
                  )}
                  {policy.allowed_users && (
                    <div className="flex items-center gap-2 text-sm text-gray-400">
                      <Users size={16} className="text-gray-500" />
                      <span>Users: <span className="text-gray-300">{policy.allowed_users}</span></span>
                    </div>
                  )}
                  {policy.allowed_domains && (
                    <div className="flex items-center gap-2 text-sm text-gray-400">
                      <Globe size={16} className="text-gray-500" />
                      <span>Domains: <span className="text-gray-300">{policy.allowed_domains}</span></span>
                    </div>
                  )}
                  {!policy.allowed_roles && !policy.allowed_users && !policy.allowed_domains && (
                    <span className="text-sm text-yellow-500">Public Access (No restrictions)</span>
                  )}
                </div>
              </div>

              <div className="flex items-start gap-2">
                <button
                  onClick={() => openEditModal(policy)}
                  className="p-2 rounded-md hover:bg-gray-700 text-gray-400 hover:text-white transition-colors"
                >
                  <Edit size={18} />
                </button>
                <button
                  onClick={() => handleDelete(policy.uuid)}
                  className="p-2 rounded-md hover:bg-red-900/30 text-gray-400 hover:text-red-400 transition-colors"
                >
                  <Trash2 size={18} />
                </button>
              </div>
            </div>
          );
        })}
        {policies.length === 0 && (
          <div className="text-center py-12 text-gray-500 bg-dark-card rounded-lg border border-gray-800 border-dashed">
            No access policies defined. Create one to protect your services.
          </div>
        )}
      </div>

      {/* Policy Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-dark-card rounded-lg border border-gray-800 max-w-2xl w-full p-6 max-h-[90vh] overflow-y-auto">
            <h3 className="text-lg font-bold text-white mb-4">
              {editingPolicy ? 'Edit Policy' : 'Add Policy'}
            </h3>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-400 mb-1">Policy Name</label>
                <input
                  type="text"
                  required
                  value={formData.name}
                  onChange={e => setFormData({ ...formData, name: e.target.value })}
                  placeholder="Admins Only"
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-400 mb-1">Description</label>
                <textarea
                  value={formData.description}
                  onChange={e => setFormData({ ...formData, description: e.target.value })}
                  rows={2}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-400 mb-1">Allowed Roles</label>
                  <input
                    type="text"
                    value={formData.allowed_roles}
                    onChange={e => setFormData({ ...formData, allowed_roles: e.target.value })}
                    placeholder="admin, editor"
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                  />
                  <p className="text-xs text-gray-500 mt-1">Comma-separated list of roles</p>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-400 mb-1">Allowed Users</label>
                  <input
                    type="text"
                    value={formData.allowed_users}
                    onChange={e => setFormData({ ...formData, allowed_users: e.target.value })}
                    placeholder="john, jane@example.com"
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                  />
                  <p className="text-xs text-gray-500 mt-1">Comma-separated usernames/emails</p>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-400 mb-1">Allowed Domains</label>
                <input
                  type="text"
                  value={formData.allowed_domains}
                  onChange={e => setFormData({ ...formData, allowed_domains: e.target.value })}
                  placeholder="example.com, corp.net"
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                />
                <p className="text-xs text-gray-500 mt-1">Restrict access to users with these email domains</p>
              </div>

              <div className="flex items-center gap-4 pt-2">
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="require_mfa"
                    checked={formData.require_mfa}
                    onChange={e => setFormData({ ...formData, require_mfa: e.target.checked })}
                    className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
                  />
                  <label htmlFor="require_mfa" className="text-sm text-gray-400">Require MFA</label>
                </div>
              </div>

              <div className="flex justify-end gap-3 mt-6">
                <Button variant="ghost" onClick={() => setIsModalOpen(false)}>Cancel</Button>
                <Button type="submit">{editingPolicy ? 'Save Changes' : 'Create Policy'}</Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
