import type { ReactNode } from "react";

import { tokenRoles } from "#/auth/roles";
import { auth } from "#/auth/token";

type AuthorizationProps = {
  allowedRoles: string[];
  fallback?: ReactNode;
  children: ReactNode;
};

export function Authorization({ allowedRoles, fallback = null, children }: AuthorizationProps) {
  const roles = tokenRoles(auth.token());

  return allowedRoles.some((role) => roles.includes(role)) ? children : fallback;
}
