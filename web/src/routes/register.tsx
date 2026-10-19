import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";

import { $api } from "#/api/client";
import { minPassword, registerSchema } from "#/auth/schema";
import { AuthCard } from "#/components/auth-card";
import { FormError } from "#/components/form/form-components";
import { useAppForm, zodValidation } from "#/components/form/use-app-form";

export const Route = createFileRoute("/register")({ component: Register });

function Register() {
  const navigate = useNavigate();

  const register = $api.useMutation("post", "/v1/auth/register", {
    onSuccess: () => void navigate({ to: "/login" }),
  });

  const form = useAppForm({
    defaultValues: { username: "", email: "", password: "" },
    ...zodValidation(registerSchema),
    onSubmit: ({ value }) => register.mutateAsync({ body: registerSchema.parse(value) }),
  });

  return (
    <AuthCard title="Create an account" description="Enter your details to get started">
      <form.AppForm>
        <form.Form>
          <form.AppField name="username">
            {(field) => (
              <field.TextField
                label="Username"
                required
                autoComplete="username"
                placeholder="Choose a username"
              />
            )}
          </form.AppField>
          <form.AppField name="email">
            {(field) => (
              <field.TextField
                label="Email address"
                required
                type="email"
                autoComplete="email"
                placeholder="Enter your email address"
              />
            )}
          </form.AppField>
          <form.AppField name="password">
            {(field) => (
              <field.PasswordField
                label="Password"
                required
                autoComplete="new-password"
                placeholder="••••••••••••"
                description={`At least ${minPassword} characters.`}
              />
            )}
          </form.AppField>
          <FormError error={register.error} />
          <form.SubmitButton size="lg" className="w-full" pending="Creating account…">
            Create account
          </form.SubmitButton>
        </form.Form>
      </form.AppForm>
      <p className="mt-4 text-center text-muted-foreground">
        Already have an account?{" "}
        <Link to="/login" className="text-foreground hover:underline">
          Sign in
        </Link>
      </p>
    </AuthCard>
  );
}
