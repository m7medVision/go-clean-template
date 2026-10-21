import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { api } from "#/api/client";
import { problemMessage } from "#/api/problem";
import { beginSso, finishSso } from "./api";
import { ssoCeremony } from "./ceremony";

vi.mock("#/api/client", () => ({
  api: { POST: vi.fn() },
  $api: { queryOptions: () => ({ queryKey: ["get", "/v1/auth/oidc"] }) },
}));
vi.mock("./ceremony", () => ({ ssoCeremony: { save: vi.fn(), take: vi.fn() } }));

const post = vi.mocked(api.POST);
const save = vi.mocked(ssoCeremony.save);
const take = vi.mocked(ssoCeremony.take);

const run = <V, R>(options: { mutationFn?: (v: V, c: never) => Promise<R> }, variables: V) =>
  options.mutationFn!(variables, {} as never);

beforeEach(() => {
  post.mockReset();
  save.mockReset();
  take.mockReset();
});

describe("beginSso", () => {
  it("keeps the ceremony token and the redirect, and resolves to the provider URL", async () => {
    post.mockResolvedValue({
      data: { url: "http://idp/auth?state=s", ceremony_token: "ct-1" },
    } as never);

    await expect(run(beginSso, { redirect: "/tasks" })).resolves.toBe("http://idp/auth?state=s");

    expect(post).toHaveBeenCalledWith("/v1/auth/oidc/begin");
    expect(save).toHaveBeenCalledWith({ token: "ct-1", redirect: "/tasks" });
  });

  it("throws the server's problem and keeps nothing", async () => {
    const problem = { status: 502, title: "Bad Gateway", detail: "sso provider unavailable" };
    post.mockResolvedValue({ error: problem } as never);

    await expect(run(beginSso, { redirect: "/tasks" })).rejects.toEqual(problem);
    expect(save).not.toHaveBeenCalled();
  });
});

describe("finishSso", () => {
  it("finishes with the kept ceremony token and resolves to the JWT and the redirect", async () => {
    take.mockReturnValue({ token: "ct-1", redirect: "/exchange" });
    post.mockResolvedValue({ data: { token: "jwt-1" } } as never);

    await expect(run(finishSso, { code: "c", state: "s" })).resolves.toEqual({
      token: "jwt-1",
      redirect: "/exchange",
    });

    expect(post).toHaveBeenCalledWith("/v1/auth/oidc/finish", {
      params: { header: { "X-Ceremony-Token": "ct-1" } },
      body: { code: "c", state: "s" },
    });
  });

  it("fails without calling the server when this tab did not begin the sign-in", async () => {
    take.mockReturnValue(null);

    const error = await run(finishSso, { code: "c", state: "s" }).catch((e: unknown) => e);

    expect(problemMessage(error)).toBe(
      "This sign-in has expired. Start again from the sign-in page.",
    );
    expect(post).not.toHaveBeenCalled();
  });

  it("throws the server's problem", async () => {
    take.mockReturnValue({ token: "ct-1", redirect: "/tasks" });
    const problem = {
      status: 409,
      title: "Conflict",
      detail: "an account with this email already exists",
    };
    post.mockResolvedValue({ error: problem } as never);

    await expect(run(finishSso, { code: "c", state: "s" })).rejects.toEqual(problem);
  });
});
