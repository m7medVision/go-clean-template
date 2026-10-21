import { z } from "zod";

import { loginSchema } from "#/auth/schema";

// Mirrors the server's passkey name rules: at most 64 characters (the name query parameter on
// POST /v1/passkeys/register/finish, and the body of PATCH /v1/passkeys/{id}, which also needs
// one). The server stays the authority.
const _maxName = 64;

const passkeyName = z.string().trim().max(_maxName, `Use at most ${_maxName} characters.`);

// passkeySchema validates the name asked for before adding a passkey. A blank name is left out,
// so the server picks its default.
export const passkeySchema = z.object({
  name: passkeyName.transform((name) => name || undefined),
});

// renamePasskeySchema validates a passkey's new name, which cannot be blank.
export const renamePasskeySchema = z.object({
  name: passkeyName.min(1, "Enter a name."),
});

// reauthSchema validates the password asked for when adding a passkey needs a fresh sign-in. It is
// the login form's password rule; the email is the signed-in user's.
export const reauthSchema = loginSchema.pick({ password: true });
