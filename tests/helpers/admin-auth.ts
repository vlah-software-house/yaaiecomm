import { Page, expect } from '@playwright/test';

export const ADMIN_EMAIL = 'admin@forgecommerce.local';
export const ADMIN_PASSWORD = 'admin123';

/**
 * Navigate to the admin login page and submit credentials.
 * Does NOT handle 2FA — returns after the login form is submitted.
 */
export async function loginAsAdmin(page: Page, email = ADMIN_EMAIL, password = ADMIN_PASSWORD) {
  await page.goto('/admin/login');
  await page.fill('input[name="email"]', email);
  await page.fill('input[name="password"]', password);
  await page.click('button[type="submit"]');
}

/**
 * Complete the full admin login including 2FA TOTP code entry.
 * Requires the TOTP secret to generate the code.
 */
export async function loginWithTOTP(page: Page, totpSecret: string, email = ADMIN_EMAIL, password = ADMIN_PASSWORD) {
  await loginAsAdmin(page, email, password);

  // Should be redirected to 2FA page
  await expect(page).toHaveURL(/\/admin\/login\/2fa/);

  // Generate TOTP code (requires otpauth library or similar)
  // For now, this is a placeholder — in real tests we'd use a TOTP library
  const code = generateTOTPCode(totpSecret);
  await page.fill('input[name="code"]', code);
  await page.click('button[type="submit"]');

  // Should land on the dashboard
  await expect(page).toHaveURL(/\/admin\/dashboard/);
}

/**
 * Generate a TOTP code from a secret.
 * Placeholder — in a real implementation, use the 'otpauth' npm package.
 */
function generateTOTPCode(_secret: string): string {
  // TODO: Implement using otpauth library
  // import { TOTP } from 'otpauth';
  // const totp = new TOTP({ secret: OTPAuth.Secret.fromBase32(secret) });
  // return totp.generate();
  throw new Error('TOTP code generation not yet implemented — install otpauth package');
}

/**
 * Check that the page shows the admin dashboard.
 */
export async function expectDashboard(page: Page) {
  await expect(page.locator('h2')).toContainText('Dashboard');
}

/**
 * Logout from the admin panel.
 */
export async function logout(page: Page) {
  await page.click('button:has-text("Logout")');
  await expect(page).toHaveURL(/\/admin\/login/);
}
