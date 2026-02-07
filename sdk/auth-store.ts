import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type * as Types from './types';

interface AuthState {
  token: string | null;
  user: Types.User | null;
  setAuth: (token: string, user: Types.User) => void;
  clearAuth: () => void;
  isAuthenticated: () => boolean;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      user: null,
      setAuth: (token, user) => set({ token, user }),
      clearAuth: () => set({ token: null, user: null }),
      isAuthenticated: () => !!get().token,
    }),
    {
      name: 'auth-storage',
    }
  )
);
