import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin Reports', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('sales report page loads', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/reports/sales');
    await expect(page.locator('h2')).toContainText('Sales Report');
  });

  test('sales report has period selector', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/reports/sales');
    await expect(page.locator('text=Report Period')).toBeVisible();
    await expect(page.locator('input#from')).toBeVisible();
    await expect(page.locator('input#to')).toBeVisible();
    await expect(page.locator('button:has-text("Generate")')).toBeVisible();
  });

  test('sales report has CSV export link', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/reports/sales');
    const exportLink = page.locator('#export-csv-link');
    await expect(exportLink).toBeVisible();
    await expect(exportLink).toContainText('Export CSV');
  });

  test('sales report data section loads', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/reports/sales');
    // The report data div should exist (loaded via HTMX)
    await expect(page.locator('#report-data')).toBeVisible();
  });

  test('VAT report page loads', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/reports/vat');
    await expect(page.locator('h2')).toContainText('VAT Report');
  });

  test('VAT report has period selector with period type', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/reports/vat');
    await expect(page.locator('text=Report Period')).toBeVisible();
    // Period type dropdown
    await expect(page.locator('select#period')).toBeVisible();
    const periodSelect = page.locator('select#period');
    await expect(periodSelect.locator('option[value="monthly"]')).toHaveText('Monthly');
    await expect(periodSelect.locator('option[value="quarterly"]')).toHaveText('Quarterly');
    await expect(periodSelect.locator('option[value="yearly"]')).toHaveText('Yearly');
  });

  test('VAT report has CSV export link', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/reports/vat');
    const exportLink = page.locator('#vat-export-csv-link');
    await expect(exportLink).toBeVisible();
    await expect(exportLink).toContainText('Export CSV');
  });
});
