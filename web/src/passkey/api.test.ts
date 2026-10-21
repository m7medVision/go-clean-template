import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { api } from "#/api/client";
import { REAUTH_REQUIRED } from "#/auth/reauth";
import { auth } from "#/auth/token";
import { addPasskey, deletePasskey, passkeyLogin, renamePasskey } from "./api";
import { createCredential, getCredential } from "./webauthn";

vi.mock("#/api/client", () => ({
  api: { POST: vi.fn(), PATCH: vi.fn(), DELETE: vi.fn() },
  $api: { queryOptions: () => ({ queryKey: ["get", "/v1/passkeys"] }) },
}));
vi.mock("./webauthn", () => ({ getCredential: vi.fn(), createCredential: vi.fn() }));
vi.mock("#/auth/token", () => ({ auth: { set: vi.fn() } }));

const post = vi.mocked(api.POST);
const patch = vi.mocked(api.PATCH);
const del = vi.mocked(api.DELETE);
const get = vi.mocked(getCredential);
const create = vi.mocked(createCredential);
const setToken = vi.mocked(auth.set);

const options = { challenge: "Y2hhbGxlbmdl", rpId: "localhost" };
const assertion = { id: "AQIDBA", type: "public-key", response: {} };
const problem = { status: 401, title: "Unauthorized", detail: "invalid passkey" };

function respond(...results: unknown[]) {
  for (const result of results) post.mockResolvedValueOnce(result as never);
}

const login = (variables = {}) =>
  passkeyLogin.mutationFn!(
    variables,
    {} as Parameters<NonNullable<typeof passkeyLogin.mutationFn>>[1],
  );

beforeEach(() => {
  post.mockReset();
  patch.mockReset();
  del.mockReset();
  get.mockReset();
  create.mockReset();
  setToken.mockReset();
});

describe("passkeyLogin", () => {
  it("begins, gets the assertion and finishes with the ceremony token", async () => {
    respond({ data: { options, ceremony_token: "ct-1" } }, { data: { token: "jwt-1" } });
    get.mockResolvedValue(assertion);
    const { signal } = new AbortController();

    await expect(login({ mediation: "conditional", signal })).resolves.toEqual({ token: "jwt-1" });

    expect(post).toHaveBeenNthCalledWith(1, "/v1/auth/passkey/begin", { signal });
    expect(get).toHaveBeenCalledWith(options, { mediation: "conditional", signal });
    expect(post).toHaveBeenNthCalledWith(2, "/v1/auth/passkey/finish", {
      params: { header: { "X-Ceremony-Token": "ct-1" } },
      body: assertion,
    });
  });

  it("resolves to null without finishing when the user cancels", async () => {
    respond({ data: { options, ceremony_token: "ct-1" } });
    get.mockResolvedValue(null);

    await expect(login()).resolves.toBeNull();
    expect(post).toHaveBeenCalledTimes(1);
  });

  it("resolves to null when aborted before the browser is asked", async () => {
    const controller = new AbortController();
    post.mockImplementationOnce(async () => {
      controller.abort();
      throw controller.signal.reason;
    });

    await expect(login({ signal: controller.signal })).resolves.toBeNull();
    expect(get).not.toHaveBeenCalled();
  });

  it("throws the server's problem from begin", async () => {
    respond({ error: { status: 500, title: "Internal Server Error" } });

    await expect(login()).rejects.toMatchObject({ status: 500 });
  });

  it("throws the server's problem from finish", async () => {
    respond({ data: { options, ceremony_token: "ct-1" } }, { error: problem });
    get.mockResolvedValue(assertion);

    await expect(login()).rejects.toEqual(problem);
  });
});

const rename = (variables: { id: string; name: string }) =>
  renamePasskey.mutationFn!(
    variables,
    {} as Parameters<NonNullable<typeof renamePasskey.mutationFn>>[1],
  );

const remove = (variables: { id: string }) =>
  deletePasskey.mutationFn!(
    variables,
    {} as Parameters<NonNullable<typeof deletePasskey.mutationFn>>[1],
  );

describe("renamePasskey", () => {
  it("patches the passkey by id and resolves to it", async () => {
    const passkey = { id: "AQIDBA", name: "Phone", created_at: "2026-01-01T00:00:00Z" };
    patch.mockResolvedValue({ data: passkey } as never);

    await expect(rename({ id: "AQIDBA", name: "Phone" })).resolves.toEqual(passkey);
    expect(patch).toHaveBeenCalledWith("/v1/passkeys/{id}", {
      params: { path: { id: "AQIDBA" } },
      body: { name: "Phone" },
    });
  });

  it("throws the server's problem", async () => {
    const notFound = { status: 404, title: "Not Found", detail: "passkey not found" };
    patch.mockResolvedValue({ error: notFound } as never);

    await expect(rename({ id: "AQIDBA", name: "Phone" })).rejects.toEqual(notFound);
  });
});

describe("deletePasskey", () => {
  it("deletes the passkey by id", async () => {
    del.mockResolvedValue({} as never);

    await expect(remove({ id: "AQIDBA" })).resolves.toBeUndefined();
    expect(del).toHaveBeenCalledWith("/v1/passkeys/{id}", { params: { path: { id: "AQIDBA" } } });
  });

  it("throws the server's problem", async () => {
    const notFound = { status: 404, title: "Not Found", detail: "passkey not found" };
    del.mockResolvedValue({ error: notFound } as never);

    await expect(remove({ id: "AQIDBA" })).rejects.toEqual(notFound);
  });
});

const add = (variables: Parameters<NonNullable<typeof addPasskey.mutationFn>>[0]) =>
  addPasskey.mutationFn!(variables, {} as Parameters<NonNullable<typeof addPasskey.mutationFn>>[1]);

describe("addPasskey", () => {
  const creationOptions = { challenge: "Y2hhbGxlbmdl", rp: { id: "localhost" } };
  const attestation = { id: "AQIDBA", type: "public-key", response: {} };
  const passkey = { id: "AQIDBA", name: "Laptop", created_at: "2026-01-01T00:00:00Z" };

  it("begins, creates the credential and finishes with the ceremony token in a header", async () => {
    respond({ data: { options: creationOptions, ceremony_token: "ct-1" } }, { data: passkey });
    create.mockResolvedValue(attestation);

    await expect(add({ name: "Laptop" })).resolves.toEqual(passkey);

    expect(post).toHaveBeenNthCalledWith(1, "/v1/passkeys/register/begin");
    expect(create).toHaveBeenCalledWith(creationOptions);
    expect(post).toHaveBeenNthCalledWith(2, "/v1/passkeys/register/finish", {
      params: { header: { "X-Ceremony-Token": "ct-1" }, query: { name: "Laptop" } },
      body: attestation,
    });
    expect(setToken).not.toHaveBeenCalled();
  });

  it("throws the reauthentication problem from begin", async () => {
    const reauth = { status: 401, title: "Unauthorized", detail: REAUTH_REQUIRED };
    respond({ error: reauth });

    await expect(add({})).rejects.toEqual(reauth);
    expect(create).not.toHaveBeenCalled();
  });

  it("signs in again first when given the password, then runs the ceremony", async () => {
    respond(
      { data: { token: "fresh" } },
      { data: { options: creationOptions, ceremony_token: "ct-1" } },
      { data: passkey },
    );
    create.mockResolvedValue(attestation);

    await expect(
      add({ name: "Laptop", reauth: { email: "a@b.co", password: "secret" } }),
    ).resolves.toEqual(passkey);

    expect(post).toHaveBeenNthCalledWith(1, "/v1/auth/login", {
      body: { email: "a@b.co", password: "secret" },
    });
    expect(setToken).toHaveBeenCalledWith("fresh");
    expect(setToken.mock.invocationCallOrder[0]).toBeLessThan(post.mock.invocationCallOrder[1]);
    expect(post).toHaveBeenNthCalledWith(2, "/v1/passkeys/register/begin");
  });

  it("stops at a wrong password, keeping the old token", async () => {
    const wrong = { status: 401, title: "Unauthorized", detail: "invalid credentials" };
    respond({ error: wrong });

    await expect(add({ reauth: { email: "a@b.co", password: "nope" } })).rejects.toEqual(wrong);
    expect(setToken).not.toHaveBeenCalled();
    expect(post).toHaveBeenCalledTimes(1);
  });

  it("resolves to null without finishing when the user cancels", async () => {
    respond({ data: { options: creationOptions, ceremony_token: "ct-1" } });
    create.mockResolvedValue(null);

    await expect(add({})).resolves.toBeNull();
    expect(post).toHaveBeenCalledTimes(1);
  });
});
