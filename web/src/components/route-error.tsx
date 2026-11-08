import { Link, type ErrorComponentProps } from "@tanstack/react-router";

import { problemMessage } from "#/api/problem";
import { Button } from "#/components/ui/button";

export function RouteError({ error, reset }: Pick<ErrorComponentProps, "error" | "reset">) {
  return (
    <div className="space-y-4">
      <h1 className="text-3xl font-semibold tracking-tight">Something went wrong</h1>
      <p role="alert" className="text-destructive">
        {problemMessage(error, "This page could not be loaded.")}
      </p>
      <Button variant="outline" onClick={reset}>
        Try again
      </Button>
    </div>
  );
}

export function RouteNotFound() {
  return (
    <div className="space-y-4">
      <h1 className="text-3xl font-semibold tracking-tight">Page not found</h1>
      <p className="text-muted-foreground">There is nothing at this address.</p>
      <Button variant="outline" asChild>
        <Link to="/tasks">Back to your tasks</Link>
      </Button>
    </div>
  );
}
