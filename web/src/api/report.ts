import { GraphQLRequestError } from "./graphql";

const _serverError = 500;
const _status = /\((\d{3})\)$/;

// shouldReport says whether a failure is one the team needs to hear about: a server fault, a network
// failure or a crash. A rejected token, a refused input and a missing record are the user's to fix.
export function shouldReport(error: unknown): boolean {
  if (error instanceof GraphQLRequestError) {
    if (error.code) return error.code === "INTERNAL";

    const status = Number(_status.exec(error.message)?.[1]);

    return !status || status >= _serverError;
  }
  if (error instanceof Error) return true;
  if (typeof error === "object" && error !== null) {
    const { status } = error as { status?: unknown };

    return typeof status !== "number" || status >= _serverError;
  }

  return true;
}
