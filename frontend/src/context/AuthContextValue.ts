import { createContext } from 'react';

export interface User {
  user_id: number;
  role: 'admin' | 'user' | 'passthrough';
  name?: string;
  email?: string;
  changelog_opt_out?: boolean;
}

export interface AuthContextType {
  user: User | null;
  login: (token?: string) => Promise<void>;
  logout: () => void;
  changePassword: (oldPassword: string, newPassword: string) => Promise<void>;
  /**
   * Re-fetches the current user from `/auth/me` and updates context state.
   * Used after mutations that change server-side user fields (e.g. the
   * "What's New" changelog opt-out toggle) so dependent UI reflects the
   * new value immediately, without a full page reload.
   */
  refetchUser: () => Promise<void>;
  isAuthenticated: boolean;
  isLoading: boolean;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);
