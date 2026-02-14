import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin Production', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('production page loads', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/production');
    await expect(page.locator('h2')).toContainText('Production Batches');
    await expect(page.locator('table')).toBeVisible();
  });

  test('New Batch button is present', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/production');
    const newBatchLink = page.locator('a[href="/admin/production/new"]');
    await expect(newBatchLink).toBeVisible();
    await expect(newBatchLink).toContainText('New Batch');
  });

  test('status filter buttons are present', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/production');
    await expect(page.locator('a:has-text("All")')).toBeVisible();
    await expect(page.locator('a:has-text("Draft")')).toBeVisible();
    await expect(page.locator('a:has-text("Scheduled")')).toBeVisible();
    await expect(page.locator('a:has-text("In Progress")')).toBeVisible();
    await expect(page.locator('a:has-text("Completed")')).toBeVisible();
  });

  test('batch table has expected columns', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/production');
    await expect(page.locator('th:has-text("Batch #")')).toBeVisible();
    await expect(page.locator('th:has-text("Product")')).toBeVisible();
    await expect(page.locator('th:has-text("Status")')).toBeVisible();
    await expect(page.locator('th:has-text("Planned")')).toBeVisible();
    await expect(page.locator('th:has-text("Actual")')).toBeVisible();
  });

  test('navigate to new batch form', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/production/new');
    await expect(page.locator('h2')).toContainText('New Production Batch');
    await expect(page.locator('select#product_id')).toBeVisible();
    await expect(page.locator('input#planned_quantity')).toBeVisible();
    await expect(page.locator('input#scheduled_date')).toBeVisible();
    await expect(page.locator('textarea#notes')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toContainText('Create Batch');
  });

  test('new batch form has cancel link', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/production/new');
    await expect(page.locator('a:has-text("Cancel")')).toBeVisible();
  });
});
