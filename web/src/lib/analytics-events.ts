// The analytics events the web client sends; the server's are in internal/entity/analytics.
export const analyticsEvents = {
  userLoggedIn: "user_logged_in",
  taskAttributesUpdated: "task_attributes_updated",
  pageViewed: "page_viewed",
} as const;

export type AnalyticsEvent = (typeof analyticsEvents)[keyof typeof analyticsEvents];

export type SignInMethodName = "password" | "passkey" | "oidc";

export type AnalyticsProperties = {
  user_logged_in: { method: SignInMethodName };
  task_attributes_updated: { fields: string[] };
  page_viewed: { path: string };
};
