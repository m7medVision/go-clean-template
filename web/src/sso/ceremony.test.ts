import { describe, expect, it } from "vite-plus/test";

import { createCeremonyStore } from "./ceremony";

function memoryStorage() {
  const items = new Map<string, string>();

  return {
    getItem: (key: string) => items.get(key) ?? null,
    setItem: (key: string, value: string) => void items.set(key, value),
    removeItem: (key: string) => void items.delete(key),
  };
}

describe("createCeremonyStore", () => {
  it("gives back what was saved, once", () => {
    const store = createCeremonyStore(memoryStorage());

    store.save({ token: "ct-1", redirect: "/tasks?page=2" });

    expect(store.take()).toEqual({ token: "ct-1", redirect: "/tasks?page=2" });
    expect(store.take()).toBeNull();
  });

  it("survives a reload: a new store over the same storage reads it", () => {
    const storage = memoryStorage();
    createCeremonyStore(storage).save({ token: "ct-1", redirect: "/tasks" });

    expect(createCeremonyStore(storage).take()).toEqual({ token: "ct-1", redirect: "/tasks" });
  });

  it("is empty when nothing was saved or the entry is not ours", () => {
    const storage = memoryStorage();
    expect(createCeremonyStore(storage).take()).toBeNull();

    storage.setItem("gct.sso", "not json");
    expect(createCeremonyStore(storage).take()).toBeNull();

    storage.setItem("gct.sso", JSON.stringify({ token: 1 }));
    expect(createCeremonyStore(storage).take()).toBeNull();
  });

  it("does nothing without storage, or when storage throws", () => {
    expect(createCeremonyStore(undefined).take()).toBeNull();

    const throwing = {
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("blocked");
      },
      removeItem: () => {
        throw new Error("blocked");
      },
    };
    const store = createCeremonyStore(throwing);

    expect(() => store.save({ token: "ct", redirect: "/" })).not.toThrow();
    expect(store.take()).toBeNull();
  });
});
