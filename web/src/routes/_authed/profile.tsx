import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { $api } from "#/api/client";

const profileQuery = $api.queryOptions("get", "/v1/user/profile");

export const Route = createFileRoute("/_authed/profile")({
  loader: ({ context }) => context.queryClient.ensureQueryData(profileQuery),
  component: Profile,
});

function Profile() {
  const { data: user } = useSuspenseQuery(profileQuery);

  return (
    <section className="space-y-2">
      <h1 className="text-2xl font-bold">{user.username}</h1>
      <p>{user.email}</p>
      <p className="text-sm text-gray-600">
        Member since {new Date(user.created_at).toLocaleDateString()}
      </p>
    </section>
  );
}
