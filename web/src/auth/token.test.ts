import { describe, expect, it } from "vite-plus/test";

import { createTokenStore } from "./token";

function memoryStorage(seed: Record<string, string> = {}) {
  const items = new Map(Object.entries(seed));

  return {
    getItem: (key: string) => items.get(key) ?? null,
    setItem: (key: string, value: string) => void items.set(key, value),
    removeItem: (key: string) => void items.delete(key),
    items,
  };
}

describe("createTokenStore", () => {
  it("restores a token saved by an earlier page load", () => {
    const store = createTokenStore(memoryStorage({ "gct.token": "saved" }));

    expect(store.token()).toBe("saved");
  });

  it("mirrors set and clear into storage", () => {
    const storage = memoryStorage();
    const store = createTokenStore(storage);

    store.set("jwt");
    expect(store.token()).toBe("jwt");
    expect(storage.items.get("gct.token")).toBe("jwt");

    store.clear();
    expect(store.token()).toBeNull();
    expect(storage.items.has("gct.token")).toBe(false);
  });

  it("keeps working in memory when storage throws", () => {
    const store = createTokenStore({
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("blocked");
      },
      removeItem: () => {
        throw new Error("blocked");
      },
    });

    expect(store.token()).toBeNull();
    store.set("jwt");
    expect(store.token()).toBe("jwt");
  });

  it("works without any storage", () => {
    const store = createTokenStore(undefined);

    store.set("jwt");
    expect(store.token()).toBe("jwt");
  });
});
