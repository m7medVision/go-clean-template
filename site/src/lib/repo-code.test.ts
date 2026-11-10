import { describe, expect, it } from "vite-plus/test";
import { excerpt } from "./repo-code";

describe("excerpt", () => {
  it("returns a Go function with its doc comment", () => {
    const { code, lang } = excerpt("internal/entity/task.go", { symbol: "ValidateTaskText" });
    expect(lang).toBe("go");
    expect(code).toMatch(/func ValidateTaskText/);
    expect(code.trimEnd().endsWith("}")).toBe(true);
  });

  it("returns a line range", () => {
    const { code, startLine } = excerpt("go.mod", { lines: "1-1" });
    expect(code).toMatch(/^module /);
    expect(startLine).toBe(1);
  });

  it.each<[{ path: string; symbol?: string; lines?: string }, RegExp]>([
    [{ path: "nope.go" }, /does not exist/],
    [{ path: "go.mod", symbol: "Missing" }, /not found/],
    [{ path: "go.mod", lines: "9-1" }, /out of range/],
    [{ path: "../etc/passwd" }, /outside/],
  ])("fails on %o", (c, err) => {
    expect(() => excerpt(c.path, c)).toThrow(err);
  });
});
