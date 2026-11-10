import { $api } from "#/api/client";

// The caller's feature flags, as the backend decides them; the client never evaluates one.
export const flagsQuery = $api.queryOptions("get", "/v1/flags");
