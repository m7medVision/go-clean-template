import { defineConfig, devices } from "@playwright/test";

const port = process.env.WEB_PORT ?? "3000";

export default defineConfig({
  testDir: ".",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: `http://localhost:${port}`,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "vp dev",
    url: `http://localhost:${port}`,
    reuseExistingServer: !process.env.CI,
    cwd: "..",
    env: { WEB_PORT: port, HTTP_PORT: process.env.HTTP_PORT ?? "8080" },
  },
});
