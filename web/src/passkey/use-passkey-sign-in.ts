import { KeyRoundIcon } from "lucide-react";

import type { SignInMethod } from "#/auth/sign-in-method";
import { usePasskeyLogin } from "./use-passkey-login";
import { passkeysSupported } from "./webauthn";

// usePasskeySignIn is the "Sign in with a passkey" button. The passkey autofill on the email field
// starts on mount (usePasskeyLogin) and signs in through the same `onSignedIn`.
export function usePasskeySignIn(onSignedIn: (token: string) => void): SignInMethod {
  const passkey = usePasskeyLogin(onSignedIn);

  return {
    key: "passkey",
    available: passkeysSupported(),
    icon: KeyRoundIcon,
    label: "Sign in with a passkey",
    pendingLabel: "Waiting for your device…",
    pending: passkey.pending,
    error: passkey.error,
    start: passkey.start,
    reset: passkey.reset,
  };
}
