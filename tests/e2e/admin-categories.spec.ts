import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin Categories', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('categories page loads with table', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/categories');
    await expect(page.locator('h2')).toContainText('Categories');
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('a[href="/admin/categories/new"]')).toContainText('New Category');
  });

  test('navigate to new category form', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/categories/new');
    await expect(page.locator('h2')).toContainText('New Category');
    await expect(page.locator('input#name')).toBeVisible();
    await expect(page.locator('input#slug')).toBeVisible();
    await expect(page.locator('select#parent_id')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toContainText('Create Category');
  });

  test('create a category with name and slug', async ({ page }) => {
    const categoryName = `Cat ${Date.now()}`;
    const categorySlug = `cat-${Date.now()}`;
    await page.goto(ADMIN_URL + '/admin/categories/new');
    await page.fill('input#name', categoryName);
    await page.fill('input#slug', categorySlug);
    await page.click('button[type="submit"]');

    await page.waitForURL(/\/admin\/categories/);
    await expect(page.locator('body')).toContainText(categoryName);
  });

  test('edit a category', async ({ page }) => {
    // Create a category first
    const categoryName = `EditCat ${Date.now()}`;
    await page.goto(ADMIN_URL + '/admin/categories/new');
    await page.fill('input#name', categoryName);
    await page.click('button[type="submit"]');
    await page.waitForURL(/\/admin\/categories/);

    // Find and click edit
    const link = page.locator(`a:has-text("${categoryName}")`).first();
    if (await link.isVisible()) {
      await link.click();
      await expect(page.locator('h2')).toContainText('Edit');
      const updatedName = `Updated ${categoryName}`;
      await page.fill('input#name', updatedName);
      await page.click('button[type="submit"]');
      await page.waitForURL(/\/admin\/categories/);
      await expect(page.locator('body')).toContainText(updatedName);
    }
  });

  test('parent category dropdown shows options', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/categories/new');
    const parentSelect = page.locator('select#parent_id');
    await expect(parentSelect).toBeVisible();
    // "None (top-level)" option should always be present
    await expect(parentSelect.locator('option[value=""]')).toHaveText('None (top-level)');
  });
});
