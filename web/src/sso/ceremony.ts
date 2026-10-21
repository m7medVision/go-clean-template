// The SSO sign-in leaves the app for the provider and comes back on a full page load, so the
// ceremony token from begin (and where to go after) waits in sessionStorage: per tab, gone with it.
// It is taken once, by the callback route.

const KEY = "gct.sso";

type CeremonyStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export type SsoCeremony = { token: string; redirect: string };

export type CeremonyStore = {
  save: (ceremony: SsoCeremony) => void;
  take: () => SsoCeremony | null;
};

function isCeremony(value: unknown): value is SsoCeremony {
  if (typeof value !== "object" || value === null) return false;
  const { token, redirect } = value as Record<string, unknown>;

  return typeof token === "string" && typeof redirect === "string";
}

export function createCeremonyStore(storage: CeremonyStorage | undefined): CeremonyStore {
  return {
    save(ceremony) {
      try {
        storage?.setItem(KEY, JSON.stringify(ceremony));
      } catch {
        // Storage is blocked: the callback reports the sign-in as expired.
      }
    },
    take() {
      try {
        const raw = storage?.getItem(KEY) ?? null;
        storage?.removeItem(KEY);
        if (raw === null) return null;

        const value: unknown = JSON.parse(raw);

        return isCeremony(value) ? value : null;
      } catch {
        return null;
      }
    },
  };
}

function sessionStore(): CeremonyStorage | undefined {
  try {
    return globalThis.sessionStorage;
  } catch {
    return undefined;
  }
}

export const ssoCeremony = createCeremonyStore(sessionStore());
