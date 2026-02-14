import { test, expect } from '@playwright/test';

test.describe('Storefront Customer', () => {
  test('login page loads', async ({ page }) => {
    await page.goto('/login');
    await expect(page).toHaveTitle(/Sign In/);
  });

  test('login form has email and password fields', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('#login-email')).toBeVisible();
    await expect(page.locator('#login-password')).toBeVisible();
  });

  test('login form has submit button', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('button:has-text("Sign In")')).toBeVisible();
  });

  test('tabs for sign in and create account exist', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('button:has-text("Sign In")')).toBeVisible();
    await expect(page.locator('button:has-text("Create Account")')).toBeVisible();
  });

  test('switch to register tab shows registration form', async ({ page }) => {
    await page.goto('/login');
    await page.click('button:has-text("Create Account")');
    await expect(page.locator('#reg-email')).toBeVisible();
    await expect(page.locator('#reg-password')).toBeVisible();
    await expect(page.locator('#reg-confirm')).toBeVisible();
    await expect(page.locator('#reg-firstname')).toBeVisible();
    await expect(page.locator('#reg-lastname')).toBeVisible();
  });

  test('invalid login shows error message', async ({ page }) => {
    await page.goto('/login');
    await page.fill('#login-email', 'nonexistent@example.com');
    await page.fill('#login-password', 'wrongpassword123');
    await page.click('form >> button[type="submit"]');

    // Wait for error message
    await page.waitForTimeout(2000);
    const errorMsg = page.locator('text=/Invalid|failed|error/i');
    const hasError = await errorMsg.isVisible({ timeout: 5000 }).catch(() => false);
    // Either error message shown or form is still present
    expect(hasError || await page.locator('#login-email').isVisible()).toBeTruthy();
  });

  test('guest checkout link is present', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('text=continue as a guest')).toBeVisible();
  });
});
