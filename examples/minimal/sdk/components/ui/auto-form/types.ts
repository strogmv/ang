import type { ComponentType } from 'react';
import type { UseFormReturn } from 'react-hook-form';

export type UIHints = {
  type?: string;
  importance?: string;
  inputKind?: string;
  intent?: string;
  density?: string;
  labelMode?: string;
  surface?: string;
  placeholder?: string;
  helperText?: string;
  rows?: number;
  min?: number;
  max?: number;
  currency?: string;
  source?: string;
  multiple?: boolean;
  accept?: string;
  disabled?: boolean;
  required?: boolean;
  fullWidth?: boolean;
  hidden?: boolean;
  columns?: number;
  component?: string;
  section?: string;
};

export type FieldSchema<TValues = any> = {
  name: keyof TValues & string;
  label: string;
  type: string;
  required?: boolean;
  options?: string[];
  ui?: UIHints;
  component?: ComponentType<any>;
};

export type FormSchema<TValues = any> = {
  schemaVersion: 1;
  fields: Array<FieldSchema<TValues>>;
  layout?: {
    type?: 'stack' | 'grid';
    columns?: number;
  };
};

export type AutoFormProps<TValues = any> = {
  form: UseFormReturn<TValues>;
  schema: FormSchema<TValues>;
  onSubmit: (values: TValues) => void;
  isPending?: boolean;
  onCancel?: () => void;
  submitLabel?: string;
  loadingLabel?: string;
  cancelLabel?: string;
};
