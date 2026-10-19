import type { Problem } from "./client";
import { GraphQLRequestError } from "./graphql";

// problemMessage turns a Huma RFC 9457 problem, a GraphQL error, or anything else thrown into one
// line for a form.
export function problemMessage(
  error: unknown,
  fallback = "Something went wrong. Try again.",
): string {
  if (error instanceof GraphQLRequestError) return error.message;
  if (typeof error !== "object" || error === null) return fallback;

  const problem = error as Problem;
  const field = problem.errors?.find((detail) => detail.message)?.message;

  return field ?? problem.detail ?? problem.title ?? fallback;
}
