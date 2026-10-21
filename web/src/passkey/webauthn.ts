// The browser half of a WebAuthn ceremony, on the native JSON helpers: no client library.

// The JSON body the finish endpoints take: PublicKeyCredential.toJSON(), sent as it is.
export type CredentialJSON = Record<string, unknown>;

// passkeysSupported reports whether this browser can run a ceremony from the server's JSON
// options. Browsers without the JSON parsers get no passkey buttons.
export function passkeysSupported(): boolean {
  const credential = globalThis.PublicKeyCredential;
  const credentials = globalThis.navigator?.credentials;

  return (
    typeof credential?.parseCreationOptionsFromJSON === "function" &&
    typeof credential.parseRequestOptionsFromJSON === "function" &&
    typeof credentials?.create === "function" &&
    typeof credentials.get === "function"
  );
}

// conditionalMediationAvailable reports whether the browser can offer passkeys in the autofill
// of an input marked autocomplete="... webauthn". Any failure of the check means no.
export async function conditionalMediationAvailable(): Promise<boolean> {
  if (!passkeysSupported()) return false;

  if (typeof PublicKeyCredential.isConditionalMediationAvailable !== "function") return false;

  try {
    return await PublicKeyCredential.isConditionalMediationAvailable();
  } catch {
    return false;
  }
}

// isCancelled is true for the error the browser raises when the user dismisses the prompt (or it
// times out), and for an aborted request. The spec gives dismissal and time-out the same
// NotAllowedError, and an abort is this app's own doing, so none is shown as a failure.
function isCancelled(error: unknown): boolean {
  return (
    error instanceof DOMException &&
    (error.name === "NotAllowedError" || error.name === "AbortError")
  );
}

// createCredential runs navigator.credentials.create with the begin endpoint's options and returns
// the new credential's JSON for the finish endpoint, or null when the user cancels.
export async function createCredential(options: object): Promise<CredentialJSON | null> {
  // The server's options are typed as an open object; their shape is the WebAuthn spec's, and the
  // browser validates them in parseCreationOptionsFromJSON.
  const publicKey = PublicKeyCredential.parseCreationOptionsFromJSON(
    options as PublicKeyCredentialCreationOptionsJSON,
  );

  let credential: Credential | null;
  try {
    credential = await navigator.credentials.create({ publicKey });
  } catch (error) {
    if (isCancelled(error)) return null;

    throw error;
  }

  if (!(credential && "toJSON" in credential)) {
    throw new Error("The browser did not create a passkey.");
  }

  return (credential as PublicKeyCredential).toJSON() as unknown as CredentialJSON;
}

export type GetCredentialOptions = {
  // "conditional" offers the passkeys in the autofill of the webauthn input instead of a prompt.
  mediation?: CredentialMediationRequirement;
  // Aborting it ends the request; getCredential then resolves to null.
  signal?: AbortSignal;
};

// getCredential runs navigator.credentials.get with the begin endpoint's options and returns the
// assertion's JSON for the finish endpoint, or null when the user cancels or the request is
// aborted.
export async function getCredential(
  options: object,
  { mediation, signal }: GetCredentialOptions = {},
): Promise<CredentialJSON | null> {
  const publicKey = PublicKeyCredential.parseRequestOptionsFromJSON(
    options as PublicKeyCredentialRequestOptionsJSON,
  );

  let credential: Credential | null;
  try {
    credential = await navigator.credentials.get({
      publicKey,
      ...(mediation && { mediation }),
      ...(signal && { signal }),
    });
  } catch (error) {
    if (isCancelled(error) || signal?.aborted) return null;

    throw error;
  }

  if (!(credential && "toJSON" in credential)) {
    throw new Error("The browser did not return a passkey.");
  }

  return (credential as PublicKeyCredential).toJSON() as unknown as CredentialJSON;
}
