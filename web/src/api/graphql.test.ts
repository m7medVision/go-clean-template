import { afterEach, describe, expect, it, vi } from "vite-plus/test";

import { auth } from "#/auth/token";
import { TypedDocumentString } from "#/graphql/graphql";
import { onUnauthorized } from "./client";
import { GraphQLRequestError, graphqlKey, graphqlQueryOptions, request } from "./graphql";
import { problemMessage } from "./problem";

const Ping = new TypedDocumentString<{ ping: string }, { id: string }>(
  "query Ping($id: ID!) { ping(id: $id) }",
);

function respond(body: unknown, status = 200) {
  const fetchMock = vi.fn(async () => new Response(JSON.stringify(body), { status }));
  vi.stubGlobal("fetch", fetchMock);

  return fetchMock;
}

afterEach(() => {
  vi.unstubAllGlobals();
  auth.clear();
  onUnauthorized(() => {});
});

describe("request", () => {
  it("posts the operation with the bearer token and returns data", async () => {
    const fetchMock = respond({ data: { ping: "pong" } });
    auth.set("token-1");

    await expect(request(Ping, { id: "1" })).resolves.toEqual({ ping: "pong" });

    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/graphql");
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer token-1");
    expect(JSON.parse(init.body as string)).toEqual({
      query: "query Ping($id: ID!) { ping(id: $id) }",
      variables: { id: "1" },
    });
  });

  it("throws the first resolver error with its code", async () => {
    respond({ errors: [{ message: "task not found", extensions: { code: "NOT_FOUND" } }] });

    const error = await request(Ping, { id: "1" }).catch((caught: unknown) => caught);

    expect(error).toBeInstanceOf(GraphQLRequestError);
    expect((error as GraphQLRequestError).code).toBe("NOT_FOUND");
    expect(problemMessage(error)).toBe("task not found");
  });

  it("drops the session on UNAUTHENTICATED", async () => {
    respond({ errors: [{ message: "unauthorized", extensions: { code: "UNAUTHENTICATED" } }] });
    const handler = vi.fn();
    onUnauthorized(handler);
    auth.set("expired");

    await expect(request(Ping, { id: "1" })).rejects.toThrow("unauthorized");

    expect(handler).toHaveBeenCalledOnce();
    expect(auth.token()).toBeNull();
  });

  it("rejects a non-2xx response", async () => {
    respond({}, 502);

    await expect(request(Ping, { id: "1" })).rejects.toThrow("GraphQL request failed (502)");
  });
});

describe("query keys", () => {
  it("prefix every call of an operation with its name", () => {
    expect(graphqlKey(Ping)).toEqual(["graphql", "Ping"]);
    expect(graphqlQueryOptions(Ping, { id: "1" }).queryKey).toEqual([
      "graphql",
      "Ping",
      { id: "1" },
    ]);
  });
});
