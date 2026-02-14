import type { ComponentType, FormEventHandler, ReactNode } from 'react';
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
      ) : null}
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
