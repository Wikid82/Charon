import { createContext } from 'react';

export interface User {
  user_id: number;
  role: 'admin' | 'user' | 'passthrough';
  name?: string;
  email?: string;
}

export interface AuthContextType {
  user: User | null;
  login: (token?: string) => Promise<void>;
  logout: () => void;
  changePassword: (oldPassword: string, newPassword: string) => Promise<void>;
  isAuthenticated: boolean;
  isLoading: boolean;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);
