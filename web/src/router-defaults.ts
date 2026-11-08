import { RouteError, RouteNotFound } from "#/components/route-error";

export const routerDefaults = {
  defaultErrorComponent: RouteError,
  defaultNotFoundComponent: RouteNotFound,
};
