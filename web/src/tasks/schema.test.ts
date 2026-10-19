import { describe, expect, it } from "vite-plus/test";

import { taskSchema } from "./schema";

describe("taskSchema", () => {
  it("trims the title and description", () => {
    expect(taskSchema.parse({ title: "  Ship it  ", description: " soon " })).toEqual({
      title: "Ship it",
      description: "soon",
    });
  });

  it.each([
    ["a blank title", { title: "   ", description: "" }, "Give the task a title."],
    ["a long title", { title: "x".repeat(256), description: "" }, "under 255"],
    ["a long description", { title: "ok", description: "x".repeat(1001) }, "under 1000"],
  ])("rejects %s", (_, input, message) => {
    const result = taskSchema.safeParse(input);

    expect(result.success).toBe(false);
    expect(result.error?.issues[0]?.message).toContain(message);
  });
});
