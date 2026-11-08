import { describe, expect, it } from "vite-plus/test";

import { attributesForm, attributesPatch, attributesSchema, taskSchema } from "./schema";

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

describe("attributesSchema", () => {
  it("accepts an empty form", () => {
    expect(attributesSchema.safeParse({ priority: "NONE", estimate: "", labels: "" }).success).toBe(
      true,
    );
  });

  it.each([
    ["an estimate under the minimum", { priority: "NONE", estimate: "0.1", labels: "" }],
    ["an estimate over the maximum", { priority: "NONE", estimate: "201", labels: "" }],
    ["an estimate that is no number", { priority: "NONE", estimate: "soon", labels: "" }],
    ["a duplicate label", { priority: "NONE", estimate: "", labels: "a, a" }],
    ["a label too long", { priority: "NONE", estimate: "", labels: "x".repeat(31) }],
    ["too many labels", { priority: "NONE", estimate: "", labels: "a,b,c,d,e,f,g,h,i,j,k" }],
  ])("rejects %s", (_, input) => {
    expect(attributesSchema.safeParse(input).success).toBe(false);
  });
});

describe("attributesPatch", () => {
  const current = { priority: "HIGH" as const, estimate: 2, labels: ["a"] };

  it("sends nothing when nothing changed", () => {
    expect(attributesPatch(current, attributesForm(current))).toEqual({});
  });

  it("sends only the changed fields, and null for a cleared one", () => {
    expect(attributesPatch(current, { priority: "HIGH", estimate: "", labels: "a, b" })).toEqual({
      estimate: null,
      labels: ["a", "b"],
    });
  });

  it("clears the priority and the labels with null", () => {
    expect(attributesPatch(current, { priority: "NONE", estimate: "2", labels: "" })).toEqual({
      priority: null,
      labels: null,
    });
  });
});
