import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios';
import { useAuthStore } from './auth-store';
import { endpointMeta } from './endpoints';
import { ErrorCode, ProblemDetail } from './types';
import { isProblemDetailLike, normalizeApiError } from './error-normalizer';
import * as Schemas from './schemas';

const getBaseUrl = () => {
  try {
    // @ts-ignore
    if (import.meta.env?.VITE_API_URL) return import.meta.env.VITE_API_URL;
  } catch (e) {}
  
  if (typeof process !== 'undefined' && process.env?.VITE_API_URL) return process.env.VITE_API_URL;
  return 'http://localhost:8080';
};

const isDevEnv = () => {
  try {
    // @ts-ignore
    if (typeof import.meta !== 'undefined' && import.meta.env?.DEV !== undefined) {
      // @ts-ignore
      return Boolean(import.meta.env.DEV);
    }
  } catch (e) {}
  if (typeof process !== 'undefined' && process.env?.NODE_ENV) {
    return process.env.NODE_ENV !== 'production';
  }
  return true;
};

export const apiClient = axios.create({
  baseURL: getBaseUrl(),
  headers: {
    'Content-Type': 'application/json',
  },
});

/**
 * Interface for Client-Side Encryption.
 * If provided, SDK will use it to handle fields marked as Encrypted<T>.
 */
export interface CryptoProvider {
  encrypt(data: any): Promise<string>;
  decrypt(encrypted: string): Promise<any>;
}

let cryptoProvider: CryptoProvider | null = null;

export const setCryptoProvider = (provider: CryptoProvider) => {
  cryptoProvider = provider;
};

// Helper: Generate hex string
const hex = (len: number) => {
  const arr = new Uint8Array(len / 2);
  if (typeof window !== 'undefined' && window.crypto) {
    window.crypto.getRandomValues(arr);
  } else if (typeof global !== 'undefined' && (global as any).crypto?.getRandomValues) {
    (global as any).crypto.getRandomValues(arr);
  } else if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
    crypto.getRandomValues(arr);
  } else {
    // Fallback for Node.js or environments without crypto.getRandomValues
    for (let i = 0; i < arr.length; i++) {
      arr[i] = Math.floor(Math.random() * 256);
    }
  }
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

// 1. JWT & Trace Context Interceptor
apiClient.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = useAuthStore.getState().token;
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`;
  }

  // W3C Trace Context (OpenTelemetry compatible)
  // Format: 00-traceId(32)-spanId(16)-01
  const traceId = hex(32);
  const spanId = hex(16);
  const traceparent = `00-${traceId}-${spanId}-01`;
  
  config.headers.set('traceparent', traceparent);
  
  // Attach traceId to config for logging in response interceptor
  (config as any).meta = { traceId };

  // Auto-Idempotency for contract-driven endpoints.
  const meta = findEndpointMeta(config.url);
  if (meta?.idempotent && config.method?.toUpperCase() !== 'GET') {
    if (!config.headers.get('Idempotency-Key')) {
      config.headers.set('Idempotency-Key', hex(32));
    }
  }

  return config;
});

apiClient.interceptors.response.use(
  (response: any) => response,
  (error: AxiosError) => {
    const traceId = (error.config as any)?.meta?.traceId;
    
    if (traceId) {
      console.error(`[TraceID: ${traceId}] API Error:`, error.message);
    }

    const problem = normalizeApiError({
      data: error.response?.data,
      status: error.response?.status,
      statusText: error.response?.statusText,
      errorMessage: error.message,
      traceId,
    });
    return Promise.reject(problem);
  }
);

export const validateResponse = <T>(schemaName: string, data: T, context: string): T => {
  if (!isDevEnv()) return data;
  const schema = (Schemas as any)[schemaName];
  if (!schema || typeof schema.safeParse !== 'function') return data;
  const result = schema.safeParse(data);
  if (!result.success) {
    console.warn(`[ANG SDK] Response schema mismatch for ${context} (${schemaName})`, result.error);
  }
  return data;
};

export const isProblemDetail = (err: unknown): err is ProblemDetail => {
  return isProblemDetailLike(err);
};

export const hasErrorCode = (err: unknown, codes: ErrorCode | ErrorCode[]): boolean => {
  if (!isProblemDetail(err)) return false;
  const list = Array.isArray(codes) ? codes : [codes];
  return list.includes(err.code);
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
