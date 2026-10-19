import type { ComponentProps, ReactNode } from "react";

import { PasswordInput } from "#/components/password-input";
import { Field, FieldDescription, FieldError, FieldLabel } from "#/components/ui/field";
import { Input } from "#/components/ui/input";
import { Textarea } from "#/components/ui/textarea";
import { useFieldContext } from "./context";

type FieldProps = {
  label: string;
  description?: ReactNode;
  // required only marks the label; the Zod schema is what enforces it.
  required?: boolean;
};

// FieldFrame is the shadcn Field layout every input shares: label, control, hint, then the
// errors. A field shows as invalid once it was touched (or the form was submitted) and fails.
function FieldFrame({
  label,
  description,
  required,
  children,
}: FieldProps & {
  children: (state: { id: string; invalid: boolean; hint?: string }) => ReactNode;
}) {
  const field = useFieldContext<string>();
  const invalid = field.state.meta.isTouched && !field.state.meta.isValid;
  const id = field.name;
  const hint = description ? `${id}-description` : undefined;

  return (
    <Field data-invalid={invalid}>
      <FieldLabel htmlFor={id}>
        {label}
        {required && "*"}
      </FieldLabel>
      {children({ id, invalid, hint })}
      {description && <FieldDescription id={hint}>{description}</FieldDescription>}
      {invalid && <FieldError errors={field.state.meta.errors} />}
    </Field>
  );
}

type InputProps = Omit<
  ComponentProps<typeof Input>,
  "id" | "name" | "value" | "onChange" | "onBlur" | "required"
>;

export function TextField({ label, description, required, ...props }: FieldProps & InputProps) {
  const field = useFieldContext<string>();

  return (
    <FieldFrame label={label} description={description} required={required}>
      {({ id, invalid, hint }) => (
        <Input
          id={id}
          name={field.name}
          value={field.state.value}
          onBlur={field.handleBlur}
          onChange={(event) => field.handleChange(event.target.value)}
          aria-invalid={invalid}
          aria-describedby={hint}
          {...props}
        />
      )}
    </FieldFrame>
  );
}

export function PasswordField({
  label,
  description,
  required,
  ...props
}: FieldProps & Omit<InputProps, "type">) {
  const field = useFieldContext<string>();

  return (
    <FieldFrame label={label} description={description} required={required}>
      {({ id, invalid, hint }) => (
        <PasswordInput
          id={id}
          name={field.name}
          value={field.state.value}
          onBlur={field.handleBlur}
          onChange={(event) => field.handleChange(event.target.value)}
          aria-invalid={invalid}
          aria-describedby={hint}
          {...props}
        />
      )}
    </FieldFrame>
  );
}

type TextareaProps = Omit<
  ComponentProps<typeof Textarea>,
  "id" | "name" | "value" | "onChange" | "onBlur" | "required"
>;

export function TextareaField({
  label,
  description,
  required,
  ...props
}: FieldProps & TextareaProps) {
  const field = useFieldContext<string>();

  return (
    <FieldFrame label={label} description={description} required={required}>
      {({ id, invalid, hint }) => (
        <Textarea
          id={id}
          name={field.name}
          value={field.state.value}
          onBlur={field.handleBlur}
          onChange={(event) => field.handleChange(event.target.value)}
          aria-invalid={invalid}
          aria-describedby={hint}
          {...props}
        />
      )}
    </FieldFrame>
  );
}
