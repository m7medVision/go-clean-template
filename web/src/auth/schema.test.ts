import { describe, expect, it } from "vite-plus/test";

import { loginSchema, registerSchema } from "./schema";

describe("auth schemas", () => {
  it("requires a valid email and a password to log in", () => {
    const result = loginSchema.safeParse({ email: "nope", password: "" });

    expect(result.error?.issues.map((issue) => issue.path[0])).toEqual(["email", "password"]);
  });

  it("enforces the register lengths", () => {
    const result = registerSchema.safeParse({ username: "ab", email: "a@b.co", password: "12345" });

    expect(result.error?.issues.map((issue) => issue.path[0])).toEqual(["username", "password"]);
  });
});
