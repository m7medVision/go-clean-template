// REAUTH_REQUIRED is the detail of the 401 the server answers when an action needs a recent
// sign-in (adding a passkey) and the token is older than WEBAUTHN_REAUTH_MAX_AGE. It is a stable
// value, documented in docs/passkeys.md: the session is still valid, so it must not sign the user out.
export const REAUTH_REQUIRED = "reauthentication required";

const _unauthorized = 401;

// isReauthRequired reports whether `error` is that problem.
export function isReauthRequired(error: unknown): boolean {
  if (typeof error !== "object" || error === null) return false;

  const problem = error as { status?: unknown; detail?: unknown };

  return problem.status === _unauthorized && problem.detail === REAUTH_REQUIRED;
}
