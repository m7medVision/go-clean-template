// The roles the app JWT was issued with, read without verifying it. This only decides what the UI
// offers (the admin nav link); the server checks the signature and the role on every request.

function payload(token: string): unknown {
  const part = token.split(".")[1];
  if (!part) return null;

  try {
    const base64 = part.replaceAll("-", "+").replaceAll("_", "/");
    const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));

    return JSON.parse(new TextDecoder().decode(bytes));
  } catch {
    return null;
  }
}

export function tokenRoles(token: string | null): string[] {
  if (!token) return [];

  const claims = payload(token);
  if (typeof claims !== "object" || claims === null) return [];

  const { roles } = claims as { roles?: unknown };

  return Array.isArray(roles) ? roles.filter((r): r is string => typeof r === "string") : [];
}

export function tokenSubject(token: string | null): string | undefined {
  if (!token) return undefined;

  const claims = payload(token);
  if (typeof claims !== "object" || claims === null) return undefined;

  const { sub } = claims as { sub?: unknown };

  return typeof sub === "string" ? sub : undefined;
}
