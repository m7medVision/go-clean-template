import { useMutation, useSuspenseQuery } from "@tanstack/react-query";
import { Building2Icon } from "lucide-react";

import type { SignInMethod } from "#/auth/sign-in-method";
import { beginSso, ssoStatusQuery } from "./api";

// useSsoSignIn is the "Sign in with SSO" button, shown when the server has a provider. `redirect`
// is where to land once signed in; the login route loads ssoStatusQuery first.
export function useSsoSignIn(redirect: string): SignInMethod {
  const { data: status } = useSuspenseQuery(ssoStatusQuery);
  const sso = useMutation({
    ...beginSso,
    // A full page load: the provider's sign-in page, which comes back to /sso/callback.
    onSuccess: (url) => window.location.assign(url),
  });

  return {
    key: "sso",
    available: status.enabled,
    icon: Building2Icon,
    label: "Sign in with SSO",
    pendingLabel: "Redirecting…",
    // Still pending while the browser leaves for the provider.
    pending: sso.isPending || sso.isSuccess,
    error: sso.error,
    start: () => sso.mutate({ redirect }),
    reset: sso.reset,
  };
}
