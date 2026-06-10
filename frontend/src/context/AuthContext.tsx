import { useState, useEffect, useCallback, useRef, type ReactNode, type FC } from 'react';

import { AuthContext, type User } from './AuthContextValue';
import client, { setAuthToken, setAuthErrorHandler } from '../api/client';

export const AuthProvider: FC<{ children: ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const authRequestVersionRef = useRef(0);

  const fetchSessionUser = useCallback(async (): Promise<User> => {
    const headers: Record<string, string> = { Accept: 'application/json' };
    const stored = localStorage.getItem('charon_auth_token');
    if (stored) {
      headers['Authorization'] = `Bearer ${stored}`;
    }

    const response = await fetch('/api/v1/auth/me', {
      method: 'GET',
      credentials: 'include',
      headers,
    });

    if (!response.ok) {
      throw new Error('Session validation failed');
    }

    return response.json() as Promise<User>;
  }, []);

  const invalidateAuthRequests = useCallback(() => {
    authRequestVersionRef.current += 1;
  }, []);

  // Handle session expiry by clearing auth state and redirecting to login
  const handleAuthError = useCallback(() => {
    console.warn('Session expired, clearing auth state');
    invalidateAuthRequests();
    localStorage.removeItem('charon_auth_token');
    setAuthToken(null);
    setUser(null);
    setIsLoading(false);
  }, [invalidateAuthRequests]);

  // Register auth error handler on mount; unregister on unmount so the axios
  // interceptor never calls a stale handler after the provider is gone
  useEffect(() => {
    setAuthErrorHandler(handleAuthError);
    return () => setAuthErrorHandler(null);
  }, [handleAuthError]);

  useEffect(() => {
    const checkAuth = async () => {
      const requestVersion = authRequestVersionRef.current + 1;
      authRequestVersionRef.current = requestVersion;

      try {
        const stored = localStorage.getItem('charon_auth_token');
        if (stored) {
          setAuthToken(stored);
        } else {
          // No token in localStorage - don't even try to authenticate
          // This prevents re-authentication via HttpOnly cookie after logout
          setAuthToken(null);
          if (authRequestVersionRef.current === requestVersion) {
            setUser(null);
            setIsLoading(false);
          }
          return;
        }
        const response = await fetchSessionUser();
        if (authRequestVersionRef.current === requestVersion) {
          setUser(response);
        }
      } catch {
        if (authRequestVersionRef.current === requestVersion) {
          setAuthToken(null);
          setUser(null);
        }
      } finally {
        if (authRequestVersionRef.current === requestVersion) {
          setIsLoading(false);
        }
      }
    };

    checkAuth();
  }, [fetchSessionUser]);

  const login = useCallback(async (token?: string) => {
    const requestVersion = authRequestVersionRef.current + 1;
    authRequestVersionRef.current = requestVersion;
    setIsLoading(true);

    if (token) {
      localStorage.setItem('charon_auth_token', token);
      setAuthToken(token);
    }

    try {
      const response = await fetchSessionUser();
      if (authRequestVersionRef.current === requestVersion) {
        setUser(response);
      }
    } catch (error) {
      if (authRequestVersionRef.current === requestVersion) {
        setUser(null);
        setAuthToken(null);
        localStorage.removeItem('charon_auth_token');
      }
      throw error;
    } finally {
      if (authRequestVersionRef.current === requestVersion) {
        setIsLoading(false);
      }
    }
  }, [fetchSessionUser]);

  const logout = useCallback(async () => {
    invalidateAuthRequests();
    localStorage.removeItem('charon_auth_token');
    setAuthToken(null);
    setUser(null);
    setIsLoading(false);

    try {
      await client.post('/auth/logout');
    } catch (error) {
      console.error("Logout failed", error);
    }
  }, [invalidateAuthRequests]);

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
      throw new Error(message, {
        cause: error,
      });
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

    for (const event of events) {
      window.addEventListener(event, handleActivity);
    }

    return () => {
      if (timeoutId) clearTimeout(timeoutId);
      for (const event of events) {
        window.removeEventListener(event, handleActivity);
      }
    };
  }, [user, logout]);

  return (
    <AuthContext.Provider value={{ user, login, logout, changePassword, isAuthenticated: !!user, isLoading }}>
      {children}
    </AuthContext.Provider>
  );
};
