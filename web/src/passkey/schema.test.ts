import { describe, expect, it } from "vite-plus/test";

import { passkeySchema, reauthSchema, renamePasskeySchema } from "./schema";

describe("passkeySchema", () => {
  it("trims the name", () => {
    expect(passkeySchema.parse({ name: "  MacBook Touch ID " })).toEqual({
      name: "MacBook Touch ID",
    });
  });

  it("leaves a blank name out", () => {
    expect(passkeySchema.parse({ name: "   " })).toEqual({ name: undefined });
  });

  it("accepts 64 characters and rejects 65", () => {
    expect(passkeySchema.safeParse({ name: "a".repeat(64) }).success).toBe(true);

    const result = passkeySchema.safeParse({ name: "a".repeat(65) });
    expect(result.success).toBe(false);
    expect(result.error?.issues[0]?.message).toContain("at most 64");
  });
});

describe("renamePasskeySchema", () => {
  it("trims the name", () => {
    expect(renamePasskeySchema.parse({ name: "  Work laptop " })).toEqual({ name: "Work laptop" });
  });

  it("rejects a blank name", () => {
    const result = renamePasskeySchema.safeParse({ name: "   " });
    expect(result.success).toBe(false);
    expect(result.error?.issues[0]?.message).toBe("Enter a name.");
  });

  it("accepts 64 characters after trimming and rejects 65", () => {
    expect(renamePasskeySchema.safeParse({ name: ` ${"a".repeat(64)} ` }).success).toBe(true);

    const result = renamePasskeySchema.safeParse({ name: "a".repeat(65) });
    expect(result.success).toBe(false);
    expect(result.error?.issues[0]?.message).toContain("at most 64");
  });
});

describe("reauthSchema", () => {
  it("requires the password", () => {
    const result = reauthSchema.safeParse({ password: "" });
    expect(result.success).toBe(false);
    expect(result.error?.issues[0]?.message).toBe("Enter your password.");
  });

  it("keeps the password as typed", () => {
    expect(reauthSchema.parse({ password: " secret " })).toEqual({ password: " secret " });
  });
});
