import type { Problem } from "./client";

// problemMessage turns a Huma RFC 9457 problem (or anything else thrown) into one line for a form.
export function problemMessage(
  error: unknown,
  fallback = "Something went wrong. Try again.",
): string {
  if (typeof error !== "object" || error === null) return fallback;

  const problem = error as Problem;
  const field = problem.errors?.find((detail) => detail.message)?.message;

  return field ?? problem.detail ?? problem.title ?? fallback;
}
