import { test, expect } from '@playwright/test';

test.describe('Admin Login @critical', () => {
  test('shows login page', async ({ page }) => {
    await page.goto('/admin/login');

    await expect(page).toHaveTitle(/Login — ForgeCommerce Admin/);
    await expect(page.locator('h1')).toContainText('ForgeCommerce');
    await expect(page.locator('input[name="email"]')).toBeVisible();
    await expect(page.locator('input[name="password"]')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toContainText('Sign In');
  });

  test('rejects invalid credentials', async ({ page }) => {
    await page.goto('/admin/login');

    await page.fill('input[name="email"]', 'wrong@example.com');
    await page.fill('input[name="password"]', 'wrongpassword');
    await page.click('button[type="submit"]');

    await expect(page.locator('.alert-error')).toContainText('Invalid email or password');
  });

  test('redirects to 2FA after valid credentials', async ({ page }) => {
    await page.goto('/admin/login');

    await page.fill('input[name="email"]', 'admin@forgecommerce.local');
    await page.fill('input[name="password"]', 'admin123');
    await page.click('button[type="submit"]');

    // User has force_2fa_setup=true, so should redirect to setup-2fa
    await expect(page).toHaveURL(/\/admin\/setup-2fa/);
    await expect(page.locator('h1')).toContainText('Set Up 2FA');
  });

  test('root redirects to login', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL(/\/admin\/login/);
  });

  test('protected routes redirect to login when not authenticated', async ({ page }) => {
    await page.goto('/admin/dashboard');
    await expect(page).toHaveURL(/\/admin\/login/);
  });

  test('health check returns ok', async ({ page }) => {
    const response = await page.goto('/admin/health');
    expect(response?.status()).toBe(200);
    const body = await response?.text();
    expect(body).toContain('"status":"ok"');
  });
});
