import { test, expect, Page } from '@playwright/test';

// --- helpers ---

async function fillAndSubmit(page: Page, email: string, repo: string) {
  await page.fill('#email', email);
  await page.fill('#repo', repo);
  await page.click('button[type=submit]');
}

function uniqueEmail(): string {
  // crypto.randomUUID is available in Node 19+ and all modern browsers
  const suffix = Math.random().toString(36).slice(2, 10);
  return `test-${suffix}@example.com`;
}

// --- tests ---

test.beforeEach(async ({ page }) => {
  await page.goto('/');
});

test('happy path: subscribe with valid inputs shows success', async ({ page }) => {
  const email = uniqueEmail();

  await fillAndSubmit(page, email, 'golang/go');

  const msg = page.locator('#message');
  await expect(msg).toBeVisible();
  await expect(msg).toHaveClass(/success/);
  await expect(msg).toContainText('check your email');
});

test('invalid email shows error message', async ({ page }) => {
  await fillAndSubmit(page, 'notanemail', 'golang/go');

  const msg = page.locator('#message');
  await expect(msg).toBeVisible();
  await expect(msg).toHaveClass(/error/);
  await expect(msg).toContainText('email');
});

test('invalid repo format shows error message', async ({ page }) => {
  await fillAndSubmit(page, uniqueEmail(), 'noslash');

  const msg = page.locator('#message');
  await expect(msg).toBeVisible();
  await expect(msg).toHaveClass(/error/);
  await expect(msg).toContainText('repo');
});

test('empty fields show error message', async ({ page }) => {
  await fillAndSubmit(page, '', '');

  const msg = page.locator('#message');
  await expect(msg).toBeVisible();
  await expect(msg).toHaveClass(/error/);
});

test('already subscribed shows conflict error', async ({ page }) => {
  const email = uniqueEmail();

  // first subscription succeeds
  await fillAndSubmit(page, email, 'golang/go');
  await expect(page.locator('#message')).toContainText('check your email');

  // second subscription for same email+repo must fail with 409
  await fillAndSubmit(page, email, 'golang/go');

  const msg = page.locator('#message');
  await expect(msg).toHaveClass(/error/);
  await expect(msg).toContainText('already subscribed');
});
