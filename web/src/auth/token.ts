// The JWT lives in memory, mirrored to sessionStorage so it survives a reload but not the tab.
// It is readable by scripts, so the Caddy CSP is what keeps it safe; see README "Web frontend".

const KEY = "gct.token";

type TokenStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export type TokenStore = {
  token: () => string | null;
  set: (token: string) => void;
  clear: () => void;
};

export function createTokenStore(storage: TokenStorage | undefined): TokenStore {
  let current: string | null = null;
  let loaded = false;

  const attempt = <T>(fn: () => T, fallback: T): T => {
    try {
      return fn();
    } catch {
      return fallback;
    }
  };

  return {
    token() {
      if (!loaded) {
        current = storage ? attempt(() => storage.getItem(KEY), null) : null;
        loaded = true;
      }

      return current;
    },
    set(token) {
      current = token;
      loaded = true;
      if (storage) attempt(() => storage.setItem(KEY, token), undefined);
    },
    clear() {
      current = null;
      loaded = true;
      if (storage) attempt(() => storage.removeItem(KEY), undefined);
    },
  };
}

function sessionStore(): TokenStorage | undefined {
  try {
    return globalThis.sessionStorage;
  } catch {
    return undefined;
  }
}

export const auth = createTokenStore(sessionStore());
