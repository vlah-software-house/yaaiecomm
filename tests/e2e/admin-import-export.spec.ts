import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin Import/Export', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('import page loads', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/import');
    await expect(page.locator('h1')).toContainText('CSV Import');
  });

  test('product import form is present', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/import');
    await expect(page.locator('text=Import Products')).toBeVisible();
    // File upload input
    const fileInput = page.locator('form[action="/admin/import/products"] input[type="file"]');
    await expect(fileInput).toBeVisible();
    // Submit button
    await expect(page.locator('form[action="/admin/import/products"] button[type="submit"]')).toBeVisible();
  });

  test('raw materials import form is present', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/import');
    await expect(page.locator('text=Import Raw Materials')).toBeVisible();
    const fileInput = page.locator('form[action="/admin/import/raw-materials"] input[type="file"]');
    await expect(fileInput).toBeVisible();
    await expect(page.locator('form[action="/admin/import/raw-materials"] button[type="submit"]')).toBeVisible();
  });

  test('CSV export endpoints are accessible', async ({ page }) => {
    // Verify the export URLs return a response (they download CSV files)
    const productsExport = await page.goto(ADMIN_URL + '/admin/export/products/csv');
    expect(productsExport?.status()).toBe(200);

    const materialsExport = await page.goto(ADMIN_URL + '/admin/export/raw-materials/csv');
    expect(materialsExport?.status()).toBe(200);

    const ordersExport = await page.goto(ADMIN_URL + '/admin/export/orders/csv');
    expect(ordersExport?.status()).toBe(200);
  });
});
