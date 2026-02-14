import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('dashboard page loads', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/dashboard');
    await expect(page.locator('h2')).toContainText('Dashboard');
  });

  test('dashboard has stat widgets', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/dashboard');
    // Four stat cards from the template
    await expect(page.locator('text=Orders Today')).toBeVisible();
    await expect(page.locator('text=Revenue (Month)')).toBeVisible();
    await expect(page.locator('text=Low Stock Items')).toBeVisible();
    await expect(page.locator('text=Pending Orders')).toBeVisible();
  });

  test('dashboard has stat value placeholders that load via HTMX', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/dashboard');
    // The stat-value spans should exist
    const statValues = page.locator('.stat-value');
    const count = await statValues.count();
    expect(count).toBeGreaterThanOrEqual(4);
  });

  test('recent orders section is present', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/dashboard');
    await expect(page.locator('text=Recent Orders')).toBeVisible();
  });

  test('dashboard grid layout renders', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/dashboard');
    await expect(page.locator('.dashboard-grid')).toBeVisible();
    // Should have multiple cards in the grid
    const cards = page.locator('.dashboard-grid .card');
    const cardCount = await cards.count();
    expect(cardCount).toBeGreaterThanOrEqual(4);
  });
});
