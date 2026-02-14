import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin Users', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('users page loads with table', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/users');
    await expect(page.locator('h2')).toContainText('Admin Users');
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('a[href="/admin/users/new"]')).toContainText('New User');
  });

  test('user table has expected columns', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/users');
    await expect(page.locator('th:has-text("Name")')).toBeVisible();
    await expect(page.locator('th:has-text("Email")')).toBeVisible();
    await expect(page.locator('th:has-text("Role")')).toBeVisible();
    await expect(page.locator('th:has-text("2FA")')).toBeVisible();
    await expect(page.locator('th:has-text("Status")')).toBeVisible();
  });

  test('current admin user is visible in the list', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/users');
    // The seeded admin user should be visible
    await expect(page.locator('text=admin@forgecommerce.local')).toBeVisible();
  });

  test('navigate to new user form', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/users/new');
    await expect(page.locator('h2')).toContainText('New User');
    await expect(page.locator('input#name')).toBeVisible();
    await expect(page.locator('input#email')).toBeVisible();
    await expect(page.locator('input#password')).toBeVisible();
    await expect(page.locator('select#role')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toContainText('Create User');
  });
});
