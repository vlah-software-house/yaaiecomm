import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin Orders', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('orders page loads with table', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/orders');
    await expect(page.locator('h2')).toContainText('Orders');
    await expect(page.locator('table')).toBeVisible();
    // Table headers should be present
    await expect(page.locator('th:has-text("Order #")')).toBeVisible();
    await expect(page.locator('th:has-text("Customer")')).toBeVisible();
    await expect(page.locator('th:has-text("Status")')).toBeVisible();
    await expect(page.locator('th:has-text("Total")')).toBeVisible();
    await expect(page.locator('th:has-text("VAT")')).toBeVisible();
  });

  test('status filter buttons are present', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/orders');
    await expect(page.locator('a:has-text("All")')).toBeVisible();
    await expect(page.locator('a:has-text("Pending")')).toBeVisible();
    await expect(page.locator('a:has-text("Confirmed")')).toBeVisible();
    await expect(page.locator('a:has-text("Processing")')).toBeVisible();
    await expect(page.locator('a:has-text("Shipped")')).toBeVisible();
    await expect(page.locator('a:has-text("Delivered")')).toBeVisible();
  });

  test('filter by pending status', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/orders?status=pending');
    await expect(page.locator('table')).toBeVisible();
    // Page should still load without errors
    await expect(page.locator('h2')).toContainText('Orders');
  });

  test('order detail page loads when orders exist', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/orders');
    // Try to click the first order link if any exist
    const firstOrderLink = page.locator('tbody a').first();
    if (await firstOrderLink.isVisible({ timeout: 3000 }).catch(() => false)) {
      await firstOrderLink.click();
      // Order detail page should show relevant sections
      await expect(page.locator('text=Order Information')).toBeVisible();
      await expect(page.locator('text=Order Items')).toBeVisible();
      await expect(page.locator('text=Order Totals')).toBeVisible();
      // Actions card with update status
      await expect(page.locator('text=Actions')).toBeVisible();
      // VAT information should be in the totals section
      await expect(page.locator('text=VAT')).toBeVisible();
    }
  });

  test('empty state shows message when no orders', async ({ page }) => {
    // Filter to a status that likely has no orders
    await page.goto(ADMIN_URL + '/admin/orders?status=cancelled');
    const emptyMessage = page.locator('text=No orders found');
    // Either orders exist or the empty message is shown
    const tableHasRows = await page.locator('tbody tr a').count();
    if (tableHasRows === 0) {
      await expect(emptyMessage).toBeVisible();
    }
  });
});
