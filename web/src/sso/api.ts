import { mutationOptions } from "@tanstack/react-query";

import { $api, api, type Problem } from "#/api/client";
import { ssoCeremony } from "./ceremony";

// Whether the server has an SSO provider configured; the login screen shows the button when it has.
export const ssoStatusQuery = $api.queryOptions("get", "/v1/auth/oidc");

const expired: Problem = {
  type: "about:blank",
  title: "Sign-in expired",
  detail: "This sign-in has expired. Start again from the sign-in page.",
};

// beginSso starts a sign-in at the provider and resolves to its URL; the caller sends the browser
// there. The ceremony token and `redirect`, where to land once signed in, wait for the callback.
export const beginSso = mutationOptions({
  mutationKey: ["sso", "begin"],
  mutationFn: async ({ redirect }: { redirect: string }) => {
    const { data, error } = await api.POST("/v1/auth/oidc/begin");
    if (error) throw error;

    ssoCeremony.save({ token: data.ceremony_token, redirect });

    return data.url;
  },
});

// finishSso trades the provider's code for the app's JWT, with the ceremony token this tab kept at
// begin. Without one (another tab, a reload after finishing, blocked storage) it fails as expired.
export const finishSso = mutationOptions({
  mutationKey: ["sso", "finish"],
  mutationFn: async ({ code, state }: { code: string; state: string }) => {
    const ceremony = ssoCeremony.take();
    if (!ceremony) throw expired;

    const { data, error } = await api.POST("/v1/auth/oidc/finish", {
      params: { header: { "X-Ceremony-Token": ceremony.token } },
      body: { code, state },
    });
    if (error) throw error;

    return { token: data.token, redirect: ceremony.redirect };
  },
});
