import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getSetupStatus, performSetup, SetupRequest } from '../api/setup';
import client from '../api/client';
import { useAuth } from '../hooks/useAuth';
import { Input } from '../components/ui/Input';
import { Button } from '../components/ui/Button';
import { PasswordStrengthMeter } from '../components/PasswordStrengthMeter';

const Setup: React.FC = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { login, isAuthenticated } = useAuth();
  const [formData, setFormData] = useState<SetupRequest>({
    name: '',
    email: '',
    password: '',
  });
  const [error, setError] = useState<string | null>(null);

  const { data: status, isLoading: statusLoading } = useQuery({
    queryKey: ['setupStatus'],
    queryFn: getSetupStatus,
    retry: false,
  });

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/');
      return;
    }
    if (status && !status.setupRequired) {
      navigate('/login');
    }
  }, [status, isAuthenticated, navigate]);

  const mutation = useMutation({
    mutationFn: async (data: SetupRequest) => {
      // 1. Perform Setup
      await performSetup(data);
      // 2. Auto Login
      await client.post('/auth/login', { email: data.email, password: data.password });
      // 3. Update Auth Context
      await login();
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['setupStatus'] });
      navigate('/');
    },
    onError: (err: any) => {
      setError(err.response?.data?.error || 'Setup failed');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    mutation.mutate(formData);
  };

  if (statusLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900">
        <div className="text-blue-500">Loading...</div>
      </div>
    );
  }

  if (status && !status.setupRequired) {
    return null; // Will redirect in useEffect
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8 bg-white dark:bg-gray-800 p-8 rounded-lg shadow-md">
        <div>
          <h2 className="mt-6 text-center text-3xl font-extrabold text-gray-900 dark:text-white">
            Welcome to CPM+
          </h2>
          <p className="mt-2 text-center text-sm text-gray-600 dark:text-gray-400">
            Create your administrator account to get started.
          </p>
        </div>
        <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
          <div className="space-y-4">
            <Input
              id="name"
              name="name"
              label="Name"
              type="text"
              required
              placeholder="Admin User"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            />
            <Input
              id="email"
              name="email"
              label="Email Address"
              type="email"
              required
              placeholder="admin@example.com"
              value={formData.email}
              onChange={(e) => setFormData({ ...formData, email: e.target.value })}
              helperText="This email will be used for Let's Encrypt certificate notifications and recovery."
            />
                        <div className="space-y-1">
              <Input
                id="password"
                name="password"
                label="Password"
                type="password"
                required
                placeholder="••••••••"
                value={formData.password}
                onChange={(e) => setFormData({ ...formData, password: e.target.value })}
              />
              <PasswordStrengthMeter password={formData.password} />
            </div>
          </div>

          {error && (
            <div className="text-red-500 text-sm text-center">
              {error}
            </div>
          )}

          <div>
            <Button
              type="submit"
              className="w-full"
              isLoading={mutation.isPending}
            >
              Create Admin Account
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default Setup;
