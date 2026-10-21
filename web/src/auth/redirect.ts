// safeRedirect keeps a post-login redirect on this origin. Anything else falls back to `fallback`,
// so `/login?redirect=https://evil.example` cannot bounce a fresh session off-site.
export function safeRedirect(target: unknown, fallback = "/tasks"): string {
  if (typeof target !== "string") return fallback;
  if (!target.startsWith("/") || target.startsWith("//") || target.startsWith("/\\"))
    return fallback;
  // Back to /login would leave a signed-in user on the login form, nesting redirects.
  if (/^\/login(?:[/?#]|$)/.test(target)) return fallback;

  return target;
}
