import { queryOptions } from "@tanstack/react-query";

import { auth } from "#/auth/token";
import type { TypedDocumentString } from "#/graphql/graphql";
import { rejectSession } from "./client";

// Same origin as the REST API: the Vite proxy in dev, Caddy in compose.
const _endpoint = "/graphql";
const _unauthenticated = "UNAUTHENTICATED";

type GraphQLErrorBody = { message: string; extensions?: { code?: string } };
type GraphQLResponse<TResult> = { data?: TResult | null; errors?: GraphQLErrorBody[] };

// GraphQLRequestError carries the first error the resolver returned. Its message is already
// safe to show (resolver.mapError never leaks internals), so problemMessage passes it through.
export class GraphQLRequestError extends Error {
  readonly code: string | undefined;

  constructor(message: string, code?: string) {
    super(message);
    this.name = "GraphQLRequestError";
    this.code = code;
  }
}

// An operation with no variables takes no second argument.
type VariablesArg<TVariables> =
  TVariables extends Record<string, never> ? [variables?: TVariables] : [variables: TVariables];

export async function request<TResult, TVariables>(
  document: TypedDocumentString<TResult, TVariables>,
  ...[variables]: VariablesArg<TVariables>
): Promise<TResult> {
  const headers = new Headers({ "Content-Type": "application/json", Accept: "application/json" });
  const token = auth.token();
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const response = await fetch(_endpoint, {
    method: "POST",
    headers,
    body: JSON.stringify({ query: document.toString(), variables }),
  });
  if (!response.ok) throw new GraphQLRequestError(`GraphQL request failed (${response.status})`);

  const body = (await response.json()) as GraphQLResponse<TResult>;
  const [first] = body.errors ?? [];
  if (first) {
    const code = first.extensions?.code;
    // gqlgen answers a missing or expired token with 200 and this code, not with a 401.
    if (code === _unauthenticated) rejectSession();
    throw new GraphQLRequestError(first.message, code);
  }
  if (!body.data) throw new GraphQLRequestError("GraphQL response had no data");

  return body.data;
}

const operationPattern = /\b(?:query|mutation)\s+(\w+)/;

function operationName<TResult, TVariables>(
  document: TypedDocumentString<TResult, TVariables>,
): string {
  const name = operationPattern.exec(document.toString())?.[1];
  if (!name) throw new Error("GraphQL operations must be named");

  return name;
}

// graphqlKey is the query-key prefix shared by every call of one operation: invalidate it to
// refetch that operation for all variables.
export function graphqlKey<TResult, TVariables>(
  document: TypedDocumentString<TResult, TVariables>,
) {
  return ["graphql", operationName(document)] as const;
}

export function graphqlQueryOptions<TResult, TVariables>(
  document: TypedDocumentString<TResult, TVariables>,
  ...[variables]: VariablesArg<TVariables>
) {
  return queryOptions({
    queryKey: [...graphqlKey(document), variables ?? {}] as const,
    queryFn: () => request(document, ...([variables] as VariablesArg<TVariables>)),
  });
}

export function graphqlMutationOptions<TResult, TVariables>(
  document: TypedDocumentString<TResult, TVariables>,
) {
  return {
    mutationKey: graphqlKey(document),
    mutationFn: (variables: TVariables) =>
      request(document, ...([variables] as VariablesArg<TVariables>)),
  };
}
