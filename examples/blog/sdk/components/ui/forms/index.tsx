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

const standardFieldSx = {
  '& .MuiOutlinedInput-root': {
    minHeight: 44,
    borderRadius: '4px',
    backgroundColor: 'background.paper',
    '& .MuiOutlinedInput-notchedOutline': { borderColor: 'divider' },
    '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: 'divider' },
    '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: 'warning.main', borderWidth: 1 },
  },
  '& .MuiOutlinedInput-input': {
    fontSize: '1.4rem',
    lineHeight: '2.2rem',
    letterSpacing: '-0.28px',
    py: '1.1rem',
  },
  '& .MuiInputBase-input::placeholder': { color: 'text.secondary', opacity: 1 },
  '& .MuiInputLabel-root': { color: 'text.primary', fontSize: '1.4rem' },
  '& .MuiFormHelperText-root': { mx: 0, mt: 0.5, color: 'text.secondary', fontSize: '1.2rem', lineHeight: '2rem' },
};

const standardActionSx = {
  minHeight: 44,
  minWidth: '16rem',
  borderRadius: '8px',
  textTransform: 'none',
  fontSize: '1.6rem',
  fontWeight: 500,
  lineHeight: '2.2rem',
  boxShadow: 'none',
  '&:hover': { boxShadow: 'none' },
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
    sx={standardFieldSx}
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
    sx={standardFieldSx}
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
      <Stack spacing={2.4}>{children}</Stack>
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
  const intentColor =
    intent === 'danger' ? 'error.main' :
    intent === 'warning' ? 'warning.main' :
    intent === 'success' ? 'success.main' :
    intent === 'info' ? 'info.main' : 'transparent';
  const registryKey = String(type || 'text').toLowerCase();
  const Renderer = FieldRegistry[registryKey] || FieldRegistry.text;

  return (
    <Box sx={{ width: '100%', maxWidth: columns > 1 ? String(100 / columns) + '%' : '100%', borderLeft: importance === 'high' ? 3 : 0, borderColor: intentColor, pl: importance === 'high' ? 1 : 0 }}>
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
    <Stack direction={{ xs: 'column-reverse', md: 'row' }} spacing={2} sx={{ justifyContent: 'flex-end', alignItems: { xs: 'stretch', md: 'center' } }}>
      {onCancel && (
        <Button variant="contained" color="inherit" onClick={onCancel} disabled={isPending} sx={{ ...standardActionSx, bgcolor: 'action.selected', color: 'text.primary', '&:hover': { bgcolor: 'action.hover', boxShadow: 'none' } }}>
          {cancelLabel}
        </Button>
      )}
      <Button type="submit" variant="contained" color="warning" disabled={isPending} sx={standardActionSx}>
        {isPending ? loadingLabel : submitLabel}
      </Button>
    </Stack>
  );
}
