package emitter

import (
	"fmt"
	"os"
	"path/filepath"
)

func (e *Emitter) emitBaseUIFormsProxyLayer() error {
	if e.resolvedUIProviderPath() != DefaultUIProviderPath {
		// Custom UI skin is owned by the host application.
		return nil
	}

	paths := []string{
		filepath.Join(e.FrontendDir, "components", "ui", "forms"),
		filepath.Join(e.FrontendDir, "@ui", "forms"),
	}

	const indexTSX = `import type { ComponentType, FormEventHandler, ReactNode } from 'react';
import { useState } from 'react';
import { Controller } from 'react-hook-form';
import { Box, Button, Checkbox, FormControlLabel, MenuItem, Stack, Switch, TextField } from '@mui/material';

type UIHints = {
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

type FormProps = {
  children: ReactNode;
  onSubmit: FormEventHandler<HTMLFormElement>;
};

type FieldProps = {
  children?: ReactNode;
  control: any;
  name: string;
  label: string;
  type?: string;
  required?: boolean;
  options?: string[];
  ui?: UIHints;
  component?: ComponentType<any>;
};

type ActionsProps = {
  isPending?: boolean;
  onCancel?: () => void;
  submitLabel?: string;
  loadingLabel?: string;
  cancelLabel?: string;
};

type RegistryFieldProps = {
  field: any;
  fieldState: any;
  label: string;
  type?: string;
  required?: boolean;
  options?: string[];
  ui?: UIHints;
};

const MuiTextField: ComponentType<RegistryFieldProps> = ({ field, fieldState, label, type = 'text', required, ui }) => (
  <TextField
    {...field}
    type={type === 'custom' ? 'text' : type}
    label={label}
    placeholder={ui?.placeholder}
    fullWidth={ui?.fullWidth ?? true}
    required={required}
    multiline={type === 'textarea'}
    rows={type === 'textarea' ? ui?.rows || 4 : undefined}
    error={!!fieldState?.error}
    helperText={fieldState?.error?.message || ui?.helperText}
    disabled={ui?.disabled}
  />
);

const MuiSelectField: ComponentType<RegistryFieldProps> = ({ field, fieldState, label, required, options = [], ui }) => (
  <TextField
    {...field}
    select
    label={label}
    fullWidth={ui?.fullWidth ?? true}
    required={required}
    error={!!fieldState?.error}
    helperText={fieldState?.error?.message || ui?.helperText}
    disabled={ui?.disabled}
  >
    {options.map((opt) => (
      <MenuItem key={opt} value={opt}>
        {opt}
      </MenuItem>
    ))}
  </TextField>
);

const MuiCheckboxField: ComponentType<RegistryFieldProps> = ({ field, label }) => (
  <FormControlLabel control={<Checkbox {...field} checked={!!field?.value} />} label={label} />
);

const MuiSwitchField: ComponentType<RegistryFieldProps> = ({ field, label }) => (
  <FormControlLabel control={<Switch {...field} checked={!!field?.value} />} label={label} />
);

export const FieldRegistry: Record<string, ComponentType<RegistryFieldProps>> = {
  text: MuiTextField,
  textarea: MuiTextField,
  number: MuiTextField,
  email: MuiTextField,
  password: MuiTextField,
  phone: MuiTextField,
  url: MuiTextField,
  date: MuiTextField,
  datetime: MuiTextField,
  time: MuiTextField,
  currency: MuiTextField,
  file: MuiTextField,
  image: MuiTextField,
  custom_map: MuiTextField,
  select: MuiSelectField,
  autocomplete: MuiTextField,
  checkbox: MuiCheckboxField,
  switch: MuiSwitchField,
};

export function registerFieldRenderer(kind: string, renderer: ComponentType<RegistryFieldProps>) {
  const key = String(kind || '').trim().toLowerCase();
  if (!key) return;
  FieldRegistry[key] = renderer;
}

export function Form({ children, onSubmit }: FormProps) {
  return (
    <Box component="form" onSubmit={onSubmit} noValidate>
      <Stack spacing={3}>{children}</Stack>
    </Box>
  );
}

export function Field(props: FieldProps) {
  const { control, name, label, type = 'text', required, options = [], ui, component: CustomComponent } = props;
  const [sensitiveVisible, setSensitiveVisible] = useState(false);
  const columns = ui?.columns && ui.columns > 0 ? ui.columns : 1;
  if (ui?.hidden) return null;
  const effectiveType = ui?.inputKind === 'sensitive' ? (sensitiveVisible ? 'text' : 'password') : type;
  const intent = String(ui?.intent || '').toLowerCase();
  const importance = String(ui?.importance || '').toLowerCase();
  const borderColor =
    intent === 'danger' ? '#d32f2f' :
    intent === 'warning' ? '#ed6c02' :
    intent === 'success' ? '#2e7d32' :
    intent === 'info' ? '#0288d1' : '#e0e0e0';
  const registryKey = String(type || 'text').toLowerCase();
  const Renderer = FieldRegistry[registryKey] || FieldRegistry.text;

  return (
    <Box sx={{ width: '100%', maxWidth: columns > 1 ? String(100 / columns) + '%' : '100%', borderLeft: importance === 'high' ? '3px solid ' + borderColor : 'none', pl: importance === 'high' ? 1 : 0 }}>
      <Controller
        name={name}
        control={control}
        render={({ field, fieldState }) => {
          if (CustomComponent) {
            return <CustomComponent {...field} label={label} ui={ui} error={fieldState?.error?.message} />;
          }
          return <Renderer field={field} fieldState={fieldState} label={label} type={effectiveType} required={required} options={options} ui={ui} />;
        }}
      />
      {ui?.inputKind === 'sensitive' ? (
        <Button size="small" variant="text" onClick={() => setSensitiveVisible((v) => !v)}>
          {sensitiveVisible ? 'Hide' : 'Show'}
        </Button>
      ) : nil}
    </Box>
  );
}

export function Actions({
  isPending,
  onCancel,
  submitLabel = 'Сохранить',
  loadingLabel = 'Сохранение...',
  cancelLabel = 'Отмена',
}: ActionsProps) {
  return (
    <Stack direction="row" spacing={2} justifyContent="flex-end">
      {onCancel && (
        <Button variant="outlined" onClick={onCancel} disabled={isPending}>
          {cancelLabel}
        </Button>
      )}
      <Button type="submit" variant="contained" disabled={isPending}>
        {isPending ? loadingLabel : submitLabel}
      </Button>
    </Stack>
  );
}
`
	for _, baseDir := range paths {
		if err := os.MkdirAll(baseDir, 0o755); err != nil {
			return err
		}
		if err := WriteFileIfChanged(filepath.Join(baseDir, "index.tsx"), []byte(indexTSX), 0o644); err != nil {
			return err
		}
		fmt.Printf("Generated Base UI Forms Layer: %s\n", filepath.Join(baseDir, "index.tsx"))
	}
	return nil
}

func (e *Emitter) emitBaseUIAutoFormLayer() error {
	baseDir := filepath.Join(e.FrontendDir, "components", "ui", "auto-form")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}
	uiProviderPath := e.resolvedUIProviderPath()

	files := map[string]string{
		"types.ts": `import type { ComponentType } from 'react';
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
`,
		"FieldRegistry.tsx": `import type { FieldSchema } from './types';

export type FieldRenderer = (args: { schema: FieldSchema; value: unknown }) => unknown;

export type FieldRegistry = Record<string, FieldRenderer>;
`,
		"LayoutRenderer.tsx": `import type { ReactNode } from 'react';

type Props = {
  children: ReactNode;
  columns?: number;
  type?: 'stack' | 'grid';
};

export function LayoutRenderer({ children, columns = 1, type = 'stack' }: Props) {
  if (type === 'grid') {
    const width = Math.max(1, Math.floor(100 / Math.max(1, columns)));
    return <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, width: '100%' }}>{children}</div>;
  }
  return <div style={{ display: 'grid', gap: 16, width: '100%' }}>{children}</div>;
}
`,
		"AutoForm.tsx": "import { Form, Field, Actions } from '" + uiProviderPath + `';
import type { AutoFormProps } from './types';

export function AutoForm<TValues = any>({
  form,
  schema,
  onSubmit,
  isPending,
  onCancel,
  submitLabel = 'Сохранить',
  loadingLabel = 'Сохранение...',
  cancelLabel = 'Отмена',
}: AutoFormProps<TValues>) {
  const grouped = schema.fields.reduce<Record<string, typeof schema.fields>>((acc, field) => {
    const key = field.ui?.section?.trim() || '_default';
    acc[key] = acc[key] || [];
    acc[key].push(field);
    return acc;
  }, {});
  const sections = Object.entries(grouped);

  return (
    <Form onSubmit={form.handleSubmit(onSubmit as any)}>
      {sections.map(([section, fields]) => (
        <div key={section} style={{ width: '100%' }}>
            {section !== '_default' ? (
              <h4 style={{ margin: 0, marginBottom: 10, fontSize: '0.95rem', fontWeight: 600 }}>
                {section}
              </h4>
            ) : null}
            {fields.map((f) => {
              if (f.ui?.hidden) return null;
              return (
                <Field
                  key={f.name}
                  control={form.control}
                  name={f.name}
                  label={f.label}
                  type={f.type}
                  required={f.required}
                  options={f.options}
                  ui={f.ui}
                  component={f.component}
                />
              );
            })}
        </div>
      ))}
      <Actions
        isPending={isPending}
        onCancel={onCancel}
        submitLabel={submitLabel}
        loadingLabel={loadingLabel}
        cancelLabel={cancelLabel}
      />
    </Form>
  );
}
`,
		"index.ts": `export { AutoForm } from './AutoForm';
export type { AutoFormProps, FieldSchema, FormSchema, UIHints } from './types';
`,
	}

	for name, content := range files {
		if err := WriteFileIfChanged(filepath.Join(baseDir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("Generated AutoForm UI Layer: %s\n", baseDir)
	return nil
}

func (e *Emitter) emitFrontendTSConfig() error {
	path := filepath.Join(e.FrontendDir, "tsconfig.json")
	const content = `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "lib": ["ES2020", "DOM"],
    "jsx": "react-jsx",
    "strict": true,
    "moduleResolution": "Bundler",
    "skipLibCheck": true,
    "resolveJsonModule": true,
    "allowSyntheticDefaultImports": true,
    "esModuleInterop": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["./*"]
    }
  },
  "include": ["./**/*.ts", "./**/*.tsx"],
  "exclude": ["node_modules", "dist"]
}
`
	if err := WriteFileIfChanged(path, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Printf("Generated Frontend TSConfig: %s\n", path)
	return nil
}
