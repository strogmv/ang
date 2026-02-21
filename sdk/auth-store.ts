import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type * as Types from './types';

interface AuthState {
  token: string | null;
  refreshToken: string | null;
  user: Types.User | null;
  setAuth: (token: string, user: Types.User, refreshToken?: string) => void;
  clearAuth: () => void;
  isAuthenticated: () => boolean;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      refreshToken: null,
      user: null,
      setAuth: (token, user, refreshToken = null) =>
        set({
          token,
          user,
          refreshToken,
        }),
      clearAuth: () =>
        set({
          token: null,
          refreshToken: null,
          user: null,
        }),
      isAuthenticated: () => !!get().token,
    }),
    {
      name: 'auth-storage',
    }
  )
);
