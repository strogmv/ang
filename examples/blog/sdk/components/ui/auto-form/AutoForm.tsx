import { Form, Field, Actions } from '@/components/ui/forms';
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
