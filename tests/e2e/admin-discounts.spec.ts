import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin Discounts', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('discounts page loads with table', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/discounts');
    await expect(page.locator('h2')).toContainText('Discounts');
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('a[href="/admin/discounts/new"]')).toContainText('New Discount');
  });

  test('navigate to new discount form', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/discounts/new');
    await expect(page.locator('h2')).toContainText('New Discount');
    await expect(page.locator('input#name')).toBeVisible();
    await expect(page.locator('select#type')).toBeVisible();
    await expect(page.locator('input#value')).toBeVisible();
    await expect(page.locator('select#scope')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toContainText('Create Discount');
  });

  test('create a percentage discount', async ({ page }) => {
    const discountName = `Pct Discount ${Date.now()}`;
    await page.goto(ADMIN_URL + '/admin/discounts/new');
    await page.fill('input#name', discountName);
    await page.selectOption('select#type', 'percentage');
    await page.fill('input#value', '15');
    await page.selectOption('select#scope', 'subtotal');
    await page.check('input[name="is_active"]');
    await page.click('button[type="submit"]');

    await page.waitForURL(/\/admin\/discounts/);
    await expect(page.locator('body')).toContainText(discountName);
  });

  test('create a fixed amount discount', async ({ page }) => {
    const discountName = `Fixed Discount ${Date.now()}`;
    await page.goto(ADMIN_URL + '/admin/discounts/new');
    await page.fill('input#name', discountName);
    await page.selectOption('select#type', 'fixed_amount');
    await page.fill('input#value', '10.00');
    await page.selectOption('select#scope', 'total');
    await page.check('input[name="is_active"]');
    await page.click('button[type="submit"]');

    await page.waitForURL(/\/admin\/discounts/);
    await expect(page.locator('body')).toContainText(discountName);
  });

  test('discount form has all fields', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/discounts/new');
    await expect(page.locator('input#minimum_amount')).toBeVisible();
    await expect(page.locator('input#maximum_discount')).toBeVisible();
    await expect(page.locator('input#priority')).toBeVisible();
    await expect(page.locator('input[name="stackable"]')).toBeVisible();
    await expect(page.locator('input#starts_at')).toBeVisible();
    await expect(page.locator('input#ends_at')).toBeVisible();
  });

  test('discount type dropdown has expected options', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/discounts/new');
    const typeSelect = page.locator('select#type');
    await expect(typeSelect.locator('option[value="percentage"]')).toHaveText('Percentage');
    await expect(typeSelect.locator('option[value="fixed_amount"]')).toHaveText('Fixed Amount');
  });
});
