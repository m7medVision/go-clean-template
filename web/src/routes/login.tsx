import { useMutation, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Building2Icon, KeyRoundIcon } from "lucide-react";

import { $api } from "#/api/client";
import { safeRedirect } from "#/auth/redirect";
import { loginSchema } from "#/auth/schema";
import { AuthCard } from "#/components/auth-card";
import { FormError } from "#/components/form/form-components";
import { useAppForm, zodValidation } from "#/components/form/use-app-form";
import { Button } from "#/components/ui/button";
import { FieldSeparator } from "#/components/ui/field";
import { usePasskeyLogin } from "#/passkey/use-passkey-login";
import { passkeysSupported } from "#/passkey/webauthn";
import { beginSso, ssoStatusQuery } from "#/sso/api";

type LoginSearch = { redirect?: string };

export const Route = createFileRoute("/login")({
  validateSearch: (search: Record<string, unknown>): LoginSearch =>
    typeof search.redirect === "string" ? { redirect: search.redirect } : {},
  loader: ({ context }) => context.queryClient.ensureQueryData(ssoStatusQuery),
  component: Login,
});

function Login() {
  const { auth } = Route.useRouteContext();
  const { redirect } = Route.useSearch();
  const navigate = useNavigate();

  // Every way of signing in ends here: password and both passkey paths.
  const signIn = (token: string) => {
    auth.set(token);
    void navigate({ to: safeRedirect(redirect) });
  };

  const passkey = usePasskeyLogin(signIn);

  const { data: ssoStatus } = useSuspenseQuery(ssoStatusQuery);
  const sso = useMutation({
    ...beginSso,
    // A full page load: the provider's sign-in page, which comes back to /sso/callback.
    onSuccess: (url) => window.location.assign(url),
  });

  const login = $api.useMutation("post", "/v1/auth/login", {
    onMutate: () => passkey.reset(),
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
                autoComplete="username webauthn"
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
          <FormError error={passkey.error ?? sso.error ?? login.error} />
          <form.SubmitButton size="lg" className="w-full" pending="Signing in…">
            Sign in
          </form.SubmitButton>
        </form.Form>
      </form.AppForm>
      {(passkeysSupported() || ssoStatus.enabled) && (
        <FieldSeparator className="my-4">Or</FieldSeparator>
      )}
      {ssoStatus.enabled && (
        <Button
          type="button"
          variant="outline"
          size="lg"
          className="mb-3 w-full"
          disabled={sso.isPending || sso.isSuccess}
          onClick={() => {
            login.reset();
            passkey.reset();
            sso.mutate({ redirect: safeRedirect(redirect) });
          }}
        >
          <Building2Icon />
          {sso.isPending || sso.isSuccess ? "Redirecting…" : "Sign in with SSO"}
        </Button>
      )}
      {passkeysSupported() && (
        <>
          <Button
            type="button"
            variant="outline"
            size="lg"
            className="w-full"
            disabled={passkey.pending}
            onClick={() => {
              login.reset();
              sso.reset();
              passkey.start();
            }}
          >
            <KeyRoundIcon />
            {passkey.pending ? "Waiting for your device…" : "Sign in with a passkey"}
          </Button>
        </>
      )}
      <p className="mt-4 text-center text-muted-foreground">
        New here?{" "}
        <Link to="/register" className="text-foreground hover:underline">
          Create an account
        </Link>
      </p>
    </AuthCard>
  );
}
