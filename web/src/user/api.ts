import { $api } from "#/api/client";

// Shared by the profile screen and the sidebar's user menu, so both read one cache entry.
export const profileQuery = $api.queryOptions("get", "/v1/user/profile");
