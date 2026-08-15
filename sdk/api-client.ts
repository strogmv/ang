import { useAuthStore } from './auth-store';
import { endpointMeta } from './endpoints/meta';
import { ErrorCode, ProblemDetail } from './types';
import * as Types from './types';
import { isProblemDetailLike, normalizeApiError } from './error-normalizer';
import * as Schemas from './schemas';

type ViteEnv = {
  VITE_API_URL?: string;
  DEV?: boolean;
};

const getViteEnv = (): ViteEnv | undefined => {
  try {
    return typeof import.meta !== 'undefined' ? (import.meta.env as ViteEnv | undefined) : undefined;
  } catch {
    return undefined;
  }
};

const getBaseUrl = () => {
  const viteEnv = getViteEnv();
  if (viteEnv?.VITE_API_URL) return viteEnv.VITE_API_URL;
  if (typeof process !== 'undefined' && process.env?.VITE_API_URL) return process.env.VITE_API_URL;
  return 'http://localhost:8080';
};

const isDevEnv = () => {
  const viteEnv = getViteEnv();
  if (viteEnv?.DEV !== undefined) {
    return Boolean(viteEnv.DEV);
  }
  if (typeof process !== 'undefined' && process.env?.NODE_ENV) {
    return process.env.NODE_ENV !== 'production';
  }
  return true;
};

export type ApiRequestOptions = {
  signal?: AbortSignal;
};

export interface ApiClientLogger {
  warn?: (message: string, meta?: unknown) => void;
  error?: (message: string, meta?: unknown) => void;
}

export type ResponseValidationReporter = (issue: {
  schemaName: string;
  context: string;
  issues: unknown;
  data: unknown;
}) => void;

const noopLogger: Required<ApiClientLogger> = {
  warn: () => undefined,
  error: () => undefined,
};

let apiLogger: Required<ApiClientLogger> = noopLogger;
let responseValidationReporter: ResponseValidationReporter | null = null;

export const setApiClientLogger = (logger: ApiClientLogger) => {
  apiLogger = {
    warn: logger.warn ?? noopLogger.warn,
    error: logger.error ?? noopLogger.error,
  };
};

export const setResponseValidationReporter = (reporter: ResponseValidationReporter | null) => {
  responseValidationReporter = reporter;
};

/**
 * Interface for Client-Side Encryption.
 * If provided, SDK will use it to handle fields marked as Encrypted<T>.
 */
export interface CryptoProvider {
  encrypt(data: unknown): Promise<string>;
  decrypt(encrypted: string): Promise<unknown>;
}

let cryptoProvider: CryptoProvider | null = null;

export const setCryptoProvider = (provider: CryptoProvider) => {
  cryptoProvider = provider;
};

// Helper: Generate hex string
const hex = (len: number) => {
  const cryptoObject = typeof globalThis !== 'undefined' ? globalThis.crypto : undefined;
  if (!cryptoObject?.getRandomValues) {
    throw new Error('Secure random generation is unavailable in this environment');
  }
  const arr = new Uint8Array(len / 2);
  cryptoObject.getRandomValues(arr);
  return Array.from(arr, (byte) => byte.toString(16).padStart(2, '0')).join('');
};

const findEndpointMeta = (url: string | undefined) => {
  if (!url) return undefined;
  const pathOnly = url.split('?')[0] || url;
  for (const meta of Object.values(endpointMeta)) {
    const pattern = meta.path.replace(/\{[^}]+\}/g, '[^/]+');
    if (new RegExp(`^${pattern}$`).test(pathOnly)) {
      return meta;
    }
  }
  return undefined;
};

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

type AuthStoreUserLike = {
  locale?: string | null;
};

// ── fetch-based transport ───────────────────────────────────────────────────
// Axios was dropped as a dependency; this reimplements the same request/retry/
// auth-refresh behaviour (previously axios interceptors) directly over fetch,
// while keeping the exact axios-shaped call surface (`.get/.post/.../.request`
// returning `{ data, status }`) that every generated endpoint file — and the
// base-api-client.ts compatibility facade — already calls.

export type RequestMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

export type ApiRequestConfig = {
  url: string;
  method: RequestMethod;
  data?: unknown;
  params?: Record<string, unknown>;
  headers?: Record<string, string>;
  signal?: AbortSignal;
  /** internal: set once a 401 retry has been attempted for this request */
  _retry?: boolean;
  /** internal: retry-strategy attempt counter, distinct from the 401 retry above */
  _retryAttempt?: number;
};

export type ApiResponse<T> = {
  data: T;
  status: number;
  statusText: string;
  headers: Headers;
};

const defaultHeaders: Record<string, string> = {
  'Content-Type': 'application/json',
};

export const apiClient = {
  defaults: {
    // Send/receive the session cookie (opaque_session_cookie auth) across origins.
    // The API must allow credentials and echo the exact Origin (not "*").
    baseURL: getBaseUrl(),
    headers: defaultHeaders,
  },
  get: <T = unknown>(url: string, config: { params?: Record<string, unknown>; headers?: Record<string, string>; signal?: AbortSignal } = {}) =>
    request<T>({ url, method: 'GET', params: config.params, headers: config.headers, signal: config.signal }),
  post: <T = unknown>(url: string, data?: unknown, config: { headers?: Record<string, string>; signal?: AbortSignal } = {}) =>
    request<T>({ url, method: 'POST', data, headers: config.headers, signal: config.signal }),
  put: <T = unknown>(url: string, data?: unknown, config: { headers?: Record<string, string>; signal?: AbortSignal } = {}) =>
    request<T>({ url, method: 'PUT', data, headers: config.headers, signal: config.signal }),
  patch: <T = unknown>(url: string, data?: unknown, config: { headers?: Record<string, string>; signal?: AbortSignal } = {}) =>
    request<T>({ url, method: 'PATCH', data, headers: config.headers, signal: config.signal }),
  delete: <T = unknown>(url: string, config: { data?: unknown; headers?: Record<string, string>; signal?: AbortSignal } = {}) =>
    request<T>({ url, method: 'DELETE', data: config.data, headers: config.headers, signal: config.signal }),
  request: <T = unknown>(config: ApiRequestConfig) => request<T>(config),
};

const buildQueryString = (params: Record<string, unknown> | undefined): string => {
  if (!params) return '';
  const usp = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null) continue;
    if (Array.isArray(value)) {
      for (const item of value) {
        if (item === undefined || item === null) continue;
        usp.append(key, typeof item === 'object' ? JSON.stringify(item) : String(item));
      }
      continue;
    }
    usp.append(key, typeof value === 'object' ? JSON.stringify(value) : String(value));
  }
  const qs = usp.toString();
  return qs ? `?${qs}` : '';
};

const buildUrl = (config: ApiRequestConfig): string => {
  const base = String(apiClient.defaults.baseURL || '').replace(/\/+$/, '');
  const path = config.url.startsWith('/') ? config.url : `/${config.url}`;
  return `${base}${path}${buildQueryString(config.params)}`;
};

const REFRESH_ENDPOINT = '/api/auth/refresh';
const isRefreshRequest = (config: ApiRequestConfig) => {
  const url = config.url;
  return url === REFRESH_ENDPOINT || url.startsWith(`${REFRESH_ENDPOINT}?`) || url.endsWith(REFRESH_ENDPOINT);
};

type RefreshWaiter = {
  resolve: (token: string) => void;
  reject: (error: unknown) => void;
};

let refreshPromise: Promise<string> | null = null;
let refreshWaiters: RefreshWaiter[] = [];

const waitForRefreshToken = () =>
  new Promise<string>((resolve, reject) => {
    refreshWaiters.push({ resolve, reject });
  });

const flushRefreshWaiters = (token?: string, error?: unknown) => {
  const pending = [...refreshWaiters];
  refreshWaiters = [];
  for (const waiter of pending) {
    if (token !== undefined) {
      waiter.resolve(token);
      continue;
    }
    waiter.reject(error ?? new Error('Token refresh failed'));
  }
};

const shouldClearAuthOnRefreshFailure = (error: unknown) => {
  if (!isProblemDetailLike(error)) return false;
  return error.status === 401 || error.status === 403;
};

const refreshAuthToken = async (): Promise<string> => {
  const store = useAuthStore.getState();
  const storedRefresh = store.refreshToken;
  if (!storedRefresh) {
    store.clearAuth();
    throw new Error('Missing refresh token');
  }

  if (!refreshPromise) {
    refreshPromise = apiClient
      .post<Types.RefreshTokenResponse>(REFRESH_ENDPOINT, { refreshToken: storedRefresh })
      .then((response) => {
        const data = response.data;
        if (import.meta.env.DEV) {
          validateResponse('RefreshTokenResponseSchema', data, 'RefreshToken');
        }
        store.setAuth(data.accessToken, store.user, data.refreshToken);
        flushRefreshWaiters(data.accessToken);
        return data.accessToken;
      })
      .catch((err) => {
        if (shouldClearAuthOnRefreshFailure(err)) {
          store.clearAuth();
        }
        flushRefreshWaiters(undefined, err);
        throw err;
      })
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
};

const shouldRetry = (config: ApiRequestConfig, isNetworkError: boolean, status?: number) => {
  const meta = findEndpointMeta(config.url);
  const strategy = meta?.retryStrategy;
  if (!strategy) return false;
  const attempt = Number(config._retryAttempt || 0);
  if (attempt >= Number(strategy.maxAttempts || 0)) return false;

  if (isNetworkError) {
    return Boolean(strategy.retryNetworkErrors);
  }
  return typeof status === 'number' && Array.isArray(strategy.retryOnStatuses) && strategy.retryOnStatuses.includes(status);
};

const buildHeaders = (config: ApiRequestConfig, traceId: string, meta: ReturnType<typeof findEndpointMeta>): Headers => {
  const headers = new Headers();
  headers.set('Content-Type', 'application/json');

  const explicitAuthorization = config.headers?.Authorization ?? config.headers?.authorization;
  const token = useAuthStore.getState().token;
  if (token && !explicitAuthorization) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  // Accept-Language: propagate user locale to backend (supports fallback chain in ANG)
  const userLocale = (useAuthStore.getState().user as AuthStoreUserLike | null | undefined)?.locale;
  if (userLocale && typeof userLocale === 'string' && userLocale.trim()) {
    headers.set('Accept-Language', userLocale.trim());
  }

  // W3C Trace Context (OpenTelemetry compatible)
  // Format: 00-traceId(32)-spanId(16)-01
  const spanId = hex(16);
  headers.set('traceparent', `00-${traceId}-${spanId}-01`);

  // Auto-Idempotency for contract-driven endpoints.
  if (meta?.idempotent && config.method !== 'GET') {
    headers.set('Idempotency-Key', hex(32));
  }

  if (config.headers) {
    for (const [key, value] of Object.entries(config.headers)) {
      if (value !== undefined) headers.set(key, value);
    }
  }

  return headers;
};

const parseBody = async (response: Response): Promise<unknown> => {
  const contentLength = response.headers.get('content-length');
  if (response.status === 204 || contentLength === '0') return undefined;
  const text = await response.text();
  if (!text) return undefined;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
};

async function request<T>(config: ApiRequestConfig): Promise<ApiResponse<T>> {
  const traceId = hex(32);
  const meta = findEndpointMeta(config.url);

  let effectiveConfig = config;
  if (meta?.cachePolicy === 'realtime' || meta?.cachePolicy === 'bypass' || meta?.cachePolicy === 'no-store') {
    if (config.method === 'GET') {
      effectiveConfig = {
        ...config,
        params: { ...(config.params || {}), _rt: Date.now().toString() },
      };
    }
  }

  const headers = buildHeaders(effectiveConfig, traceId, meta);
  if (effectiveConfig.method === 'GET' || effectiveConfig.method === 'DELETE') {
    if ((meta?.cachePolicy === 'realtime' || meta?.cachePolicy === 'bypass' || meta?.cachePolicy === 'no-store') && effectiveConfig.method === 'GET') {
      headers.set('Cache-Control', 'no-store, no-cache, max-age=0, must-revalidate');
      headers.set('Pragma', 'no-cache');
    }
  }

  const url = buildUrl(effectiveConfig);
  const hasBody = effectiveConfig.data !== undefined && effectiveConfig.method !== 'GET';

  let response: Response;
  try {
    response = await fetch(url, {
      method: effectiveConfig.method,
      headers,
      body: hasBody ? JSON.stringify(effectiveConfig.data) : undefined,
      signal: effectiveConfig.signal,
      credentials: 'include',
    });
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw err;
    }
    // Network error — no response was ever received.
    if (shouldRetry(effectiveConfig, true)) {
      const strategy = meta?.retryStrategy;
      const attempt = Number(effectiveConfig._retryAttempt || 0);
      const delay = Math.max(0, Number(strategy?.baseDelayMs || 0)) * Math.pow(2, attempt);
      if (delay > 0) await sleep(delay);
      return request<T>({ ...effectiveConfig, _retryAttempt: attempt + 1 });
    }
    apiLogger.error('[ANG SDK] API request failed', {
      traceId,
      message: err instanceof Error ? err.message : String(err),
      status: undefined,
      url: effectiveConfig.url,
    });
    throw normalizeApiError({
      data: undefined,
      errorMessage: err instanceof Error ? err.message : 'Network Error',
      traceId,
    });
  }

  if (response.ok) {
    const data = (await parseBody(response)) as T;
    return { data, status: response.status, statusText: response.statusText, headers: response.headers };
  }

  // 401 → refresh-and-retry, once per request (mirrors the old axios response interceptor).
  if (
    response.status === 401 &&
    !effectiveConfig._retry &&
    !isRefreshRequest(effectiveConfig) &&
    useAuthStore.getState().refreshToken
  ) {
    const retryConfig: ApiRequestConfig = { ...effectiveConfig, _retry: true };
    try {
      const newToken = refreshPromise ? await waitForRefreshToken() : await refreshAuthToken();
      if (newToken) {
        retryConfig.headers = { ...(retryConfig.headers || {}), Authorization: `Bearer ${newToken}` };
      }
      return request<T>(retryConfig);
    } catch (refreshErr) {
      throw refreshErr;
    }
  }

  if (shouldRetry(effectiveConfig, false, response.status)) {
    const strategy = meta?.retryStrategy;
    const attempt = Number(effectiveConfig._retryAttempt || 0);
    const delay = Math.max(0, Number(strategy?.baseDelayMs || 0)) * Math.pow(2, attempt);
    if (delay > 0) await sleep(delay);
    return request<T>({ ...effectiveConfig, _retryAttempt: attempt + 1 });
  }

  const errorBody = await parseBody(response);
  apiLogger.error('[ANG SDK] API request failed', {
    traceId,
    message: response.statusText,
    status: response.status,
    url: effectiveConfig.url,
  });
  const problem = normalizeApiError({
    data: errorBody,
    status: response.status,
    statusText: response.statusText,
    traceId,
  });
  throw problem;
}

// Zod schemas are typed with specific key names; dynamic lookup requires a structural cast.
type _ZodSchemaLookup = Record<string, { safeParse: (value: unknown) => { success: boolean; error?: unknown } } | undefined>;
const _schemasByName = Schemas as unknown as _ZodSchemaLookup;

export const validateResponse = <T>(schemaName: string, data: T, context: string): T => {
  const schema = _schemasByName[schemaName];
  if (!schema) return data;
  const result = schema.safeParse(data);
  if (!result.success) {
    const issue = {
      schemaName,
      context,
      issues: result.error,
      data,
    };
    apiLogger.warn(`[ANG SDK] Response schema mismatch for ${context} (${schemaName})`, issue);
    if (responseValidationReporter) {
      try {
        responseValidationReporter(issue);
      } catch (reportError) {
        apiLogger.error('[ANG SDK] Response validation reporter failed', reportError);
      }
    }
  }
  return data;
};

export const isProblemDetail = (err: unknown): err is ProblemDetail => {
  return isProblemDetailLike(err);
};

export const hasErrorCode = (err: unknown, codes: ErrorCode | ErrorCode[]): boolean => {
  if (!isProblemDetail(err)) return false;
  const list = Array.isArray(codes) ? codes : [codes];
  return list.includes(err.message_code);
};

// Helper for React Hook Form
export const mapApiErrorsToForm = (problem: ProblemDetail, setError: Function) => {
  if (problem.invalidFields) {
    Object.entries(problem.invalidFields).forEach(([field, message]) => {
      setError(field, { type: 'server', message });
    });
  }
  // If no specific fields, usually you set a root error
  if (!problem.invalidFields && problem.detail) {
      setError('root', { type: 'server', message: problem.detail });
  }
};
