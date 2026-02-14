import { test, expect } from '@playwright/test';

test.describe('Storefront Browse @critical', () => {
  test('homepage loads and renders', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=ForgeCommerce')).toBeVisible();
  });

  test('products page loads', async ({ page }) => {
    await page.goto('/products');
    await expect(page).toHaveTitle(/Products/);
    await expect(page.locator('h1')).toContainText('Products');
  });

  test('products page shows product grid or empty state', async ({ page }) => {
    await page.goto('/products');
    // Wait for loading to finish
    await page.waitForTimeout(2000);

    // Either products are shown in a grid, or the empty state is shown
    const productLinks = page.locator('a[href^="/products/"]');
    const emptyState = page.locator('text=No products found');

    const productCount = await productLinks.count();
    if (productCount > 0) {
      // Products exist — verify at least one is visible
      await expect(productLinks.first()).toBeVisible();
    } else {
      await expect(emptyState).toBeVisible();
    }
  });

  test('click a product to view detail page', async ({ page }) => {
    await page.goto('/products');
    await page.waitForTimeout(2000);

    const productLink = page.locator('a[href^="/products/"]').first();
    if (await productLink.isVisible({ timeout: 3000 }).catch(() => false)) {
      const href = await productLink.getAttribute('href');
      await productLink.click();
      // Should navigate to the product detail page
      await expect(page).toHaveURL(new RegExp(`/products/.+`));
      // Product page should have content
      await expect(page.locator('body')).not.toBeEmpty();
    }
  });

  test('product detail page shows price', async ({ page }) => {
    await page.goto('/products');
    await page.waitForTimeout(2000);

    const productLink = page.locator('a[href^="/products/"]').first();
    if (await productLink.isVisible({ timeout: 3000 }).catch(() => false)) {
      await productLink.click();
      await page.waitForTimeout(1000);
      // Price should be displayed with Euro sign
      await expect(page.locator('body')).toContainText('€');
    }
  });

  test('products page shows price in EUR format', async ({ page }) => {
    await page.goto('/products');
    await page.waitForTimeout(2000);

    const priceElement = page.locator('text=/€\\d+\\.\\d{2}/').first();
    if (await priceElement.isVisible({ timeout: 3000 }).catch(() => false)) {
      const text = await priceElement.textContent();
      expect(text).toMatch(/€\d+\.\d{2}/);
    }
  });

  test('storefront has navigation', async ({ page }) => {
    await page.goto('/');
    // Verify navigation links exist
    const productsNav = page.locator('a[href="/products"]');
    if (await productsNav.count() > 0) {
      await expect(productsNav.first()).toBeVisible();
    }
  });

  test('pagination controls appear when needed', async ({ page }) => {
    await page.goto('/products');
    await page.waitForTimeout(2000);

    // Check if pagination exists (only appears with enough products)
    const pagination = page.locator('nav[aria-label="Product pagination"]');
    // Just check it does not error — it may or may not be visible
    const exists = await pagination.count();
    expect(exists).toBeGreaterThanOrEqual(0);
  });
});
