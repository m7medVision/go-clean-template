import { useMutation } from "@tanstack/react-query";
import { useCallback, useEffect, useRef } from "react";

import { passkeyLogin } from "./api";
import { conditionalMediationAvailable } from "./webauthn";

// usePasskeyLogin signs in with a passkey two ways: the passkey offered in the autofill of the
// input marked autocomplete="username webauthn" (conditional mediation, started on mount), and
// `start` for a button. One ceremony at a time: `start` aborts the autofill request, which comes
// back once the button flow ends without signing in. Unmounting aborts it too. A cancelled or
// aborted ceremony is silent; `error` is the server's rejection, for FormError.
export function usePasskeyLogin(onSignedIn: (token: string) => void) {
  const autofillAbort = useRef<AbortController | null>(null);
  const mounted = useRef(false);

  const autofill = useMutation({
    ...passkeyLogin,
    onSuccess: (result) => {
      if (result) onSignedIn(result.token);
    },
  });
  const { mutate: runAutofill } = autofill;

  const startAutofill = useCallback(async () => {
    if (!mounted.current) return;

    autofillAbort.current?.abort();
    const controller = new AbortController();
    autofillAbort.current = controller;

    if (!(await conditionalMediationAvailable()) || controller.signal.aborted) return;

    runAutofill({ mediation: "conditional", signal: controller.signal });
  }, [runAutofill]);

  const stopAutofill = useCallback(() => {
    autofillAbort.current?.abort();
    autofillAbort.current = null;
  }, []);

  const button = useMutation({
    ...passkeyLogin,
    onSuccess: (result) => {
      if (result) onSignedIn(result.token);
      else void startAutofill();
    },
    onError: () => void startAutofill(),
  });

  useEffect(() => {
    mounted.current = true;
    void startAutofill();

    return () => {
      mounted.current = false;
      stopAutofill();
    };
  }, [startAutofill, stopAutofill]);

  const { mutate: runButton, reset: resetButton } = button;
  const { reset: resetAutofill } = autofill;

  const start = useCallback(() => {
    stopAutofill();
    resetAutofill();
    runButton({});
  }, [stopAutofill, resetAutofill, runButton]);

  return {
    start,
    pending: button.isPending,
    error: button.error ?? autofill.error,
    reset: useCallback(() => {
      resetButton();
      resetAutofill();
    }, [resetButton, resetAutofill]),
  };
}
