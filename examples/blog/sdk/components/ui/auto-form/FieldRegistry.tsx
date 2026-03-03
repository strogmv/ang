import type { FieldSchema } from './types';

export type FieldRenderer = (args: { schema: FieldSchema; value: unknown }) => unknown;

export type FieldRegistry = Record<string, FieldRenderer>;
