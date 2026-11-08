import { describe, expect, it } from "vite-plus/test";

import { tokenRoles, tokenSubject } from "./roles";

// A JWT with the given payload; the signature is irrelevant here, the server checks it.
const jwt = (payload: unknown) =>
  [
    "e30",
    btoa(JSON.stringify(payload)).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, ""),
    "sig",
  ].join(".");

describe("tokenRoles", () => {
  it("reads the roles claim", () => {
    expect(tokenRoles(jwt({ sub: "u1", roles: ["admin", "user"] }))).toEqual(["admin", "user"]);
  });

  it("decodes base64url payloads", () => {
    // "~~~" encodes to "fn5+" in base64, "fn5-" in base64url.
    expect(tokenRoles(jwt({ roles: ["user"], note: "~~~?>" }))).toEqual(["user"]);
  });

  it("is empty for no token, a malformed token, or no roles claim", () => {
    expect(tokenRoles(null)).toEqual([]);
    expect(tokenRoles("not-a-jwt")).toEqual([]);
    expect(tokenRoles("a.%%%.c")).toEqual([]);
    expect(tokenRoles(jwt({ sub: "u1" }))).toEqual([]);
    expect(tokenRoles(jwt({ roles: "admin" }))).toEqual([]);
    expect(tokenRoles(jwt({ roles: ["admin", 1] }))).toEqual(["admin"]);
  });
});

describe("tokenSubject", () => {
  it("reads the sub claim", () => {
    expect(tokenSubject(jwt({ sub: "u1", roles: ["admin"] }))).toBe("u1");
  });

  it.each([
    ["no token", null],
    ["no sub claim", jwt({ roles: [] })],
    ["a non-string sub", jwt({ sub: 7 })],
    ["a malformed token", "not-a-token"],
  ])("is undefined for %s", (_name, token) => {
    expect(tokenSubject(token)).toBeUndefined();
  });
});
