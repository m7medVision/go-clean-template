import { describe, expect, it } from "vite-plus/test";

import { problemMessage } from "./problem";

describe("problemMessage", () => {
  it("prefers the first field error", () => {
    const problem = {
      detail: "validation failed",
      errors: [{ location: "body.email", message: "expected email" }],
    };

    expect(problemMessage(problem)).toBe("expected email");
  });

  it("falls back to detail, then title", () => {
    expect(problemMessage({ detail: "invalid credentials", title: "Unauthorized" })).toBe(
      "invalid credentials",
    );
    expect(problemMessage({ title: "Conflict" })).toBe("Conflict");
  });

  it("uses the fallback for anything that is not a problem", () => {
    expect(problemMessage(new TypeError("Failed to fetch"), "offline")).toBe("offline");
    expect(problemMessage(null, "offline")).toBe("offline");
  });
});
