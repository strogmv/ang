import ky from 'ky';
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
  if (viteEnv?.DEV !== undefined) return Boolean(viteEnv.DEV);
  if (typeof process !== 'undefined' && process.env?.NODE_ENV) return process.env.NODE_ENV !== 'production';
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
    if (new RegExp(`^${pattern}$`).test(pathOnly)) return meta;
  }
  return undefined;
};

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

type AuthStoreUserLike = {
  locale?: string | null;
};

type ApiClientRequestMeta = {
  traceId: string;
  endpointMeta?: ReturnType<typeof findEndpointMeta>;
  retryAttempt: number;
};

type ApiRequestConfig = {
  url: string;
  method: string;
  params?: Record<string, unknown>;
  data?: unknown;
  signal?: AbortSignal;
  headers?: HeadersInit;
  retryAttempt?: number;
};

type ApiResponse<T> = {
  data: T;
  status: number;
  statusText: string;
  headers: Headers;
  config: ApiRequestConfig;
};

type ApiClientError = Error & {
  response?: Response;
  config?: ApiRequestConfig;
  status?: number;
  data?: unknown;
  traceId?: string;
  networkError?: boolean;
  normalized?: boolean;
};

const REFRESH_ENDPOINT = '/api/auth/refresh';

const isRefreshRequest = (url: string) => url.endsWith(REFRESH_ENDPOINT) || url.includes(`${REFRESH_ENDPOINT}?`);

const joinUrl = (baseUrl: string, path: string) => {
  if (/^https?:\/\//i.test(path)) return path;
  const base = baseUrl.replace(/\/+$/, '');
  return `${base}${path.startsWith('/') ? path : `/${path}`}`;
};

const appendQuery = (url: string, params: Record<string, unknown> | undefined) => {
  if (!params) return url;
  const query = new URLSearchParams();
  for (const [key, raw] of Object.entries(params)) {
    if (raw === undefined || raw === null) continue;
    if (Array.isArray(raw)) {
      raw.forEach((item) => item !== undefined && item !== null && query.append(key, String(item)));
      continue;
    }
    query.set(key, String(raw));
  }
  const queryString = query.toString();
  if (!queryString) return url;
  return url.includes('?') ? `${url}&${queryString}` : `${url}?${queryString}`;
};

const getHeaderValue = (headers: HeadersInit | undefined, name: string) => new Headers(headers).get(name);

const isBodyInit = (value: unknown): value is BodyInit =>
  typeof value === 'string' || value instanceof FormData || value instanceof Blob || value instanceof URLSearchParams || value instanceof ArrayBuffer;

const readResponseBody = async (response: Response): Promise<unknown> => {
  if (response.status === 204 || response.status === 205) return undefined;
  const text = await response.text();
  if (!text) return undefined;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
};

const makeError = (message: string, config: ApiRequestConfig, meta: ApiClientRequestMeta, response?: Response, data?: unknown, networkError = false): ApiClientError => {
  const error = new Error(message) as ApiClientError;
  error.response = response;
  error.config = config;
  error.status = response?.status;
  error.data = data;
  error.traceId = meta.traceId;
  error.networkError = networkError;
  return error;
};

const shouldRetry = (meta: ApiClientRequestMeta, status?: number, networkError = false) => {
  const strategy = meta.endpointMeta?.retryStrategy;
  if (!strategy || meta.retryAttempt >= Number(strategy.maxAttempts || 0)) return false;
  if (networkError) return Boolean(strategy.retryNetworkErrors);
  return typeof status === 'number' && Array.isArray(strategy.retryOnStatuses) && strategy.retryOnStatuses.includes(status);
};

const retryDelay = (meta: ApiClientRequestMeta) => {
  const baseDelay = Math.max(0, Number(meta.endpointMeta?.retryStrategy?.baseDelayMs || 0));
  return baseDelay * Math.pow(2, meta.retryAttempt);
};

const normalizeError = (error: ApiClientError): ProblemDetail => {
  const problem = normalizeApiError({
    data: error.data,
    status: error.response?.status ?? error.status,
    statusText: error.response?.statusText,
    errorMessage: error.message,
    traceId: error.traceId,
  }) as ProblemDetail & ApiClientError;
  problem.status = error.response?.status ?? error.status;
  problem.traceId = error.traceId;
  problem.normalized = true;
  return problem;
};

let refreshPromise: Promise<string> | null = null;

const shouldClearAuthOnRefreshFailure = (error: unknown) => {
  const status = Number((error as ApiClientError | null)?.status || 0);
  return status === 401 || status === 403;
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
      .post<Types.RefreshTokenResponse>(REFRESH_ENDPOINT, { refreshToken: storedRefresh }, { skipAuthRefresh: true })
      .then((response) => {
        const data = response.data;
        if (import.meta.env.DEV) validateResponse('RefreshTokenResponseSchema', data, 'RefreshToken');
        store.setAuth(data.accessToken, store.user, data.refreshToken);
        return data.accessToken;
      })
      .catch((error) => {
        if (shouldClearAuthOnRefreshFailure(error)) store.clearAuth();
        throw error;
      })
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
};

class KyApiClient {
  defaults = { baseURL: getBaseUrl() };

  get<T>(url: string, config: Omit<ApiRequestConfig, 'url' | 'method'> = {}) {
    return this.request<T>({ ...config, url, method: 'GET' });
  }

  post<T>(url: string, data?: unknown, config: Omit<ApiRequestConfig, 'url' | 'method' | 'data'> & { skipAuthRefresh?: boolean } = {}) {
    return this.request<T>({ ...config, url, method: 'POST', data });
  }

  put<T>(url: string, data?: unknown, config: Omit<ApiRequestConfig, 'url' | 'method' | 'data'> = {}) {
    return this.request<T>({ ...config, url, method: 'PUT', data });
  }

  patch<T>(url: string, data?: unknown, config: Omit<ApiRequestConfig, 'url' | 'method' | 'data'> = {}) {
    return this.request<T>({ ...config, url, method: 'PATCH', data });
  }

  delete<T>(url: string, config: Omit<ApiRequestConfig, 'url' | 'method'> = {}) {
    return this.request<T>({ ...config, url, method: 'DELETE' });
  }

  async request<T>(input: ApiRequestConfig & { skipAuthRefresh?: boolean }): Promise<ApiResponse<T>> {
    const config: ApiRequestConfig & { skipAuthRefresh?: boolean } = { ...input };
    let refreshed = false;

    while (true) {
      const attempt = Number(config.retryAttempt || 0);
      const endpoint = findEndpointMeta(config.url);
      const meta: ApiClientRequestMeta = {
        traceId: `${hex(32)}-${hex(16)}`,
        endpointMeta: endpoint,
        retryAttempt: attempt,
      };
      const headers = new Headers(config.headers);
      const token = useAuthStore.getState().token;
      const hasExplicitAuthorization = Boolean(getHeaderValue(config.headers, 'Authorization'));
      if (token && !hasExplicitAuthorization) headers.set('Authorization', `Bearer ${token}`);

      const userLocale = (useAuthStore.getState().user as AuthStoreUserLike | null | undefined)?.locale;
      if (userLocale?.trim()) headers.set('Accept-Language', userLocale.trim());
      headers.set('traceparent', `00-${meta.traceId.slice(0, 32)}-${meta.traceId.slice(33)}-01`);

      let requestUrl = appendQuery(joinUrl(this.defaults.baseURL, config.url), config.params);
      if ((endpoint?.cachePolicy === 'realtime' || endpoint?.cachePolicy === 'bypass' || endpoint?.cachePolicy === 'no-store') && config.method === 'GET') {
        requestUrl = appendQuery(requestUrl, { _rt: Date.now().toString() });
        headers.set('Cache-Control', 'no-store, no-cache, max-age=0, must-revalidate');
        headers.set('Pragma', 'no-cache');
      }
      if (endpoint?.idempotent && config.method !== 'GET' && !headers.has('Idempotency-Key')) headers.set('Idempotency-Key', hex(32));

      const kyOptions: Parameters<typeof ky>[1] = {
        method: config.method,
        headers,
        credentials: 'include',
        signal: config.signal,
        retry: 0,
        throwHttpErrors: false,
      };
      if (config.data !== undefined) {
        if (isBodyInit(config.data)) {
          kyOptions.body = config.data;
        } else {
          kyOptions.body = JSON.stringify(config.data);
          if (!headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
        }
      }
      config.headers = headers;

      let response: Response;
      let data: unknown;
      try {
        response = await ky(requestUrl, kyOptions);
        data = await readResponseBody(response);
      } catch (cause) {
        if (config.signal?.aborted) throw cause;
        const error = makeError(cause instanceof Error ? cause.message : 'Network request failed', config, meta, undefined, undefined, true);
        if (shouldRetry(meta, undefined, true)) {
          await sleep(retryDelay(meta));
          config.retryAttempt = attempt + 1;
          continue;
        }
        apiLogger.error('[ANG SDK] API request failed', { traceId: meta.traceId, message: error.message, url: config.url });
        throw normalizeError(error);
      }

      if (response.ok) {
        return { data: data as T, status: response.status, statusText: response.statusText, headers: response.headers, config };
      }

      const error = makeError(`Request failed with status ${response.status}`, config, meta, response, data);
      if (response.status === 401 && !refreshed && !config.skipAuthRefresh && !isRefreshRequest(config.url) && useAuthStore.getState().refreshToken) {
        refreshed = true;
        await refreshAuthToken();
        continue;
      }
      if (shouldRetry(meta, response.status, false)) {
        await sleep(retryDelay(meta));
        config.retryAttempt = attempt + 1;
        continue;
      }

      apiLogger.error('[ANG SDK] API request failed', {
        traceId: meta.traceId,
        message: error.message,
        status: response.status,
        url: config.url,
      });
      throw normalizeError(error);
    }
  }
}

export const apiClient = new KyApiClient();

// Zod schemas are typed with specific key names; dynamic lookup requires a structural cast.
type _ZodSchemaLookup = Record<string, { safeParse: (value: unknown) => { success: boolean; error?: unknown } } | undefined>;
const _schemasByName = Schemas as unknown as _ZodSchemaLookup;

export const validateResponse = <T>(schemaName: string, data: T, context: string): T => {
  const schema = _schemasByName[schemaName];
  if (!schema) return data;
  const result = schema.safeParse(data);
  if (!result.success) {
    const issue = { schemaName, context, issues: result.error, data };
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

export const isProblemDetail = (err: unknown): err is ProblemDetail => isProblemDetailLike(err);

export const hasErrorCode = (err: unknown, codes: ErrorCode | ErrorCode[]): boolean => {
  if (!isProblemDetail(err)) return false;
  const list = Array.isArray(codes) ? codes : [codes];
  return list.includes(err.message_code);
};

export const mapApiErrorsToForm = (problem: ProblemDetail, setError: Function) => {
  if (problem.invalidFields) {
    Object.entries(problem.invalidFields).forEach(([field, message]) => {
      setError(field, { type: 'server', message });
    });
  }
  if (!problem.invalidFields && problem.detail) {
    setError('root', { type: 'server', message: problem.detail });
  }
};
