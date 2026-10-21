import { mutationOptions } from "@tanstack/react-query";

import { $api, api } from "#/api/client";
import type { components } from "#/api/schema.gen";
import { auth } from "#/auth/token";
import { createCredential, type GetCredentialOptions, getCredential } from "./webauthn";

export type Passkey = components["schemas"]["Passkey"];

// The passkey list on the profile screen. Adding, renaming or deleting a passkey invalidates it.
export const passkeysQuery = $api.queryOptions("get", "/v1/passkeys");

export const passkeysKey = passkeysQuery.queryKey;

// Reauth is the password sign-in addPasskey runs first when the server asks for a recent one.
export type Reauth = { email: string; password: string };

// addPasskey runs the whole registration ceremony: begin on the server, create the credential in
// the browser, finish on the server. It resolves to null when the user cancels the prompt.
//
// Adding a passkey needs a recent sign-in. When begin answers the reauthentication problem
// (isReauthRequired), ask for the password and call again with `reauth`: it signs in through the
// password login first and stores the fresh token, as the login screen does.
export const addPasskey = mutationOptions({
  mutationKey: ["passkeys", "add"],
  mutationFn: async ({ name, reauth }: { name?: string; reauth?: Reauth }) => {
    if (reauth) {
      const login = await api.POST("/v1/auth/login", { body: reauth });
      if (login.error) throw login.error;

      auth.set(login.data.token);
    }

    const begin = await api.POST("/v1/passkeys/register/begin");
    if (begin.error) throw begin.error;

    const credential = await createCredential(begin.data.options);
    if (!credential) return null;

    const finish = await api.POST("/v1/passkeys/register/finish", {
      params: { header: { "X-Ceremony-Token": begin.data.ceremony_token }, query: { name } },
      body: credential,
    });
    if (finish.error) throw finish.error;

    return finish.data;
  },
});

// renamePasskey names one of the user's passkeys and resolves to the updated passkey.
export const renamePasskey = mutationOptions({
  mutationKey: ["passkeys", "rename"],
  mutationFn: async ({ id, name }: { id: string; name: string }) => {
    const { data, error } = await api.PATCH("/v1/passkeys/{id}", {
      params: { path: { id } },
      body: { name },
    });
    if (error) throw error;

    return data;
  },
});

// deletePasskey removes one of the user's passkeys; it can no longer sign in.
export const deletePasskey = mutationOptions({
  mutationKey: ["passkeys", "delete"],
  mutationFn: async ({ id }: { id: string }) => {
    const { error } = await api.DELETE("/v1/passkeys/{id}", { params: { path: { id } } });
    if (error) throw error;
  },
});

// passkeyLogin runs the whole sign-in ceremony: begin on the server, get the assertion in the
// browser, finish on the server for the same token password login returns. It resolves to null
// when the user cancels or `signal` aborts it; only server rejections reach the caller as errors.
export const passkeyLogin = mutationOptions({
  mutationKey: ["passkeys", "login"],
  mutationFn: async ({ mediation, signal }: GetCredentialOptions) => {
    try {
      const begin = await api.POST("/v1/auth/passkey/begin", { signal });
      if (begin.error) throw begin.error;

      const credential = await getCredential(begin.data.options, { mediation, signal });
      if (!credential) return null;

      const finish = await api.POST("/v1/auth/passkey/finish", {
        params: { header: { "X-Ceremony-Token": begin.data.ceremony_token } },
        body: credential,
      });
      if (finish.error) throw finish.error;

      return finish.data;
    } catch (error) {
      // An aborted begin rejects with the signal's AbortError; the abort was ours, so stay silent.
      if (signal?.aborted) return null;

      throw error;
    }
  },
});
