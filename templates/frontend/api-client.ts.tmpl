import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios';
import { useAuthStore } from './auth-store';
import { ErrorCode, ProblemDetail } from './types';
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

  return config;
});

apiClient.interceptors.response.use(
  (response: any) => response,
  (error: AxiosError) => {
    const traceId = (error.config as any)?.meta?.traceId;
    
    if (traceId) {
      console.error(`[TraceID: ${traceId}] API Error:`, error.message);
    }

    if (error.response && error.response.data && typeof error.response.data === 'object') {
      // Cast to ProblemDetail if structure matches
      const problem = error.response.data as ProblemDetail;
      
      // Inject TraceID into the problem object for UI display
      (problem as any).traceId = traceId;
      
      return Promise.reject(problem);
    }
    return Promise.reject({
      title: 'Network Error',
      detail: error.message,
      status: 0,
      traceId
    } as ProblemDetail);
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
  if (!err || typeof err !== 'object') return false;
  const maybe = err as ProblemDetail;
  return typeof maybe.title === 'string' && typeof maybe.status === 'number' && typeof maybe.detail === 'string';
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
