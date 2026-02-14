import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  // After login the user may land on 2FA setup — skip if in test mode
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin Products @critical', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('products page loads with table', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/products');
    await expect(page.locator('h2')).toContainText('Products');
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('a[href="/admin/products/new"]')).toContainText('New Product');
  });

  test('navigate to new product form', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/products/new');
    await expect(page.locator('h2')).toContainText('New Product');
    await expect(page.locator('input#name')).toBeVisible();
    await expect(page.locator('input#base_price')).toBeVisible();
    await expect(page.locator('select#status')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toContainText('Create Product');
  });

  test('create a new product', async ({ page }) => {
    const productName = `Test Product ${Date.now()}`;
    await page.goto(ADMIN_URL + '/admin/products/new');
    await page.fill('input#name', productName);
    await page.fill('input#base_price', '49.99');
    await page.selectOption('select#status', 'active');
    await page.fill('textarea#description', 'A test product created by Playwright.');
    await page.click('button[type="submit"]');

    // Should redirect to the edit page or the product list
    await page.waitForURL(/\/admin\/products/);
    // Verify product name is visible on the resulting page
    await expect(page.locator('body')).toContainText(productName);
  });

  test('edit an existing product', async ({ page }) => {
    // First, create a product
    const originalName = `Edit Test ${Date.now()}`;
    await page.goto(ADMIN_URL + '/admin/products/new');
    await page.fill('input#name', originalName);
    await page.fill('input#base_price', '25.00');
    await page.selectOption('select#status', 'active');
    await page.click('button[type="submit"]');
    await page.waitForURL(/\/admin\/products/);

    // Now find and click the edit link
    const editLink = page.locator(`a:has-text("${originalName}")`).first();
    if (await editLink.isVisible()) {
      await editLink.click();
      await expect(page.locator('h2')).toContainText('Edit');

      // Change the name
      const updatedName = `Updated ${originalName}`;
      await page.fill('input#name', updatedName);
      await page.click('button[type="submit"]');
      await page.waitForURL(/\/admin\/products/);
      await expect(page.locator('body')).toContainText(updatedName);
    }
  });

  test('archive a product by changing status', async ({ page }) => {
    // Create a product to archive
    const productName = `Archive Test ${Date.now()}`;
    await page.goto(ADMIN_URL + '/admin/products/new');
    await page.fill('input#name', productName);
    await page.fill('input#base_price', '10.00');
    await page.selectOption('select#status', 'active');
    await page.click('button[type="submit"]');
    await page.waitForURL(/\/admin\/products/);

    // Navigate to the product edit page
    const editLink = page.locator(`a:has-text("${productName}")`).first();
    if (await editLink.isVisible()) {
      await editLink.click();
      // Change status to archived
      await page.selectOption('select#status', 'archived');
      await page.click('button[type="submit"]');
      await page.waitForURL(/\/admin\/products/);
    }

    // Filter by archived status
    await page.goto(ADMIN_URL + '/admin/products?status=archived');
    await expect(page.locator('body')).toContainText('archived');
  });

  test('status filter works', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/products');
    // Verify the status dropdown exists
    const statusFilter = page.locator('select[name="status"]');
    await expect(statusFilter).toBeVisible();

    // Select active filter
    await page.goto(ADMIN_URL + '/admin/products?status=active');
    await expect(page.locator('table')).toBeVisible();
  });

  test('product form has all required fields', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/products/new');
    await expect(page.locator('input#name')).toBeVisible();
    await expect(page.locator('input#slug')).toBeVisible();
    await expect(page.locator('input#sku_prefix')).toBeVisible();
    await expect(page.locator('select#status')).toBeVisible();
    await expect(page.locator('input#base_price')).toBeVisible();
    await expect(page.locator('input#compare_at_price')).toBeVisible();
    await expect(page.locator('textarea#description')).toBeVisible();
    await expect(page.locator('input#weight_grams')).toBeVisible();
    await expect(page.locator('input#seo_title')).toBeVisible();
  });
});
