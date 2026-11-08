import { auth } from "#/auth/token";
import { onRest } from "./handlers";

const jwt = (payload: unknown) =>
  ["e30", btoa(JSON.stringify(payload)).replaceAll("=", ""), "sig"].join(".");

// signInAs puts a token with the given roles in the store and answers the profile the sidebar reads.
export function signInAs(roles: string[] = ["user"]) {
  auth.set(jwt({ roles }));
  onRest("get", "/v1/user/profile", {
    id: "550e8400-e29b-41d4-a716-446655440000",
    email: "jane@example.com",
    username: "jane",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  });
}
