import { useState, useEffect, useCallback, type ReactNode, type FC } from 'react';
import client, { setAuthToken, setAuthErrorHandler } from '../api/client';
import { AuthContext, User } from './AuthContextValue';

export const AuthProvider: FC<{ children: ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Handle session expiry by clearing auth state and redirecting to login
  const handleAuthError = useCallback(() => {
    console.log('Session expired, redirecting to login');
    localStorage.removeItem('charon_auth_token');
    setAuthToken(null);
    setUser(null);
    // Use window.location for full page redirect to clear any stale state
    if (window.location.pathname !== '/login') {
      window.location.href = '/login';
    }
  }, []);

  // Register auth error handler on mount
  useEffect(() => {
    setAuthErrorHandler(handleAuthError);
  }, [handleAuthError]);

  useEffect(() => {
    const checkAuth = async () => {
      try {
        const stored = localStorage.getItem('charon_auth_token');
        if (stored) {
          setAuthToken(stored);
        } else {
          // No token in localStorage - don't even try to authenticate
          // This prevents re-authentication via HttpOnly cookie after logout
          setAuthToken(null);
          setUser(null);
          setIsLoading(false);
          return;
        }
        const response = await client.get('/auth/me');
        setUser(response.data);
      } catch {
        setAuthToken(null);
        setUser(null);
      } finally {
        setIsLoading(false);
      }
    };

    checkAuth();
  }, []);

  const login = async (token?: string) => {
    if (token) {
      localStorage.setItem('charon_auth_token', token);
      setAuthToken(token);
    }
    try {
      const response = await client.get<User>('/auth/me');
      setUser(response.data);
    } catch (error) {
      setUser(null);
      setAuthToken(null);
      localStorage.removeItem('charon_auth_token');
      throw error;
    }
  };

  const logout = async () => {
    try {
      await client.post('/auth/logout');
    } catch (error) {
      console.error("Logout failed", error);
    }
    localStorage.removeItem('charon_auth_token');
    setAuthToken(null);
    setUser(null);

    // Force navigation to login with full page reload to clear any stale state
    // This ensures all React state and cookies are cleared
    if (window.location.pathname !== '/login') {
      window.location.href = '/login';
    }
  };

  const changePassword = async (oldPassword: string, newPassword: string) => {
    try {
      await client.post('/auth/change-password', {
        old_password: oldPassword,
        new_password: newPassword,
      });
    } catch (error: unknown) {
      // Extract error message from API response
      const message = error instanceof Error
        ? error.message
        : typeof error === 'object' && error !== null && 'response' in error
          ? (error as { response?: { data?: { error?: string } } }).response?.data?.error || 'Password change failed'
          : 'Password change failed';
      throw new Error(message);
    }
  };

  // Auto-logout logic
  useEffect(() => {
    if (!user) return;

    const TIMEOUT_MS = 15 * 60 * 1000; // 15 minutes
    let timeoutId: ReturnType<typeof setTimeout>;

    const resetTimer = () => {
      if (timeoutId) clearTimeout(timeoutId);
      timeoutId = setTimeout(() => {
        console.log('Auto-logging out due to inactivity');
        logout();
      }, TIMEOUT_MS);
    };

    // Initial timer start
    resetTimer();

    // Event listeners for activity
    const events = ['mousedown', 'keydown', 'scroll', 'touchstart'];
    const handleActivity = () => resetTimer();

    events.forEach(event => {
      window.addEventListener(event, handleActivity);
    });

    return () => {
      if (timeoutId) clearTimeout(timeoutId);
      events.forEach(event => {
        window.removeEventListener(event, handleActivity);
      });
    };
  }, [user]);

  return (
    <AuthContext.Provider value={{ user, login, logout, changePassword, isAuthenticated: !!user, isLoading }}>
      {children}
    </AuthContext.Provider>
  );
};
