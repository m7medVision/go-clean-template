// What the provider appends to the redirect URI: a code and the state on success, an error code
// (and maybe a description) when the user cancels or the provider refuses.
export type CallbackSearch = {
  code?: string;
  state?: string;
  error?: string;
  error_description?: string;
};

// callbackProblem is what the callback screen reports, or null while the sign-in is finishing.
export function callbackProblem(search: CallbackSearch, finishError: unknown): unknown {
  if (search.error)
    return { detail: search.error_description ?? "The sign-in was cancelled or refused." };
  if (!search.code || !search.state)
    return { detail: "The sign-in link is incomplete. Start again from the sign-in page." };

  return finishError ?? null;
}
