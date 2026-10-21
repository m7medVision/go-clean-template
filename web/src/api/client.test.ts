import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { REAUTH_REQUIRED } from "#/auth/reauth";
import { auth } from "#/auth/token";
import { bearer, onUnauthorized } from "./client";

const unauthorized = vi.fn();

function respond(path: string, status: number, body: unknown) {
  const request = new Request(`http://localhost${path}`);
  const response = new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/problem+json" },
  });

  return bearer.onResponse!({ request, response } as Parameters<
    NonNullable<typeof bearer.onResponse>
  >[0]);
}

beforeEach(() => {
  unauthorized.mockReset();
  onUnauthorized(unauthorized);
  auth.set("jwt");
});

describe("bearer.onResponse", () => {
  it("signs out when the API rejects the token", async () => {
    await respond("/v1/passkeys", 401, { status: 401, detail: "invalid or expired token" });

    expect(unauthorized).toHaveBeenCalledOnce();
    expect(auth.token()).toBeNull();
  });

  it("keeps the session when the API asks to reauthenticate", async () => {
    const response = await respond("/v1/passkeys/register/begin", 401, {
      status: 401,
      detail: REAUTH_REQUIRED,
    });

    expect(unauthorized).not.toHaveBeenCalled();
    expect(auth.token()).toBe("jwt");
    // The caller still reads the problem from the body.
    await expect((response as Response).json()).resolves.toMatchObject({ detail: REAUTH_REQUIRED });
  });

  it("leaves bad credentials on /v1/auth to the form", async () => {
    await respond("/v1/auth/login", 401, { status: 401, detail: "invalid credentials" });

    expect(unauthorized).not.toHaveBeenCalled();
  });
});
