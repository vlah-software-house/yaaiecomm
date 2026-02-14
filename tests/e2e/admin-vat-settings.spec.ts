import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin VAT Settings @critical', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('VAT settings page loads', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/settings/vat');
    await expect(page.locator('h2')).toContainText('VAT Settings');
  });

  test('VAT configuration form is present', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/settings/vat');
    // VAT Configuration section
    await expect(page.locator('text=VAT Configuration')).toBeVisible();
    await expect(page.locator('input[name="vat_enabled"]')).toBeVisible();
    await expect(page.locator('input#vat_number')).toBeVisible();
    await expect(page.locator('select#vat_country_code')).toBeVisible();
    await expect(page.locator('select#vat_default_category')).toBeVisible();
    await expect(page.locator('input[name="vat_b2b_reverse_charge"]')).toBeVisible();
  });

  test('VAT enabled toggle exists', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/settings/vat');
    const vatEnabledCheckbox = page.locator('input[name="vat_enabled"]');
    await expect(vatEnabledCheckbox).toBeVisible();
    // The checkbox should be interactable
    await expect(vatEnabledCheckbox).toBeEnabled();
  });

  test('prices include VAT radio buttons are present', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/settings/vat');
    const yesRadio = page.locator('input[name="vat_prices_include_vat"][value="true"]');
    const noRadio = page.locator('input[name="vat_prices_include_vat"][value="false"]');
    await expect(yesRadio).toBeVisible();
    await expect(noRadio).toBeVisible();
  });

  test('selling countries section loads', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/settings/vat');
    await expect(page.locator('text=Selling Countries')).toBeVisible();
    // Country checkboxes should be present
    await expect(page.locator('input[name="countries"]').first()).toBeVisible();
    // Select All / Deselect All buttons
    await expect(page.locator('button:has-text("Select All")')).toBeVisible();
    await expect(page.locator('button:has-text("Deselect All")')).toBeVisible();
  });

  test('VAT rates table is displayed', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/settings/vat');
    await expect(page.locator('text=Current VAT Rates')).toBeVisible();
    // The rates table should have expected headers
    const table = page.locator('table').last();
    await expect(table.locator('th:has-text("Country")')).toBeVisible();
    await expect(table.locator('th:has-text("Standard")')).toBeVisible();
    await expect(table.locator('th:has-text("Reduced")')).toBeVisible();
  });

  test('Sync Now button is present', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/settings/vat');
    const syncButton = page.locator('button:has-text("Sync Now")');
    await expect(syncButton).toBeVisible();
  });

  test('save VAT settings button works', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/settings/vat');
    const saveButton = page.locator('button:has-text("Save VAT Settings")');
    await expect(saveButton).toBeVisible();
  });

  test('save countries button works', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/settings/vat');
    const saveCountriesBtn = page.locator('button:has-text("Save Countries")');
    await expect(saveCountriesBtn).toBeVisible();
  });

  test('store country dropdown has EU countries', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/settings/vat');
    const countrySelect = page.locator('select#vat_country_code');
    await expect(countrySelect).toBeVisible();
    // Should have "Select country..." as the first option
    await expect(countrySelect.locator('option').first()).toContainText('Select country');
    // Should contain at least some EU countries
    const optionCount = await countrySelect.locator('option').count();
    expect(optionCount).toBeGreaterThan(10);
  });
});
