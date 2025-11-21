import { createContext } from 'react';

export interface User {
  user_id: number;
  role: string;
}

export interface AuthContextType {
  user: User | null;
  login: () => Promise<void>;
  logout: () => void;
  isAuthenticated: boolean;
  isLoading: boolean;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);
