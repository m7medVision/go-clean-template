import { describe, expect, it } from "vite-plus/test";

import { safeRedirect } from "./redirect";

describe("safeRedirect", () => {
  it.each([
    ["/profile", "/profile"],
    ["/tasks?status=done", "/tasks?status=done"],
    ["https://evil.example", "/profile"],
    ["//evil.example", "/profile"],
    ["/\\evil.example", "/profile"],
    ["profile", "/profile"],
    [undefined, "/profile"],
  ])("%s -> %s", (target, want) => {
    expect(safeRedirect(target)).toBe(want);
  });
});
