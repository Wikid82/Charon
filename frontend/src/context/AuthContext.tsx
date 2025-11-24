import React, { useState, useEffect } from 'react';
import client from '../api/client';
import { AuthContext, User } from './AuthContextValue';

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const checkAuth = async () => {
      try {
        const response = await client.get('/auth/me');
        setUser(response.data);
      } catch {
        setUser(null);
      } finally {
        setIsLoading(false);
      }
    };

    checkAuth();
  }, []);

  const login = async () => {
    // Token is stored in cookie by backend, but we might want to store it in memory or trigger a re-fetch
    // Actually, if backend sets cookie, we just need to fetch /auth/me
    try {
      const response = await client.get<User>('/auth/me');
      setUser(response.data);
    } catch (error) {
      setUser(null);
      throw error;
    }
  };

  const logout = async () => {
    try {
      await client.post('/auth/logout');
    } catch (error) {
      console.error("Logout failed", error);
    }
    setUser(null);
  };

  const changePassword = async (oldPassword: string, newPassword: string) => {
    await client.post('/auth/change-password', {
      old_password: oldPassword,
      new_password: newPassword,
    });
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
