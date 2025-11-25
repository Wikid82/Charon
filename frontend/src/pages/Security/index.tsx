import { useState } from 'react';
import { Users, Globe, Lock } from 'lucide-react';
import UsersPage from './Users';
import ProvidersPage from './Providers';
import PoliciesPage from './Policies';

export default function Security() {
  const [activeTab, setActiveTab] = useState<'users' | 'providers' | 'policies'>('users');

  return (
    <div className="space-y-6">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-white">Security & Access Control</h1>
          <p className="text-gray-400">Manage users, identity providers, and access policies for your services.</p>
        </div>
      </div>

      <div className="flex border-b border-gray-800">
        <button
          onClick={() => setActiveTab('users')}
          className={`px-6 py-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
            activeTab === 'users'
              ? 'border-blue-500 text-blue-500'
              : 'border-transparent text-gray-400 hover:text-gray-300 hover:border-gray-700'
          }`}
        >
          <Users size={16} />
          Users
        </button>
        <button
          onClick={() => setActiveTab('providers')}
          className={`px-6 py-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
            activeTab === 'providers'
              ? 'border-blue-500 text-blue-500'
              : 'border-transparent text-gray-400 hover:text-gray-300 hover:border-gray-700'
          }`}
        >
          <Globe size={16} />
          Identity Providers
        </button>
        <button
          onClick={() => setActiveTab('policies')}
          className={`px-6 py-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
            activeTab === 'policies'
              ? 'border-blue-500 text-blue-500'
              : 'border-transparent text-gray-400 hover:text-gray-300 hover:border-gray-700'
          }`}
        >
          <Lock size={16} />
          Access Policies
        </button>
      </div>

      <div className="pt-4">
        {activeTab === 'users' && <UsersPage />}
        {activeTab === 'providers' && <ProvidersPage />}
        {activeTab === 'policies' && <PoliciesPage />}
      </div>
    </div>
  );
}
