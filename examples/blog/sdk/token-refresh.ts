import { apiClient } from './api-client';
import { useAuthStore } from './auth-store';
const REFRESH_CHECK_INTERVAL_MS = 30_000;
const REFRESH_AHEAD_MS = 120_000;
const REFRESH_ENDPOINT = '/api/auth/refresh';

let refreshTimer: ReturnType<typeof setInterval> | null = null;
let refreshInFlight: Promise<void> | null = null;
let started = false;

const isBrowser = () => typeof window !== 'undefined' && typeof document !== 'undefined';

const decodeJwtPayload = (token: string | null | undefined): Record<string, unknown> | null => {
  const raw = typeof token === 'string' ? token.trim() : '';
  if (!raw) return null;
  const parts = raw.split('.');
  if (parts.length < 2) return null;
  try {
    const payload = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const normalized = payload + '='.repeat((4 - (payload.length % 4 || 4)) % 4);
    const decoded = typeof atob === 'function' ? atob(normalized) : '';
    if (!decoded) return null;
    return JSON.parse(decoded) as Record<string, unknown>;
  } catch {
    return null;
  }
};

const getTokenExpiryMs = (token: string | null | undefined): number | null => {
  const payload = decodeJwtPayload(token);
  const exp = payload?.exp;
  if (typeof exp !== 'number' || !Number.isFinite(exp) || exp <= 0) return null;
  return exp * 1000;
};

const shouldRefreshSoon = (token: string | null | undefined) => {
  const expiryMs = getTokenExpiryMs(token);
  if (!expiryMs) return false;
  return expiryMs - Date.now() <= REFRESH_AHEAD_MS;
};

const refreshAccessToken = async () => {
  const store = useAuthStore.getState();
  const currentRefresh = store.refreshToken;
  if (!currentRefresh) {
    return;
  }
  const response = await apiClient.post<{ accessToken: string; refreshToken: string }>(REFRESH_ENDPOINT, { refreshToken: currentRefresh });
  const data = response.data;
  store.setAuth(data.accessToken, store.user, data.refreshToken);
};

const tickRefresh = async () => {
  const store = useAuthStore.getState();
  if (!store.token || !store.refreshToken) {
    return;
  }
  if (!shouldRefreshSoon(store.token)) {
    return;
  }
  if (!refreshInFlight) {
    refreshInFlight = refreshAccessToken()
      .catch((error) => {
        if ((error as { status?: number } | null)?.status === 401 || (error as { status?: number } | null)?.status === 403) {
          useAuthStore.getState().clearAuth();
        }
      })
      .finally(() => {
        refreshInFlight = null;
      });
  }
  await refreshInFlight;
};

export const startTokenRefresh = () => {
  if (!isBrowser() || started) {
    return;
  }
  started = true;
  void tickRefresh();
  refreshTimer = setInterval(() => {
    void tickRefresh();
  }, REFRESH_CHECK_INTERVAL_MS);
};

export const stopTokenRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
  started = false;
};

startTokenRefresh();
