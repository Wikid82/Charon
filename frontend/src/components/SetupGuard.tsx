import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getSetupStatus } from '../api/setup';

interface SetupGuardProps {
  children: React.ReactNode;
}

export const SetupGuard: React.FC<SetupGuardProps> = ({ children }) => {
  const navigate = useNavigate();

  const { data: status, isLoading } = useQuery({
    queryKey: ['setupStatus'],
    queryFn: getSetupStatus,
    retry: false,
  });

  useEffect(() => {
    if (status?.setupRequired) {
      navigate('/setup');
    }
  }, [status, navigate]);

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900">
        <div className="text-blue-500">Loading...</div>
      </div>
    );
  }

  if (status?.setupRequired) {
    return null; // Will redirect
  }

  return <>{children}</>;
};
