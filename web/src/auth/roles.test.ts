import { describe, expect, it } from "vite-plus/test";

import { isAdmin, tokenRoles } from "./roles";

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

describe("isAdmin", () => {
  it("is true only with the admin role", () => {
    expect(isAdmin(jwt({ roles: ["user", "admin"] }))).toBe(true);
    expect(isAdmin(jwt({ roles: ["user"] }))).toBe(false);
    expect(isAdmin(null)).toBe(false);
  });
});
