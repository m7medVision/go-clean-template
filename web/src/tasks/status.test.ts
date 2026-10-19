import { describe, expect, it } from "vite-plus/test";

import { isTaskStatus, nextStatuses, shortID } from "./status";

describe("nextStatuses", () => {
  it.each([
    ["TODO", ["IN_PROGRESS"]],
    ["IN_PROGRESS", ["DONE", "TODO"]],
    ["DONE", []],
  ] as const)("%s -> %j", (status, want) => {
    expect(nextStatuses(status)).toEqual(want);
  });
});

describe("isTaskStatus", () => {
  it.each([
    ["TODO", true],
    ["IN_PROGRESS", true],
    ["DONE", true],
    ["todo", false],
    ["ARCHIVED", false],
    [undefined, false],
    [1, false],
  ])("%s -> %s", (value, want) => {
    expect(isTaskStatus(value)).toBe(want);
  });
});

describe("shortID", () => {
  it("upper-cases the first UUID segment", () => {
    expect(shortID("550e8400-e29b-41d4-a716-446655440000")).toBe("TASK-550E8400");
  });
});
