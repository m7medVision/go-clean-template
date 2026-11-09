import { afterEach, describe, expect, it, vi } from "vite-plus/test";

import { analyticsEvents } from "./analytics-events";

const vendor = vi.hoisted(() => ({
  init: vi.fn(),
  identify: vi.fn(),
  capture: vi.fn(),
  reset: vi.fn(),
}));

vi.mock("posthog-js", () => ({ default: vendor }));

const jwt = (payload: unknown) =>
  ["e30", btoa(JSON.stringify(payload)).replaceAll("=", ""), "sig"].join(".");

const load = async (key: string) => {
  vi.resetModules();
  vi.stubEnv("VITE_POSTHOG_KEY", key);
  const analytics = await import("./analytics");
  const { auth } = await import("#/auth/token");
  await analytics.initAnalytics();

  return { ...analytics, auth };
};

describe("analytics", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.clearAllMocks();
  });

  it("loads and sends nothing when no key was built in", async () => {
    const analytics = await load("");

    analytics.pageview("/tasks");
    analytics.signedIn("password");

    expect(vendor.init).not.toHaveBeenCalled();
    expect(vendor.capture).not.toHaveBeenCalled();
  });

  it("masks every input and all text in replays and captures no page view by itself", async () => {
    await load("key");

    expect(vendor.init).toHaveBeenCalledWith(
      "key",
      expect.objectContaining({
        capture_pageview: false,
        autocapture: false,
        session_recording: { maskAllInputs: true, maskTextSelector: "*" },
      }),
    );
  });

  it("sends a page view with the route template under PostHog's name", async () => {
    const analytics = await load("key");

    analytics.pageview("/tasks/$id");

    expect(vendor.capture).toHaveBeenCalledWith("$pageview", { path: "/tasks/$id" });
  });

  it("identifies the user by ID after a sign-in, then resets on sign-out", async () => {
    const analytics = await load("key");
    analytics.auth.set(jwt({ sub: "user-123", email: "jane@example.com" }));

    analytics.signedIn("passkey");
    analytics.signedOut();
    analytics.auth.clear();

    expect(vendor.identify).toHaveBeenCalledWith("user-123");
    expect(vendor.capture).toHaveBeenCalledWith(analyticsEvents.userLoggedIn, {
      method: "passkey",
    });
    expect(vendor.reset).toHaveBeenCalledTimes(1);
  });

  it("sends field names, never values", async () => {
    const analytics = await load("key");

    analytics.capture(analyticsEvents.taskAttributesUpdated, { fields: ["priority"] });

    expect(vendor.capture).toHaveBeenCalledWith("task_attributes_updated", {
      fields: ["priority"],
    });
  });
});
