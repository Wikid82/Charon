import { useState } from 'react';
import { useAuthUsers } from '../../hooks/useSecurity';
import { Button } from '../../components/ui/Button';
import { Plus, Edit, Trash2, Shield, User } from 'lucide-react';
import toast from 'react-hot-toast';
import type { AuthUser, CreateAuthUserRequest, UpdateAuthUserRequest } from '../../api/security';

export default function Users() {
  const { users, createUser, updateUser, deleteUser, isLoading } = useAuthUsers();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<AuthUser | null>(null);
  const [formData, setFormData] = useState<CreateAuthUserRequest>({
    username: '',
    email: '',
    name: '',
    password: '',
    roles: '',
    mfa_enabled: false,
    additional_emails: '',
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editingUser) {
        const updateData: UpdateAuthUserRequest = {
          email: formData.email,
          name: formData.name,
          roles: formData.roles,
          mfa_enabled: formData.mfa_enabled,
          additional_emails: formData.additional_emails,
        };
        if (formData.password) {
          updateData.password = formData.password;
        }
        await updateUser({ uuid: editingUser.uuid, data: updateData });
        toast.success('User updated successfully');
      } else {
        await createUser(formData);
        toast.success('User created successfully');
      }
      setIsModalOpen(false);
      resetForm();
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(err.response?.data?.error || 'Failed to save user');
    }
  };

  const handleDelete = async (uuid: string) => {
    if (confirm('Are you sure you want to delete this user?')) {
      try {
        await deleteUser(uuid);
        toast.success('User deleted successfully');
      } catch (error: unknown) {
        const err = error as { response?: { data?: { error?: string } } };
        toast.error(err.response?.data?.error || 'Failed to delete user');
      }
    }
  };

  const resetForm = () => {
    setFormData({
      username: '',
      email: '',
      name: '',
      password: '',
      roles: '',
      mfa_enabled: false,
      additional_emails: '',
    });
    setEditingUser(null);
  };

  const openEditModal = (user: AuthUser) => {
    setEditingUser(user);
    setFormData({
      username: user.username,
      email: user.email,
      name: user.name,
      password: '', // Don't populate password
      roles: user.roles,
      mfa_enabled: user.mfa_enabled,
      additional_emails: user.additional_emails || '',
    });
    setIsModalOpen(true);
  };

  if (isLoading) return <div>Loading...</div>;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-xl font-semibold text-white">Local Users</h2>
        <Button onClick={() => { resetForm(); setIsModalOpen(true); }}>
          <Plus size={16} className="mr-2" />
          Add User
        </Button>
      </div>

      <div className="bg-dark-card rounded-lg border border-gray-800 overflow-hidden">
        <table className="w-full text-left text-sm text-gray-400">
          <thead className="bg-gray-900 text-gray-200 uppercase font-medium">
            <tr>
              <th className="px-6 py-3">User</th>
              <th className="px-6 py-3">Name</th>
              <th className="px-6 py-3">Roles</th>
              <th className="px-6 py-3">Created</th>
              <th className="px-6 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {users.map((user) => (
              <tr key={user.uuid} className="hover:bg-gray-800/50 transition-colors">
                <td className="px-6 py-4">
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-full bg-blue-900/30 flex items-center justify-center text-blue-400">
                      <User size={16} />
                    </div>
                    <div>
                      <div className="font-medium text-white">{user.username}</div>
                      <div className="text-xs text-gray-500">{user.email}</div>
                    </div>
                  </div>
                </td>
                <td className="px-6 py-4">
                  {user.name}
                </td>
                <td className="px-6 py-4">
                  {user.roles ? (
                    <span className="text-blue-400 flex items-center gap-1">
                      <Shield size={14} /> {user.roles}
                    </span>
                  ) : (
                    <span className="text-gray-600">User</span>
                  )}
                </td>
                <td className="px-6 py-4">
                  {new Date(user.created_at).toLocaleDateString()}
                </td>
                <td className="px-6 py-4 text-right">
                  <div className="flex justify-end gap-2">
                    <button
                      onClick={() => openEditModal(user)}
                      className="p-1.5 rounded-md hover:bg-gray-700 text-gray-400 hover:text-white transition-colors"
                    >
                      <Edit size={16} />
                    </button>
                    <button
                      onClick={() => handleDelete(user.uuid)}
                      className="p-1.5 rounded-md hover:bg-red-900/30 text-gray-400 hover:text-red-400 transition-colors"
                    >
                      <Trash2 size={16} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {users.length === 0 && (
              <tr>
                <td colSpan={5} className="px-6 py-8 text-center text-gray-500">
                  No users found. Create one to get started.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* User Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-dark-card rounded-lg border border-gray-800 max-w-md w-full p-6">
            <h3 className="text-lg font-bold text-white mb-4">
              {editingUser ? 'Edit User' : 'Add User'}
            </h3>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-400 mb-1">Username</label>
                <input
                  type="text"
                  required
                  value={formData.username}
                  onChange={e => setFormData({ ...formData, username: e.target.value })}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                  disabled={!!editingUser}
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-400 mb-1">Email</label>
                <input
                  type="email"
                  required
                  value={formData.email}
                  onChange={e => setFormData({ ...formData, email: e.target.value })}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-400 mb-1">Full Name</label>
                <input
                  type="text"
                  required
                  value={formData.name}
                  onChange={e => setFormData({ ...formData, name: e.target.value })}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-400 mb-1">
                  {editingUser ? 'New Password (leave blank to keep)' : 'Password'}
                </label>
                <input
                  type="password"
                  required={!editingUser}
                  value={formData.password}
                  onChange={e => setFormData({ ...formData, password: e.target.value })}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-400 mb-1">Roles (comma separated)</label>
                <input
                  type="text"
                  value={formData.roles}
                  onChange={e => setFormData({ ...formData, roles: e.target.value })}
                  placeholder="admin, editor"
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-400 mb-1">Additional Emails (comma separated)</label>
                <input
                  type="text"
                  value={formData.additional_emails || ''}
                  onChange={e => setFormData({ ...formData, additional_emails: e.target.value })}
                  placeholder="email2@example.com, email3@example.com"
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-500"
                />
                <p className="text-xs text-gray-500 mt-1">Used for linking multiple OAuth identities to this user.</p>
              </div>
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="mfa_enabled"
                  checked={formData.mfa_enabled}
                  onChange={e => setFormData({ ...formData, mfa_enabled: e.target.checked })}
                  className="w-4 h-4 text-blue-600 bg-gray-900 border-gray-700 rounded focus:ring-blue-500"
                />
                <label htmlFor="mfa_enabled" className="text-sm text-gray-400">MFA Enabled</label>
              </div>

              <div className="flex justify-end gap-3 mt-6">
                <Button variant="ghost" onClick={() => setIsModalOpen(false)}>Cancel</Button>
                <Button type="submit">{editingUser ? 'Save Changes' : 'Create User'}</Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
