import { createFormHookContexts } from "@tanstack/react-form";

// The contexts the form kit's field and form components read. useAppForm provides them.
export const { fieldContext, formContext, useFieldContext, useFormContext } =
  createFormHookContexts();
