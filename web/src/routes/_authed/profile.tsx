import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { profileQuery } from "#/user/api";
import { passkeysQuery } from "#/passkey/api";
import { PasskeysCard } from "#/passkey/passkeys-card";
import { Avatar, AvatarFallback } from "#/components/ui/avatar";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "#/components/ui/card";

export const Route = createFileRoute("/_authed/profile")({
  staticData: { title: "Profile" },
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(profileQuery),
      context.queryClient.ensureQueryData(passkeysQuery),
    ]),
  component: Profile,
});

function Profile() {
  const { data: user } = useSuspenseQuery(profileQuery);

  return (
    <div className="mx-auto max-w-3xl space-y-8">
      <div>
        <h1 className="text-3xl font-semibold tracking-tight">Profile</h1>
        <p className="mt-1 text-muted-foreground">Your account details.</p>
      </div>
      <Card>
        <CardHeader className="flex flex-row items-center gap-4">
          <Avatar className="size-12 rounded-lg">
            <AvatarFallback className="rounded-lg text-base">
              {user.username.slice(0, 2).toUpperCase()}
            </AvatarFallback>
          </Avatar>
          <div>
            <CardTitle className="text-lg">{user.username}</CardTitle>
            <CardDescription>{user.email}</CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <dl className="grid gap-4 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-muted-foreground">User ID</dt>
              <dd className="mt-1 font-mono">{user.id}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Member since</dt>
              <dd className="mt-1">{new Date(user.created_at).toLocaleDateString()}</dd>
            </div>
          </dl>
        </CardContent>
      </Card>
      <PasskeysCard />
    </div>
  );
}
