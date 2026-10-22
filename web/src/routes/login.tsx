import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";

import { $api } from "#/api/client";
import { safeRedirect } from "#/auth/redirect";
import { loginSchema } from "#/auth/schema";
import type { SignInMethod } from "#/auth/sign-in-method";
import { AuthCard } from "#/components/auth-card";
import { FormError } from "#/components/form/form-components";
import { useAppForm, zodValidation } from "#/components/form/use-app-form";
import { Button } from "#/components/ui/button";
import { FieldSeparator } from "#/components/ui/field";
import { usePasskeySignIn } from "#/passkey/use-passkey-sign-in"; // scaffold:passkeys
import { ssoStatusQuery } from "#/sso/api"; // scaffold:sso
import { useSsoSignIn } from "#/sso/use-sso-sign-in"; // scaffold:sso

// The email field's autofill tokens.
const emailAutoComplete = [
  "username",
  "webauthn", // scaffold:passkeys
].join(" ");

type LoginSearch = { redirect?: string };

export const Route = createFileRoute("/login")({
  validateSearch: (search: Record<string, unknown>): LoginSearch =>
    typeof search.redirect === "string" ? { redirect: search.redirect } : {},
  loader: ({ context }) => context.queryClient.ensureQueryData(ssoStatusQuery), // scaffold:sso
  component: Login,
});

function Login() {
  const { auth } = Route.useRouteContext();
  const { redirect } = Route.useSearch();
  const navigate = useNavigate();

  // Every sign-in that finishes on this page ends here.
  const signIn = (token: string) => {
    auth.set(token);
    void navigate({ to: safeRedirect(redirect) });
  };

  // The other ways of signing in, as buttons under the form.
  const methods: SignInMethod[] = [
    useSsoSignIn(safeRedirect(redirect)), // scaffold:sso
    usePasskeySignIn(signIn), // scaffold:passkeys
  ];
  const available = methods.filter((method) => method.available);

  const login = $api.useMutation("post", "/v1/auth/login", {
    onMutate: () => methods.forEach((method) => method.reset()),
    onSuccess: ({ token }) => signIn(token),
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
                autoComplete={emailAutoComplete}
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
          <FormError error={methods.find((method) => method.error)?.error ?? login.error} />
          <form.SubmitButton size="lg" className="w-full" pending="Signing in…">
            Sign in
          </form.SubmitButton>
        </form.Form>
      </form.AppForm>
      {available.length > 0 && <FieldSeparator className="my-4">Or</FieldSeparator>}
      <div className="flex flex-col gap-3">
        {available.map((method) => (
          <Button
            key={method.key}
            type="button"
            variant="outline"
            size="lg"
            className="w-full"
            disabled={method.pending}
            onClick={() => {
              login.reset();
              methods.filter((other) => other !== method).forEach((other) => other.reset());
              method.start();
            }}
          >
            <method.icon />
            {method.pending ? method.pendingLabel : method.label}
          </Button>
        ))}
      </div>
      <p className="mt-4 text-center text-muted-foreground">
        New here?{" "}
        <Link to="/register" className="text-foreground hover:underline">
          Create an account
        </Link>
      </p>
    </AuthCard>
  );
}
