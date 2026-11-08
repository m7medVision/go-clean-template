import { expect, test } from "@playwright/test";

test("sign up, sign in, add a task, move it, sign out", async ({ page }) => {
  const id = `${Date.now()}${Math.floor(Math.random() * 1000)}`;
  const email = `e2e-${id}@example.com`;
  const password = "correct horse battery";

  await page.goto("/register");
  await page.getByLabel("Username").fill(`e2e${id}`);
  await page.getByLabel("Email address").fill(email);
  await page.getByRole("textbox", { name: /^Password/ }).fill(password);
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page.getByText("Welcome back")).toBeVisible();
  await page.getByLabel("Email address").fill(email);
  await page.getByRole("textbox", { name: /^Password/ }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();

  await expect(page.getByRole("heading", { name: "Tasks" })).toBeVisible();

  await page.getByRole("button", { name: "Add task" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel(/^Title/).fill("Ship the e2e suite");
  await dialog.getByRole("button", { name: "Add task" }).click();
  await expect(page.getByText("Ship the e2e suite")).toBeVisible();

  await page.getByRole("button", { name: "Actions for Ship the e2e suite" }).click();
  await page.getByRole("menuitem", { name: /Move to In progress/ }).click();
  await page.getByRole("button", { name: "Actions for Ship the e2e suite" }).click();
  await expect(page.getByRole("menuitem", { name: /Move to Done/ })).toBeVisible();
  await page.keyboard.press("Escape");

  await page.getByRole("button", { name: new RegExp(email.split("@")[0] ?? "", "i") }).click();
  await page.getByRole("menuitem", { name: /Log out/ }).click();
  await expect(page.getByText("Welcome back")).toBeVisible();
});
