import { test, expect } from '@playwright/test';

test.describe('Storefront Checkout @critical', () => {
  test('checkout success page loads', async ({ page }) => {
    await page.goto('/checkout/success');
    await expect(page).toHaveTitle(/Order Confirmed/);
    await expect(page.locator('h1')).toContainText('Thank You');
  });

  test('checkout success page shows session reference when provided', async ({ page }) => {
    await page.goto('/checkout/success?session_id=cs_test_123');
    await expect(page.locator('h1')).toContainText('Thank You');
    await expect(page.locator('text=cs_test_123')).toBeVisible();
  });

  test('checkout success page has continue shopping link', async ({ page }) => {
    await page.goto('/checkout/success');
    await expect(page.locator('a:has-text("Continue Shopping")')).toBeVisible();
  });

  test('checkout success page has back to home link', async ({ page }) => {
    await page.goto('/checkout/success');
    await expect(page.locator('a:has-text("Back to Home")')).toBeVisible();
  });

  test('checkout cancel page loads', async ({ page }) => {
    await page.goto('/checkout/cancel');
    await expect(page).toHaveTitle(/Checkout Cancelled/);
    await expect(page.locator('h1')).toContainText('Checkout Cancelled');
  });

  test('checkout cancel page has return to cart link', async ({ page }) => {
    await page.goto('/checkout/cancel');
    await expect(page.locator('a:has-text("Return to Cart")')).toBeVisible();
  });

  test('checkout cancel page has continue shopping link', async ({ page }) => {
    await page.goto('/checkout/cancel');
    await expect(page.locator('a:has-text("Continue Shopping")')).toBeVisible();
  });

  test('checkout cancel page informs no payment taken', async ({ page }) => {
    await page.goto('/checkout/cancel');
    await expect(page.locator('body')).toContainText('no payment has been taken');
  });

  test('cart page checkout button disabled without country', async ({ page }) => {
    await page.goto('/cart');
    await page.waitForTimeout(2000);
    const checkoutBtn = page.locator('button:has-text("Proceed to Checkout")');
    if (await checkoutBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      // Button should be disabled when no country is selected
      await expect(checkoutBtn).toBeDisabled();
    }
  });
});
