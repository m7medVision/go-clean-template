import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it } from "vite-plus/test";

import { auth } from "#/auth/token";
import { Authorization } from "./authorization";

const jwt = (payload: unknown) =>
  ["e30", btoa(JSON.stringify(payload)).replaceAll("=", ""), "sig"].join(".");

const view = (allowedRoles: string[]) =>
  renderToStaticMarkup(
    <Authorization allowedRoles={allowedRoles} fallback={<i>denied</i>}>
      <b>secret</b>
    </Authorization>,
  );

describe("Authorization", () => {
  afterEach(() => auth.clear());

  it("renders its children for a matching role", () => {
    auth.set(jwt({ roles: ["user", "admin"] }));

    expect(view(["admin"])).toBe("<b>secret</b>");
  });

  it("renders its children when any of several roles matches", () => {
    auth.set(jwt({ roles: ["user"] }));

    expect(view(["admin", "user"])).toBe("<b>secret</b>");
  });

  it.each([
    ["no token", null],
    ["no roles claim", jwt({})],
    ["a different role", jwt({ roles: ["user"] })],
    ["a malformed token", "not-a-token"],
  ])("renders the fallback for %s", (_name, token) => {
    if (token) auth.set(token);

    expect(view(["admin"])).toBe("<i>denied</i>");
  });
});
