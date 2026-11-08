import { describe, expect, it } from "vite-plus/test";

import { GraphQLRequestError } from "./graphql";
import { shouldReport } from "./report";

describe("shouldReport", () => {
  it.each([
    ["a 500 problem", { status: 500, title: "Internal Server Error" }],
    ["a 502 problem", { status: 502, detail: "upstream" }],
    ["a network failure", new TypeError("Failed to fetch")],
    ["a render crash", new Error("boom")],
    ["a GraphQL INTERNAL error", new GraphQLRequestError("internal error", "INTERNAL")],
    ["a GraphQL 500 response", new GraphQLRequestError("GraphQL request failed (500)")],
    ["an empty GraphQL response", new GraphQLRequestError("GraphQL response had no data")],
    ["a problem without a status", { title: "?" }],
  ])("reports %s", (_name, error) => {
    expect(shouldReport(error)).toBe(true);
  });

  it.each([
    ["a 401 problem", { status: 401 }],
    ["a 404 problem", { status: 404 }],
    ["a 422 problem", { status: 422, detail: "validation failed" }],
    ["a 429 problem", { status: 429 }],
    ["GraphQL UNAUTHENTICATED", new GraphQLRequestError("unauthenticated", "UNAUTHENTICATED")],
    ["GraphQL BAD_USER_INPUT", new GraphQLRequestError("bad input", "BAD_USER_INPUT")],
    ["a GraphQL 403 response", new GraphQLRequestError("GraphQL request failed (403)")],
  ])("does not report %s", (_name, error) => {
    expect(shouldReport(error)).toBe(false);
  });
});
