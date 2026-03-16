import React from 'react';
import { Navigate, useLocation } from 'react-router-dom';

import { LoadingOverlay } from './LoadingStates';
import { useAuth } from '../hooks/useAuth';

const RequireAuth: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { isAuthenticated, isLoading, user } = useAuth();
  const location = useLocation();

  if (isLoading) {
    return <LoadingOverlay message="Authenticating..." />;  // Consistent loading UX
  }

  // Check both context state AND localStorage for token
  // This prevents access if either check fails (defense in depth)
  const hasToken = localStorage.getItem('charon_auth_token');
  const hasUser = user !== null;

  if (!isAuthenticated || !hasToken || !hasUser) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return children;
};

export default RequireAuth;
