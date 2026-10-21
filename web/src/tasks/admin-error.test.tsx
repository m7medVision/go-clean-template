import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";

import { AllTasksError } from "./admin-error";

describe("AllTasksError", () => {
  // Regression: a non-admin opening /admin/tasks got the router's default "Something went wrong!" box.
  it("shows the API's refusal in the page, as an alert", () => {
    const refusal = { type: "about:blank", status: 403, detail: "admin role required" };
    const html = renderToStaticMarkup(<AllTasksError error={refusal as unknown as Error} />);

    expect(html).toContain("admin role required");
    expect(html).toContain('role="alert"');
  });

  it("falls back to a plain message for an error without a problem body", () => {
    expect(renderToStaticMarkup(<AllTasksError error={null as unknown as Error} />)).toContain(
      "The task list could not be loaded.",
    );
  });
});
