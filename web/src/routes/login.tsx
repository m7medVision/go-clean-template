import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";

import { $api } from "#/api/client";
import { safeRedirect } from "#/auth/redirect";
import { loginSchema } from "#/auth/schema";
import { AuthCard } from "#/components/auth-card";
import { FormError } from "#/components/form/form-components";
import { useAppForm, zodValidation } from "#/components/form/use-app-form";

type LoginSearch = { redirect?: string };

export const Route = createFileRoute("/login")({
  validateSearch: (search: Record<string, unknown>): LoginSearch =>
    typeof search.redirect === "string" ? { redirect: search.redirect } : {},
  component: Login,
});

function Login() {
  const { auth } = Route.useRouteContext();
  const { redirect } = Route.useSearch();
  const navigate = useNavigate();

  const login = $api.useMutation("post", "/v1/auth/login", {
    onSuccess: ({ token }) => {
      auth.set(token);
      void navigate({ to: safeRedirect(redirect) });
    },
  });

  const form = useAppForm({
    defaultValues: { email: "", password: "" },
    ...zodValidation(loginSchema),
    onSubmit: ({ value }) => login.mutateAsync({ body: loginSchema.parse(value) }),
  });

  return (
    <AuthCard title="Welcome back" description="Enter your details to sign in">
      <form.AppForm>
        <form.Form>
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
                autoComplete="current-password"
                placeholder="••••••••••••"
              />
            )}
          </form.AppField>
          <FormError error={login.error} />
          <form.SubmitButton size="lg" className="w-full" pending="Signing in…">
            Sign in
          </form.SubmitButton>
        </form.Form>
      </form.AppForm>
      <p className="mt-4 text-center text-muted-foreground">
        New here?{" "}
        <Link to="/register" className="text-foreground hover:underline">
          Create an account
        </Link>
      </p>
    </AuthCard>
  );
}
