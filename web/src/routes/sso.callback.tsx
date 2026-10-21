import { useMutation } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useRef } from "react";

import { safeRedirect } from "#/auth/redirect";
import { AuthCard } from "#/components/auth-card";
import { FormError } from "#/components/form/form-components";
import { finishSso } from "#/sso/api";
import { type CallbackSearch, callbackProblem } from "#/sso/callback";

const text = (value: unknown) => (typeof value === "string" && value !== "" ? value : undefined);

export const Route = createFileRoute("/sso/callback")({
  validateSearch: (search: Record<string, unknown>): CallbackSearch => ({
    code: text(search.code),
    state: text(search.state),
    error: text(search.error),
    error_description: text(search.error_description),
  }),
  component: SsoCallback,
});

function SsoCallback() {
  const { auth } = Route.useRouteContext();
  const search = Route.useSearch();
  const { code, state } = search;
  const navigate = useNavigate();

  const finish = useMutation({
    ...finishSso,
    onSuccess: ({ token, redirect }) => {
      auth.set(token);
      void navigate({ to: safeRedirect(redirect), replace: true });
    },
  });

  // A code works once, so finish exactly once, even when StrictMode mounts the component twice.
  const started = useRef(false);
  const { mutate } = finish;

  useEffect(() => {
    if (started.current || !code || !state) return;

    started.current = true;
    mutate({ code, state });
  }, [code, state, mutate]);

  const refused = callbackProblem(search, finish.error);

  if (!refused) {
    return (
      <AuthCard title="Signing you in" description="Finishing your SSO sign-in…">
        <p className="text-center text-muted-foreground" role="status">
          One moment…
        </p>
      </AuthCard>
    );
  }

  return (
    <AuthCard title="Sign-in failed" description="Your SSO sign-in did not complete">
      <FormError error={refused} />
      <p className="mt-4 text-center">
        <Link to="/login" className="text-foreground hover:underline">
          Back to sign in
        </Link>
      </p>
    </AuthCard>
  );
}
