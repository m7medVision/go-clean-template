import type { ComponentProps, ReactNode } from "react";

import { problemMessage } from "#/api/problem";
import { Button } from "#/components/ui/button";
import { FieldGroup } from "#/components/ui/field";
import { useFormContext } from "./context";

// Form is the <form> element for a useAppForm form. Validation is Zod's, so the browser's own
// is off (noValidate) and every error renders the same way.
export function Form({ className, children }: { className?: string; children: ReactNode }) {
  const form = useFormContext();

  return (
    <form
      noValidate
      className={className}
      onSubmit={(event) => {
        event.preventDefault();
        event.stopPropagation();
        // onSubmit returns the mutation's promise; its failure is shown by <FormError>, so the
        // rethrow from handleSubmit is not an unhandled rejection.
        form.handleSubmit().catch(() => {});
      }}
    >
      <FieldGroup className="gap-4">{children}</FieldGroup>
    </form>
  );
}

// SubmitButton is disabled while the form submits and shows `pending` meanwhile.
export function SubmitButton({
  children,
  pending,
  ...props
}: Omit<ComponentProps<typeof Button>, "type"> & { pending: ReactNode }) {
  const form = useFormContext();

  return (
    <form.Subscribe selector={(state) => state.isSubmitting}>
      {(submitting) => (
        <Button type="submit" disabled={submitting} {...props}>
          {submitting ? pending : children}
        </Button>
      )}
    </form.Subscribe>
  );
}

// FormError shows why the server rejected the submission: pass the mutation's error.
export function FormError({ error }: { error: unknown }) {
  if (!error) return null;

  return (
    <p role="alert" className="text-sm text-destructive">
      {problemMessage(error)}
    </p>
  );
}
