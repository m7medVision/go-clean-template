import { describe, expect, it } from "vite-plus/test";

import { isReauthRequired, REAUTH_REQUIRED } from "./reauth";

describe("isReauthRequired", () => {
  it("recognises the server's reauthentication problem", () => {
    expect(isReauthRequired({ status: 401, title: "Unauthorized", detail: REAUTH_REQUIRED })).toBe(
      true,
    );
  });

  it.each([
    ["another 401", { status: 401, detail: "invalid or expired token" }],
    ["the same detail on another status", { status: 403, detail: REAUTH_REQUIRED }],
    ["null", null],
    ["a thrown Error", new Error(REAUTH_REQUIRED)],
  ])("rejects %s", (_, error) => {
    expect(isReauthRequired(error)).toBe(false);
  });
});
