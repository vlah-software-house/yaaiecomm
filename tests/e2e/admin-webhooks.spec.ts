import { test, expect, Page } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';

async function adminLogin(page: Page) {
  await page.goto(ADMIN_URL + '/admin/login');
  await page.fill('input[name="email"]', 'admin@forgecommerce.local');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('button[type="submit"]');
  await page.waitForURL(/\/admin\//);
}

test.describe('Admin Webhooks', () => {
  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('webhooks page loads with table', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/webhooks');
    await expect(page.locator('h2')).toContainText('Webhook Endpoints');
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('a[href="/admin/webhooks/new"]')).toContainText('New Endpoint');
  });

  test('webhook table has expected columns', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/webhooks');
    await expect(page.locator('th:has-text("URL")')).toBeVisible();
    await expect(page.locator('th:has-text("Description")')).toBeVisible();
    await expect(page.locator('th:has-text("Events")')).toBeVisible();
    await expect(page.locator('th:has-text("Status")')).toBeVisible();
  });

  test('navigate to new webhook form', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/webhooks/new');
    await expect(page.locator('h2')).toContainText('New Webhook Endpoint');
    await expect(page.locator('input#url')).toBeVisible();
    await expect(page.locator('input#secret')).toBeVisible();
    await expect(page.locator('input#description')).toBeVisible();
  });

  test('webhook form has event checkboxes', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/webhooks/new');
    // There should be event checkboxes
    const eventCheckboxes = page.locator('input[name="events"]');
    const count = await eventCheckboxes.count();
    expect(count).toBeGreaterThan(0);
  });

  test('webhook form has active checkbox', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/webhooks/new');
    await expect(page.locator('input[name="is_active"]')).toBeVisible();
  });

  test('webhook form has submit and cancel', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/webhooks/new');
    await expect(page.locator('button[type="submit"]')).toContainText('Create Endpoint');
    await expect(page.locator('a:has-text("Cancel")')).toBeVisible();
  });

  test('fill in webhook URL and check events', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/webhooks/new');
    await page.fill('input#url', 'https://example.com/webhook');
    await page.fill('input#secret', 'test-secret-key-123');
    await page.fill('input#description', 'Test webhook endpoint');

    // Check the first event checkbox if any exist
    const firstEvent = page.locator('input[name="events"]').first();
    if (await firstEvent.isVisible()) {
      await firstEvent.check();
      await expect(firstEvent).toBeChecked();
    }

    // Verify form is filled
    await expect(page.locator('input#url')).toHaveValue('https://example.com/webhook');
    await expect(page.locator('input#description')).toHaveValue('Test webhook endpoint');
  });
});
