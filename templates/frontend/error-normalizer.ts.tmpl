import { ErrorCode, ProblemDetail } from './types';

type AnyRecord = Record<string, unknown>;

const isRecord = (value: unknown): value is AnyRecord =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim() !== '' ? value : undefined;

const asNumber = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const normalizeFieldErrors = (value: unknown): Record<string, string> | undefined => {
  if (!isRecord(value)) return undefined;
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(value)) {
    if (typeof v === 'string') {
      out[k] = v;
      continue;
    }
    if (isRecord(v) && typeof v.message === 'string') {
      out[k] = v.message;
    }
  }
  return Object.keys(out).length > 0 ? out : undefined;
};

const inferMessageCode = (value: unknown): ErrorCode => {
  if (typeof value !== 'string') return ErrorCode.INTERNAL_ERROR;
  const normalized = value.trim().toUpperCase();
  if (normalized in ErrorCode) {
    return ErrorCode[normalized as keyof typeof ErrorCode];
  }
  return ErrorCode.INTERNAL_ERROR;
};

export type NormalizeApiErrorInput = {
  data: unknown;
  status?: number;
  statusText?: string;
  errorMessage?: string;
  traceId?: string;
};

export const normalizeProblemDetail = (
  raw: unknown,
  fallback: {
    status?: number;
    title?: string;
    detail?: string;
    traceId?: string;
  } = {},
): ProblemDetail => {
  const fallbackStatus = fallback.status ?? 0;
  const fallbackTitle = fallback.title || (fallbackStatus > 0 ? 'Request Failed' : 'Network Error');
  const fallbackDetail = fallback.detail || '';

  if (typeof raw === 'string') {
    return {
      type: 'about:blank',
      title: fallbackTitle,
      status: fallbackStatus,
      detail: raw || fallbackDetail,
      code: fallbackStatus,
      message_code: ErrorCode.INTERNAL_ERROR,
      traceId: fallback.traceId,
    };
  }

  if (isRecord(raw)) {
    const status = asNumber(raw.status) ?? fallbackStatus;
    const title = asString(raw.title) || asString(raw.error) || fallbackTitle;
    const detail =
      asString(raw.detail) ||
      asString(raw.message) ||
      asString(raw.error_description) ||
      fallbackDetail;
    const invalidFields =
      normalizeFieldErrors(raw.invalidFields) ||
      normalizeFieldErrors(raw.errors) ||
      normalizeFieldErrors(raw.fieldErrors);

    return {
      type: asString(raw.type) || 'about:blank',
      title,
      status,
      detail,
      instance: asString(raw.instance),
      code: asNumber(raw.code) ?? status,
      message_code: inferMessageCode(raw.message_code ?? raw.messageCode ?? raw.code),
      traceId: asString(raw.traceId) || fallback.traceId,
      invalidFields,
    };
  }

  return {
    type: 'about:blank',
    title: fallbackTitle,
    status: fallbackStatus,
    detail: fallbackDetail,
    code: fallbackStatus,
    message_code: ErrorCode.INTERNAL_ERROR,
    traceId: fallback.traceId,
  };
};

export const normalizeApiError = (input: NormalizeApiErrorInput): ProblemDetail =>
  normalizeProblemDetail(input.data, {
    status: input.status,
    title: input.statusText || 'Request Failed',
    detail: input.errorMessage || '',
    traceId: input.traceId,
  });

export const isProblemDetailLike = (value: unknown): value is ProblemDetail => {
  if (!isRecord(value)) return false;
  return typeof value.title === 'string' && typeof value.status === 'number' && typeof value.detail === 'string';
};

