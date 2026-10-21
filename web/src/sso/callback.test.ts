import { describe, expect, it } from "vite-plus/test";

import { problemMessage } from "#/api/problem";
import { callbackProblem } from "./callback";

describe("callbackProblem", () => {
  it("is nothing while a complete callback is finishing", () => {
    expect(callbackProblem({ code: "c", state: "s" }, null)).toBeNull();
  });

  it("shows the provider's own description when it refused", () => {
    const problem = callbackProblem(
      { error: "access_denied", error_description: "User cancelled" },
      null,
    );

    expect(problemMessage(problem)).toBe("User cancelled");
  });

  it("explains a refusal the provider did not describe", () => {
    expect(problemMessage(callbackProblem({ error: "access_denied" }, null))).toBe(
      "The sign-in was cancelled or refused.",
    );
  });

  it("explains a callback without a code or state", () => {
    expect(problemMessage(callbackProblem({ code: "c" }, null))).toBe(
      "The sign-in link is incomplete. Start again from the sign-in page.",
    );
  });

  it("passes the server's rejection through", () => {
    const rejection = { type: "about:blank", status: 401, detail: "invalid sso sign-in" };

    expect(callbackProblem({ code: "c", state: "s" }, rejection)).toBe(rejection);
  });
});
