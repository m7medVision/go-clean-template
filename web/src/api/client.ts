import createFetchClient, { type Middleware } from "openapi-fetch";
import createQueryClient from "openapi-react-query";

import { isReauthRequired } from "#/auth/reauth";
import { auth } from "#/auth/token";
import type { components, paths } from "./schema.gen";

export type Problem = components["schemas"]["ErrorModel"];

// Same origin in every environment: the Vite proxy in dev, Caddy in compose.
export const api = createFetchClient<paths>({ baseUrl: "/" });

const _authPrefix = "/v1/auth/";

let unauthorized: () => void = () => {};

// onUnauthorized sets what happens when the API rejects the token (expired or revoked).
export function onUnauthorized(handler: () => void) {
  unauthorized = handler;
}

// rejectSession drops the token and runs the onUnauthorized handler. Both transports call it.
export function rejectSession() {
  auth.clear();
  unauthorized();
}

// isReauthResponse reads a 401's problem without consuming the body the caller will read.
async function isReauthResponse(response: Response): Promise<boolean> {
  try {
    return isReauthRequired(await response.clone().json());
  } catch {
    return false;
  }
}

export const bearer: Middleware = {
  onRequest({ request }) {
    const token = auth.token();
    if (token) request.headers.set("Authorization", `Bearer ${token}`);

    return request;
  },
  async onResponse({ request, response }) {
    // middleware.Auth answers 401 for a missing or invalid token; /v1/auth/* answers 401 for bad
    // credentials. A 401 asking to reauthenticate comes with a valid token, so the session stays.
    if (
      response.status === 401 &&
      !new URL(request.url).pathname.startsWith(_authPrefix) &&
      !(await isReauthResponse(response))
    ) {
      rejectSession();
    }

    return response;
  },
};

api.use(bearer);

export const $api = createQueryClient(api);
