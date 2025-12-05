import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Switch } from '../components/ui/Switch'
import { toast } from '../utils/toast'
import {
  listUsers,
  inviteUser,
  deleteUser,
  updateUser,
  updateUserPermissions,
} from '../api/users'
import type { User, InviteUserRequest, PermissionMode, UpdateUserPermissionsRequest } from '../api/users'
import { getProxyHosts } from '../api/proxyHosts'
import type { ProxyHost } from '../api/proxyHosts'
import {
  Users,
  UserPlus,
  Mail,
  Shield,
  Trash2,
  Settings,
  X,
  Check,
  AlertCircle,
  Clock,
  Copy,
  Loader2,
} from 'lucide-react'

interface InviteModalProps {
  isOpen: boolean
  onClose: () => void
  proxyHosts: ProxyHost[]
}

function InviteModal({ isOpen, onClose, proxyHosts }: InviteModalProps) {
  const queryClient = useQueryClient()
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<'user' | 'admin'>('user')
  const [permissionMode, setPermissionMode] = useState<PermissionMode>('allow_all')
  const [selectedHosts, setSelectedHosts] = useState<number[]>([])
  const [inviteResult, setInviteResult] = useState<{
    token: string
    emailSent: boolean
    expiresAt: string
  } | null>(null)

  const inviteMutation = useMutation({
    mutationFn: async () => {
      const request: InviteUserRequest = {
        email,
        role,
        permission_mode: permissionMode,
        permitted_hosts: selectedHosts,
      }
      return inviteUser(request)
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setInviteResult({
        token: data.invite_token,
        emailSent: data.email_sent,
        expiresAt: data.expires_at,
      })
      if (data.email_sent) {
        toast.success('Invitation email sent')
      } else {
        toast.success('User invited - copy the invite link below')
      }
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || 'Failed to invite user')
    },
  })

  const copyInviteLink = () => {
    if (inviteResult?.token) {
      const link = `${window.location.origin}/accept-invite?token=${inviteResult.token}`
      navigator.clipboard.writeText(link)
      toast.success('Invite link copied to clipboard')
    }
  }

  const handleClose = () => {
    setEmail('')
    setRole('user')
    setPermissionMode('allow_all')
    setSelectedHosts([])
    setInviteResult(null)
    onClose()
  }

  const toggleHost = (hostId: number) => {
    setSelectedHosts((prev) =>
      prev.includes(hostId) ? prev.filter((id) => id !== hostId) : [...prev, hostId]
    )
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-dark-card border border-gray-800 rounded-lg w-full max-w-lg max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-4 border-b border-gray-800">
          <h3 className="text-lg font-semibold text-white flex items-center gap-2">
            <UserPlus className="h-5 w-5" />
            Invite User
          </h3>
          <button onClick={handleClose} className="text-gray-400 hover:text-white">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="p-4 space-y-4">
          {inviteResult ? (
            <div className="space-y-4">
              <div className="bg-green-900/20 border border-green-800 rounded-lg p-4">
                <div className="flex items-center gap-2 text-green-400 mb-2">
                  <Check className="h-5 w-5" />
                  <span className="font-medium">User Invited Successfully</span>
                </div>
                {inviteResult.emailSent ? (
                  <p className="text-sm text-gray-300">
                    An invitation email has been sent to the user.
                  </p>
                ) : (
                  <p className="text-sm text-gray-300">
                    Email was not sent. Share the invite link manually.
                  </p>
                )}
              </div>

              {!inviteResult.emailSent && (
                <div className="space-y-2">
                  <label className="block text-sm font-medium text-gray-300">
                    Invite Link
                  </label>
                  <div className="flex gap-2">
                    <Input
                      type="text"
                      value={`${window.location.origin}/accept-invite?token=${inviteResult.token}`}
                      readOnly
                      className="flex-1 text-sm"
                    />
                    <Button onClick={copyInviteLink}>
                      <Copy className="h-4 w-4" />
                    </Button>
                  </div>
                  <p className="text-xs text-gray-500">
                    Expires: {new Date(inviteResult.expiresAt).toLocaleString()}
                  </p>
                </div>
              )}

              <Button onClick={handleClose} className="w-full">
                Done
              </Button>
            </div>
          ) : (
            <>
              <Input
                label="Email Address"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="user@example.com"
              />

              <div className="w-full">
                <label className="block text-sm font-medium text-gray-300 mb-1.5">
                  Role
                </label>
                <select
                  value={role}
                  onChange={(e) => setRole(e.target.value as 'user' | 'admin')}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="user">User</option>
                  <option value="admin">Admin</option>
                </select>
              </div>

              {role === 'user' && (
                <>
                  <div className="w-full">
                    <label className="block text-sm font-medium text-gray-300 mb-1.5">
                      Permission Mode
                    </label>
                    <select
                      value={permissionMode}
                      onChange={(e) => setPermissionMode(e.target.value as PermissionMode)}
                      className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                      <option value="allow_all">Allow All (Blacklist)</option>
                      <option value="deny_all">Deny All (Whitelist)</option>
                    </select>
                    <p className="mt-1 text-xs text-gray-500">
                      {permissionMode === 'allow_all'
                        ? 'User can access all hosts EXCEPT those selected below'
                        : 'User can ONLY access hosts selected below'}
                    </p>
                  </div>

                  <div className="w-full">
                    <label className="block text-sm font-medium text-gray-300 mb-1.5">
                      {permissionMode === 'allow_all' ? 'Blocked Hosts' : 'Allowed Hosts'}
                    </label>
                    <div className="max-h-48 overflow-y-auto border border-gray-700 rounded-lg">
                      {proxyHosts.length === 0 ? (
                        <p className="p-3 text-sm text-gray-500">No proxy hosts configured</p>
                      ) : (
                        proxyHosts.map((host) => (
                          <label
                            key={host.uuid}
                            className="flex items-center gap-3 p-3 hover:bg-gray-800/50 cursor-pointer border-b border-gray-800 last:border-0"
                          >
                            <input
                              type="checkbox"
                              checked={selectedHosts.includes(
                                parseInt(host.uuid.split('-')[0], 16) || 0
                              )}
                              onChange={() =>
                                toggleHost(parseInt(host.uuid.split('-')[0], 16) || 0)
                              }
                              className="rounded bg-gray-900 border-gray-700 text-blue-500 focus:ring-blue-500"
                            />
                            <div>
                              <p className="text-sm text-white">{host.name || host.domain_names}</p>
                              <p className="text-xs text-gray-500">{host.domain_names}</p>
                            </div>
                          </label>
                        ))
                      )}
                    </div>
                  </div>
                </>
              )}

              <div className="flex gap-3 pt-4 border-t border-gray-700">
                <Button variant="secondary" onClick={handleClose} className="flex-1">
                  Cancel
                </Button>
                <Button
                  onClick={() => inviteMutation.mutate()}
                  isLoading={inviteMutation.isPending}
                  disabled={!email}
                  className="flex-1"
                >
                  <Mail className="h-4 w-4 mr-2" />
                  Send Invite
                </Button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}

interface PermissionsModalProps {
  isOpen: boolean
  onClose: () => void
  user: User | null
  proxyHosts: ProxyHost[]
}

function PermissionsModal({ isOpen, onClose, user, proxyHosts }: PermissionsModalProps) {
  const queryClient = useQueryClient()
  const [permissionMode, setPermissionMode] = useState<PermissionMode>('allow_all')
  const [selectedHosts, setSelectedHosts] = useState<number[]>([])

  // Update state when user changes
  useState(() => {
    if (user) {
      setPermissionMode(user.permission_mode || 'allow_all')
      setSelectedHosts(user.permitted_hosts || [])
    }
  })

  const updatePermissionsMutation = useMutation({
    mutationFn: async () => {
      if (!user) return
      const request: UpdateUserPermissionsRequest = {
        permission_mode: permissionMode,
        permitted_hosts: selectedHosts,
      }
      return updateUserPermissions(user.id, request)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('Permissions updated')
      onClose()
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || 'Failed to update permissions')
    },
  })

  const toggleHost = (hostId: number) => {
    setSelectedHosts((prev) =>
      prev.includes(hostId) ? prev.filter((id) => id !== hostId) : [...prev, hostId]
    )
  }

  if (!isOpen || !user) return null

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-dark-card border border-gray-800 rounded-lg w-full max-w-lg max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-4 border-b border-gray-800">
          <h3 className="text-lg font-semibold text-white flex items-center gap-2">
            <Shield className="h-5 w-5" />
            Edit Permissions - {user.name || user.email}
          </h3>
          <button onClick={onClose} className="text-gray-400 hover:text-white">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="p-4 space-y-4">
          <div className="w-full">
            <label className="block text-sm font-medium text-gray-300 mb-1.5">
              Permission Mode
            </label>
            <select
              value={permissionMode}
              onChange={(e) => setPermissionMode(e.target.value as PermissionMode)}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="allow_all">Allow All (Blacklist)</option>
              <option value="deny_all">Deny All (Whitelist)</option>
            </select>
            <p className="mt-1 text-xs text-gray-500">
              {permissionMode === 'allow_all'
                ? 'User can access all hosts EXCEPT those selected below'
                : 'User can ONLY access hosts selected below'}
            </p>
          </div>

          <div className="w-full">
            <label className="block text-sm font-medium text-gray-300 mb-1.5">
              {permissionMode === 'allow_all' ? 'Blocked Hosts' : 'Allowed Hosts'}
            </label>
            <div className="max-h-64 overflow-y-auto border border-gray-700 rounded-lg">
              {proxyHosts.length === 0 ? (
                <p className="p-3 text-sm text-gray-500">No proxy hosts configured</p>
              ) : (
                proxyHosts.map((host) => (
                  <label
                    key={host.uuid}
                    className="flex items-center gap-3 p-3 hover:bg-gray-800/50 cursor-pointer border-b border-gray-800 last:border-0"
                  >
                    <input
                      type="checkbox"
                      checked={selectedHosts.includes(
                        parseInt(host.uuid.split('-')[0], 16) || 0
                      )}
                      onChange={() =>
                        toggleHost(parseInt(host.uuid.split('-')[0], 16) || 0)
                      }
                      className="rounded bg-gray-900 border-gray-700 text-blue-500 focus:ring-blue-500"
                    />
                    <div>
                      <p className="text-sm text-white">{host.name || host.domain_names}</p>
                      <p className="text-xs text-gray-500">{host.domain_names}</p>
                    </div>
                  </label>
                ))
              )}
            </div>
          </div>

          <div className="flex gap-3 pt-4 border-t border-gray-700">
            <Button variant="secondary" onClick={onClose} className="flex-1">
              Cancel
            </Button>
            <Button
              onClick={() => updatePermissionsMutation.mutate()}
              isLoading={updatePermissionsMutation.isPending}
              className="flex-1"
            >
              Save Permissions
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default function UsersPage() {
  const queryClient = useQueryClient()
  const [inviteModalOpen, setInviteModalOpen] = useState(false)
  const [permissionsModalOpen, setPermissionsModalOpen] = useState(false)
  const [selectedUser, setSelectedUser] = useState<User | null>(null)

  const { data: users, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: listUsers,
  })

  const { data: proxyHosts = [] } = useQuery({
    queryKey: ['proxyHosts'],
    queryFn: getProxyHosts,
  })

  const toggleEnabledMutation = useMutation({
    mutationFn: async ({ id, enabled }: { id: number; enabled: boolean }) => {
      return updateUser(id, { enabled })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('User updated')
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || 'Failed to update user')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('User deleted')
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      toast.error(err.response?.data?.error || 'Failed to delete user')
    },
  })

  const openPermissions = (user: User) => {
    setSelectedUser(user)
    setPermissionsModalOpen(true)
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-8 w-8 animate-spin text-blue-500" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Users className="h-6 w-6 text-blue-500" />
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">User Management</h1>
        </div>
        <Button onClick={() => setInviteModalOpen(true)}>
          <UserPlus className="h-4 w-4 mr-2" />
          Invite User
        </Button>
      </div>

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-800">
                <th className="text-left py-3 px-4 text-sm font-medium text-gray-400">User</th>
                <th className="text-left py-3 px-4 text-sm font-medium text-gray-400">Role</th>
                <th className="text-left py-3 px-4 text-sm font-medium text-gray-400">Status</th>
                <th className="text-left py-3 px-4 text-sm font-medium text-gray-400">Permissions</th>
                <th className="text-left py-3 px-4 text-sm font-medium text-gray-400">Enabled</th>
                <th className="text-right py-3 px-4 text-sm font-medium text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody>
              {users?.map((user) => (
                <tr key={user.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="py-3 px-4">
                    <div>
                      <p className="text-sm font-medium text-white">{user.name || '(No name)'}</p>
                      <p className="text-xs text-gray-500">{user.email}</p>
                    </div>
                  </td>
                  <td className="py-3 px-4">
                    <span
                      className={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${
                        user.role === 'admin'
                          ? 'bg-purple-900/30 text-purple-400'
                          : 'bg-blue-900/30 text-blue-400'
                      }`}
                    >
                      {user.role}
                    </span>
                  </td>
                  <td className="py-3 px-4">
                    {user.invite_status === 'pending' ? (
                      <span className="inline-flex items-center gap-1 text-yellow-400 text-xs">
                        <Clock className="h-3 w-3" />
                        Pending Invite
                      </span>
                    ) : user.invite_status === 'expired' ? (
                      <span className="inline-flex items-center gap-1 text-red-400 text-xs">
                        <AlertCircle className="h-3 w-3" />
                        Invite Expired
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-green-400 text-xs">
                        <Check className="h-3 w-3" />
                        Active
                      </span>
                    )}
                  </td>
                  <td className="py-3 px-4">
                    <span className="text-xs text-gray-400">
                      {user.permission_mode === 'deny_all' ? 'Whitelist' : 'Blacklist'}
                    </span>
                  </td>
                  <td className="py-3 px-4">
                    <Switch
                      checked={user.enabled}
                      onChange={() =>
                        toggleEnabledMutation.mutate({
                          id: user.id,
                          enabled: !user.enabled,
                        })
                      }
                      disabled={user.role === 'admin'}
                    />
                  </td>
                  <td className="py-3 px-4">
                    <div className="flex items-center justify-end gap-2">
                      {user.role !== 'admin' && (
                        <button
                          onClick={() => openPermissions(user)}
                          className="p-1.5 text-gray-400 hover:text-white hover:bg-gray-800 rounded"
                          title="Edit Permissions"
                        >
                          <Settings className="h-4 w-4" />
                        </button>
                      )}
                      <button
                        onClick={() => {
                          if (confirm('Are you sure you want to delete this user?')) {
                            deleteMutation.mutate(user.id)
                          }
                        }}
                        className="p-1.5 text-gray-400 hover:text-red-400 hover:bg-gray-800 rounded"
                        title="Delete User"
                        disabled={user.role === 'admin'}
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

      <InviteModal
        isOpen={inviteModalOpen}
        onClose={() => setInviteModalOpen(false)}
        proxyHosts={proxyHosts}
      />

      <PermissionsModal
        isOpen={permissionsModalOpen}
        onClose={() => {
          setPermissionsModalOpen(false)
          setSelectedUser(null)
        }}
        user={selectedUser}
        proxyHosts={proxyHosts}
      />
    </div>
  )
}
