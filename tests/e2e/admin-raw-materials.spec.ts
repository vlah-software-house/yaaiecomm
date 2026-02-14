import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin Raw Materials', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('raw materials page loads with table', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/inventory/raw-materials');
    await expect(page.locator('h2')).toContainText('Raw Materials');
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('a[href="/admin/inventory/raw-materials/new"]')).toContainText('New Material');
  });

  test('navigate to new material form', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/inventory/raw-materials/new');
    await expect(page.locator('h2')).toContainText('New Raw Material');
    await expect(page.locator('input#name')).toBeVisible();
    await expect(page.locator('input#sku')).toBeVisible();
    await expect(page.locator('select#unit_of_measure')).toBeVisible();
    await expect(page.locator('input#cost_per_unit')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toContainText('Create Material');
  });

  test('create a raw material', async ({ page }) => {
    const materialName = `Material ${Date.now()}`;
    const materialSku = `MAT-${Date.now()}`;
    await page.goto(ADMIN_URL + '/admin/inventory/raw-materials/new');
    await page.fill('input#name', materialName);
    await page.fill('input#sku', materialSku);
    await page.selectOption('select#unit_of_measure', 'piece');
    await page.fill('input#cost_per_unit', '5.50');
    await page.fill('input#stock_quantity', '100');
    await page.click('button[type="submit"]');

    await page.waitForURL(/\/admin\/inventory\/raw-materials/);
    await expect(page.locator('body')).toContainText(materialName);
  });

  test('edit a raw material', async ({ page }) => {
    // Create first
    const materialName = `EditMat ${Date.now()}`;
    await page.goto(ADMIN_URL + '/admin/inventory/raw-materials/new');
    await page.fill('input#name', materialName);
    await page.fill('input#sku', `EMAT-${Date.now()}`);
    await page.fill('input#cost_per_unit', '3.00');
    await page.fill('input#stock_quantity', '50');
    await page.click('button[type="submit"]');
    await page.waitForURL(/\/admin\/inventory\/raw-materials/);

    // Find and edit
    const link = page.locator(`a:has-text("${materialName}")`).first();
    if (await link.isVisible()) {
      await link.click();
      await expect(page.locator('h2')).toContainText('Edit');
      await page.fill('input#stock_quantity', '200');
      await page.click('button[type="submit"]');
      await page.waitForURL(/\/admin\/inventory\/raw-materials/);
    }
  });

  test('unit of measure dropdown has expected options', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/inventory/raw-materials/new');
    const unitSelect = page.locator('select#unit_of_measure');
    await expect(unitSelect).toBeVisible();
    await expect(unitSelect.locator('option[value="piece"]')).toHaveText('Piece');
    await expect(unitSelect.locator('option[value="meter"]')).toHaveText('Meter');
    await expect(unitSelect.locator('option[value="kg"]')).toHaveText('Kilogram');
  });
});
