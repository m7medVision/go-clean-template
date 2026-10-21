import { describe, expect, it } from "vite-plus/test";

import { safeRedirect } from "./redirect";

describe("safeRedirect", () => {
  it.each([
    ["/profile", "/profile"],
    ["/tasks?status=done", "/tasks?status=done"],
    ["https://evil.example", "/tasks"],
    ["//evil.example", "/tasks"],
    ["/\\evil.example", "/tasks"],
    ["profile", "/tasks"],
    ["/login", "/tasks"],
    ["/login?redirect=/profile", "/tasks"],
    ["/login/", "/tasks"],
    ["/loginx", "/loginx"],
    [undefined, "/tasks"],
  ])("%s -> %s", (target, want) => {
    expect(safeRedirect(target)).toBe(want);
  });
});
