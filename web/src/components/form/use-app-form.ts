import { createFormHook, revalidateLogic } from "@tanstack/react-form";
import type { StandardSchemaV1 } from "@tanstack/react-form";

import { fieldContext, formContext } from "./context";
import { PasswordField, TextareaField, TextField } from "./fields";
import { Form, SubmitButton } from "./form-components";

// useAppForm is the only way to build a form: TanStack Form state, shadcn Field components
// (form.AppField → field.TextField / PasswordField / TextareaField), and form.Form /
// form.SubmitButton inside form.AppForm.
export const { useAppForm, withForm } = createFormHook({
  fieldContext,
  formContext,
  fieldComponents: { TextField, PasswordField, TextareaField },
  formComponents: { Form, SubmitButton },
});

// zodValidation validates the whole form with one Zod schema: first on submit, then on every
// change, so an error clears as soon as it is fixed. Spread it into useAppForm's options.
export function zodValidation<TSchema extends StandardSchemaV1>(schema: TSchema) {
  return {
    validationLogic: revalidateLogic({ mode: "submit", modeAfterSubmission: "change" }),
    validators: { onDynamic: schema },
  };
}
