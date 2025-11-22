import { createContext } from 'react';

export interface User {
  user_id: number;
  role: string;
  name?: string;
  email?: string;
}

export interface AuthContextType {
  user: User | null;
  login: () => Promise<void>;
  logout: () => void;
  changePassword: (oldPassword: string, newPassword: string) => Promise<void>;
  isAuthenticated: boolean;
  isLoading: boolean;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);
