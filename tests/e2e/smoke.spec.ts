import { test, expect } from '@playwright/test'

test.describe('Storefront Smoke Tests', () => {
  test('home page loads', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('text=ForgeCommerce')).toBeVisible()
  })

  test('products page loads', async ({ page }) => {
    await page.goto('/products')
    await expect(page).toHaveTitle(/Products/)
  })

  test('cart page loads', async ({ page }) => {
    await page.goto('/cart')
    await expect(page).toHaveTitle(/Cart/)
  })

  test('login page loads', async ({ page }) => {
    await page.goto('/login')
    await expect(page.locator('text=Sign In')).toBeVisible()
  })
})

test.describe('Admin Smoke Tests', () => {
  const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081'

  test('admin login page loads', async ({ page }) => {
    await page.goto(ADMIN_URL + '/admin/login')
    await expect(page.locator('input[type="email"]')).toBeVisible()
  })
})
