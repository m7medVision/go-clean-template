import type { ErrorComponentProps } from "@tanstack/react-router";

import { problemMessage } from "#/api/problem";

// The "All tasks" route's error view. A signed-in user without the admin role who opens the URL
// directly gets the API's refusal here, in the page, instead of the router's default error box.
export function AllTasksError({ error }: Pick<ErrorComponentProps, "error">) {
  return (
    <div className="space-y-2">
      <h1 className="text-3xl font-semibold tracking-tight">All tasks</h1>
      <p role="alert" className="text-destructive">
        {problemMessage(error, "The task list could not be loaded.")}
      </p>
    </div>
  );
}
