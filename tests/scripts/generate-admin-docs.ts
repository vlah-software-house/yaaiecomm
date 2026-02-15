/**
 * ForgeCommerce — Comprehensive Admin Documentation Generator
 *
 * Navigates through the ENTIRE admin panel, taking screenshots at each step,
 * and generates a self-contained HTML documentation file with Base64-inlined images.
 *
 * Usage:
 *   cd tests && npx tsx scripts/generate-admin-docs.ts
 *
 * Prerequisites:
 *   - Admin server running on ADMIN_URL (default: http://localhost:8081)
 *   - Database seeded with both seed.sql AND seed-docs.sql
 *   - Admin user: admin@forgecommerce.local / admin123
 *
 * Output:
 *   docs/admin-guide.html (~15-20 MB, self-contained)
 */

import { chromium, Page, Browser } from 'playwright';
import * as fs from 'fs';
import * as path from 'path';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';
const ADMIN_EMAIL = process.env.ADMIN_EMAIL || 'admin@forgecommerce.local';
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || 'admin123';
const OUTPUT_DIR = path.resolve(__dirname, '../../docs');
const OUTPUT_FILE = path.join(OUTPUT_DIR, 'admin-guide.html');

// ─── Types ────────────────────────────────────────────────────────────

interface DocStep {
  title: string;
  description: string;
  screenshot: string; // Base64 PNG
  tips?: string[];
  warning?: string; // EU compliance or important notes
}

interface DocSection {
  id: string;
  title: string;
  intro: string;
  steps: DocStep[];
  failed?: boolean;
  errorMsg?: string;
}

// ─── Helper Functions ─────────────────────────────────────────────────

async function screenshot(page: Page, label: string): Promise<string> {
  await page.waitForTimeout(500);
  const buffer = await page.screenshot({ fullPage: false, type: 'png' });
  console.log(`  📸 ${label}`);
  return buffer.toString('base64');
}

async function screenshotFull(page: Page, label: string): Promise<string> {
  await page.waitForTimeout(500);
  const buffer = await page.screenshot({ fullPage: true, type: 'png' });
  console.log(`  📸 ${label} (full page)`);
  return buffer.toString('base64');
}

async function adminLogin(page: Page): Promise<void> {
  console.log('🔐 Logging into admin panel...');
  await page.goto(ADMIN_URL + '/admin/login');

  await page.fill('input[name="email"]', ADMIN_EMAIL);
  await page.fill('input[name="password"]', ADMIN_PASSWORD);

  const [response] = await Promise.all([
    page.waitForResponse(r => r.url().includes('/admin/login') && r.request().method() === 'POST'),
    page.locator('form[action*="login"] button[type="submit"], form button[type="submit"]').first().click(),
  ]);

  console.log(`   → Login response: ${response.status()}`);
  await page.waitForTimeout(2000);

  // Handle 2FA setup redirect
  if (page.url().includes('setup-2fa')) {
    await page.goto(ADMIN_URL + '/admin/dashboard');
    await page.waitForTimeout(1000);
  }

  console.log(`   → Landed on: ${page.url()}`);
  console.log('✅ Logged in successfully');
}

/** Wrap a capture function in try/catch for resilience */
async function captureWithFallback(
  page: Page,
  captureFn: (page: Page) => Promise<DocSection>,
  fallbackId: string,
  fallbackTitle: string,
  fallbackIntro: string,
): Promise<DocSection> {
  try {
    return await captureFn(page);
  } catch (err) {
    console.error(`  ❌ ERROR in "${fallbackTitle}":`, err);
    // Try to take a screenshot of whatever state we're in
    let errorScreenshot = '';
    try {
      errorScreenshot = await screenshot(page, 'Error state');
    } catch { /* ignore */ }
    return {
      id: fallbackId,
      title: fallbackTitle,
      intro: fallbackIntro,
      steps: errorScreenshot ? [{
        title: 'Section Generation Failed',
        description: `This section could not be fully generated. Error: ${String(err)}`,
        screenshot: errorScreenshot,
      }] : [],
      failed: true,
      errorMsg: String(err),
    };
  }
}

// ─── Section 1: Logging In ────────────────────────────────────────────

async function captureLogin(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 1: Logging In');
  const steps: DocStep[] = [];

  // Start fresh at the login page
  await page.goto(ADMIN_URL + '/admin/login');
  await page.waitForSelector('input[name="email"]');

  steps.push({
    title: 'Admin Login Page',
    description:
      'Navigate to your admin panel URL to see the login page. ' +
      'Enter the email and password for your administrator account.',
    screenshot: await screenshot(page, 'Login page'),
    tips: [
      'Bookmark your admin URL for quick access.',
      'Admin accounts are separate from customer accounts.',
      'Contact your system administrator if you\'ve forgotten your credentials.',
    ],
  });

  // Fill credentials
  await page.fill('input[name="email"]', ADMIN_EMAIL);
  await page.fill('input[name="password"]', ADMIN_PASSWORD);

  steps.push({
    title: 'Enter Your Credentials',
    description:
      'Type your admin email address and password, then click "Sign In". ' +
      'If two-factor authentication (2FA) is enabled, you\'ll be prompted for a verification code next.',
    screenshot: await screenshot(page, 'Filled login form'),
  });

  // Submit (we're already logged in from adminLogin, so just navigate to dashboard)
  await page.goto(ADMIN_URL + '/admin/dashboard');
  await page.waitForSelector('h2');
  await page.waitForTimeout(1000);

  steps.push({
    title: 'Welcome to Your Dashboard',
    description:
      'After successful login, you land on the Dashboard — your command center. ' +
      'The sidebar on the left provides quick access to every section of your store administration.',
    screenshot: await screenshot(page, 'Dashboard after login'),
    tips: [
      'Your session lasts 8 hours. After that, you\'ll need to log in again.',
      'Two-factor authentication (2FA) adds an extra layer of security and is recommended for all admin users.',
    ],
  });

  return {
    id: 'login',
    title: '1. Logging In',
    intro:
      'Access your store\'s admin panel by navigating to your admin URL. ' +
      'All administrative functions — from managing products to processing orders — start here.',
    steps,
  };
}

// ─── Section 2: Dashboard ─────────────────────────────────────────────

async function captureDashboard(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 2: Your Dashboard');
  const steps: DocStep[] = [];

  await page.goto(ADMIN_URL + '/admin/dashboard');
  await page.waitForSelector('h2');
  // Wait for HTMX stat cards to load
  await page.waitForTimeout(2500);

  steps.push({
    title: 'Dashboard Overview',
    description:
      'The dashboard shows four key metrics at a glance: Orders Today, Revenue This Month, ' +
      'Low Stock Alerts, and Pending Orders. These stats load automatically and update in real-time.',
    screenshot: await screenshot(page, 'Dashboard stats'),
    tips: [
      'Stats update automatically via HTMX — no need to refresh the page.',
      'Click on any stat card to navigate to the relevant section.',
      'Low Stock alerts help you reorder raw materials before running out.',
    ],
  });

  // Scroll to recent orders
  const recentOrders = page.locator('text=Recent Orders').first();
  if (await recentOrders.isVisible().catch(() => false)) {
    await recentOrders.scrollIntoViewIfNeeded();
    await page.waitForTimeout(1500);
  }

  steps.push({
    title: 'Recent Orders & Quick Actions',
    description:
      'Below the stats, you\'ll find your most recent orders with their status, customer email, and total. ' +
      'Click any order number to view its full details.',
    screenshot: await screenshot(page, 'Recent orders'),
  });

  // Show sidebar navigation
  await page.goto(ADMIN_URL + '/admin/dashboard');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Sidebar Navigation',
    description:
      'The left sidebar is your main navigation. It\'s organized by workflow: ' +
      'Products & Categories for your catalog, Orders for fulfillment, Reports for analytics, ' +
      'and Settings for store configuration. The divider separates daily operations from setup tasks.',
    screenshot: await screenshot(page, 'Sidebar navigation'),
    tips: [
      'The sidebar is always visible — you can jump to any section from anywhere.',
      'Settings sections (VAT, Shipping, Users) are below the divider for less frequent access.',
    ],
  });

  return {
    id: 'dashboard',
    title: '2. Your Dashboard',
    intro:
      'The Dashboard is your daily starting point. It provides a quick overview of your store\'s ' +
      'health: today\'s orders, monthly revenue, stock alerts, and pending work. ' +
      'Think of it as your store\'s heartbeat monitor.',
    steps,
  };
}

// ─── Section 3: Store Setup — VAT ─────────────────────────────────────

async function captureVATSettings(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 3: Store Setup — VAT');
  const steps: DocStep[] = [];

  await page.goto(ADMIN_URL + '/admin/settings/vat');
  await page.waitForSelector('h2');
  await page.waitForTimeout(1000);

  steps.push({
    title: 'VAT Configuration',
    description:
      'The VAT Settings page is where you configure your store\'s tax compliance. ' +
      'Enable VAT, enter your VAT registration number, select your home country, ' +
      'and choose whether your product prices include VAT or are displayed as net prices.',
    screenshot: await screenshot(page, 'VAT settings top'),
    tips: [
      'If you\'re not VAT-registered, leave VAT disabled — all prices will be treated as net.',
      'The "Prices Include VAT" setting affects how prices are calculated at checkout.',
      'Enable "B2B Reverse Charge" to allow businesses in other EU countries to purchase without VAT.',
    ],
    warning:
      'EU VAT compliance is mandatory for VAT-registered businesses. ' +
      'Consult your tax advisor to ensure correct configuration.',
  });

  // Scroll to selling countries
  const countriesSection = page.locator('text=Selling Countries').first();
  if (await countriesSection.isVisible().catch(() => false)) {
    await countriesSection.scrollIntoViewIfNeeded();
    await page.waitForTimeout(500);
  }

  steps.push({
    title: 'Selling Countries',
    description:
      'Select which EU countries your store ships to. Only enabled countries appear in the ' +
      'customer checkout, and VAT is calculated based on the destination country. ' +
      'Use "Select All" / "Deselect All" for bulk changes.',
    screenshot: await screenshot(page, 'Selling countries grid'),
    tips: [
      'Your home country is always highlighted in the list.',
      'Only enable countries where you can legally fulfill orders.',
      'Shipping zones (configured separately) can group these countries for rate calculation.',
    ],
  });

  // Scroll to VAT rates table
  const ratesSection = page.locator('text=Current VAT Rates').first();
  if (await ratesSection.isVisible().catch(() => false)) {
    await ratesSection.scrollIntoViewIfNeeded();
    await page.waitForTimeout(500);
  }

  steps.push({
    title: 'Current VAT Rates',
    description:
      'This table shows the current VAT rates for each enabled country, organized by rate type. ' +
      'Rates are synced automatically from the European Commission TEDB service every midnight (UTC). ' +
      'You can also trigger a manual sync with the "Sync Now" button.',
    screenshot: await screenshot(page, 'VAT rates table'),
    tips: [
      'Rates are sourced from the official EU TEDB service and updated daily.',
      'If TEDB is unavailable, rates fall back to the euvatrates.com community database.',
      'Manual edits are overwritten on the next sync unless the rate source is set to "manual".',
    ],
  });

  // Take a full-page shot for the complete picture
  await page.goto(ADMIN_URL + '/admin/settings/vat');
  await page.waitForTimeout(1000);

  steps.push({
    title: 'Complete VAT Settings Page',
    description:
      'Here\'s the full VAT settings page showing all three sections: ' +
      'configuration, country selection, and rate management.',
    screenshot: await screenshotFull(page, 'Full VAT settings page'),
  });

  return {
    id: 'vat-settings',
    title: '3. Store Setup: VAT Configuration',
    intro:
      'ForgeCommerce is built EU-first with comprehensive VAT support. Configure your VAT registration, ' +
      'select which countries you sell to, and let the system automatically calculate the correct VAT rate ' +
      'for each order based on the destination country and product category.',
    steps,
  };
}

// ─── Section 4: Store Setup — Shipping ────────────────────────────────

async function captureShippingSettings(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 4: Store Setup — Shipping');
  const steps: DocStep[] = [];

  await page.goto(ADMIN_URL + '/admin/settings/shipping');
  await page.waitForSelector('h2');
  await page.waitForTimeout(1000);

  steps.push({
    title: 'Shipping Configuration',
    description:
      'Configure your global shipping settings: enable/disable shipping, choose a calculation method ' +
      '(fixed fee, weight-based, or size-based), and set a free shipping threshold. ' +
      'The default currency is used for all shipping calculations.',
    screenshot: await screenshot(page, 'Shipping config'),
    tips: [
      'Fixed: Every order pays the same shipping fee.',
      'Weight-based: Shipping cost varies by total order weight (configurable brackets).',
      'Free shipping threshold: Orders above this amount ship for free — a great incentive!',
    ],
  });

  // Scroll to shipping zones
  const zonesSection = page.locator('text=Shipping Zones').first();
  if (await zonesSection.isVisible().catch(() => false)) {
    await zonesSection.scrollIntoViewIfNeeded();
    await page.waitForTimeout(500);
  }

  steps.push({
    title: 'Shipping Zones',
    description:
      'Shipping zones let you group countries with similar shipping rates. ' +
      'For example, "Iberian Peninsula" (Spain + Portugal) might have lower rates ' +
      'than "Central Europe" (Germany, France, Belgium). Each zone overrides the global rate for its countries.',
    screenshot: await screenshot(page, 'Shipping zones'),
    tips: [
      'Countries not in any zone use the global shipping rate.',
      'You can have as many zones as you need.',
      'Zone rates use the same calculation method options as the global config.',
    ],
  });

  steps.push({
    title: 'Full Shipping Settings',
    description:
      'The complete shipping settings page with global configuration at the top ' +
      'and zone management below.',
    screenshot: await screenshotFull(page, 'Full shipping settings'),
  });

  return {
    id: 'shipping-settings',
    title: '4. Store Setup: Shipping',
    intro:
      'Set up shipping rates that work for your business. ForgeCommerce supports flat-rate, ' +
      'weight-based, and size-based shipping with optional zones for per-country rate overrides. ' +
      'Combine with a free shipping threshold to boost your average order value.',
    steps,
  };
}

// ─── Section 5: Categories ────────────────────────────────────────────

async function captureCategories(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 5: Managing Categories');
  const steps: DocStep[] = [];

  await page.goto(ADMIN_URL + '/admin/categories');
  await page.waitForSelector('h2');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Category List',
    description:
      'The Categories page shows your product category hierarchy. Categories help organize your catalog ' +
      'and improve navigation for customers. Each category has a name, slug (URL path), and optional parent.',
    screenshot: await screenshot(page, 'Category list'),
    tips: [
      'Categories can be nested — a "Leather" sub-category under "Bags", for example.',
      'The slug is used in storefront URLs: /products/bags/leather.',
      'Drag categories to reorder them (position determines display order).',
    ],
  });

  // Navigate to create form
  await page.goto(ADMIN_URL + '/admin/categories/new');
  await page.waitForSelector('h2');

  steps.push({
    title: 'Creating a Category',
    description:
      'To create a category, provide a name and optional description. ' +
      'Choose a parent category for nesting, and add SEO fields for search engine optimization.',
    screenshot: await screenshot(page, 'New category form'),
  });

  // Click on existing category to show edit
  await page.goto(ADMIN_URL + '/admin/categories');
  await page.waitForTimeout(500);
  const categoryLink = page.locator('table tbody td a[href*="/admin/categories/"]:not([href$="/new"])').first();
  if (await categoryLink.isVisible().catch(() => false)) {
    await categoryLink.click();
    await page.waitForSelector('h2');

    steps.push({
      title: 'Editing a Category',
      description:
        'Click any category name to edit it. You can change the name, description, parent category, ' +
        'and SEO settings. The slug updates automatically if you change the name.',
      screenshot: await screenshot(page, 'Edit category'),
    });
  }

  return {
    id: 'categories',
    title: '5. Managing Categories',
    intro:
      'Categories organize your products into a logical hierarchy that customers can browse. ' +
      'A well-organized category tree makes it easy for shoppers to find what they\'re looking for.',
    steps,
  };
}

// ─── Section 6: Raw Materials & Inventory ─────────────────────────────

async function captureRawMaterials(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 6: Raw Materials & Inventory');
  const steps: DocStep[] = [];

  await page.goto(ADMIN_URL + '/admin/inventory/raw-materials');
  await page.waitForSelector('h2');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Raw Materials List',
    description:
      'The Raw Materials page shows your inventory of components and supplies. ' +
      'Each material has a SKU, unit of measure, current stock level, and supplier info. ' +
      'Materials with stock below the low-stock threshold are highlighted.',
    screenshot: await screenshot(page, 'Raw materials list'),
    tips: [
      'Low stock items are flagged so you can reorder before running out.',
      'Use categories to organize materials (Leather, Hardware, Thread, etc.).',
      'Lead time helps you plan reorders with enough buffer.',
    ],
  });

  // Click first material to show detail
  const materialLink = page.locator('table tbody td a[href*="/admin/inventory/raw-materials/"]:not([href$="/new"])').first();
  if (await materialLink.isVisible().catch(() => false)) {
    await materialLink.click();
    await page.waitForSelector('h2');

    steps.push({
      title: 'Material Details',
      description:
        'Each raw material has detailed information: cost per unit, stock quantity, supplier details, ' +
        'and lead time. This data feeds into the Bill of Materials (BOM) calculations for your products.',
      screenshot: await screenshotFull(page, 'Material detail'),
    });
  }

  // Show the create form
  await page.goto(ADMIN_URL + '/admin/inventory/raw-materials/new');
  await page.waitForSelector('h2');

  steps.push({
    title: 'Adding a New Material',
    description:
      'When you source a new material, add it here with its SKU, cost, unit of measure, ' +
      'initial stock quantity, and supplier information. This material can then be used in product BOMs.',
    screenshot: await screenshot(page, 'New material form'),
    tips: [
      'Choose the correct Unit of Measure — it affects BOM calculations.',
      'Set a meaningful Low Stock Threshold based on your production rate.',
      'Supplier SKU helps with reordering from the same supplier.',
    ],
  });

  // Go back to list
  await page.goto(ADMIN_URL + '/admin/inventory/raw-materials');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Inventory at a Glance',
    description:
      'The list page gives you a complete overview of your inventory levels. ' +
      'Filter or search to find specific materials quickly.',
    screenshot: await screenshot(page, 'Materials overview'),
  });

  return {
    id: 'raw-materials',
    title: '6. Raw Materials & Inventory',
    intro:
      'ForgeCommerce is designed for manufacturers who track raw materials. ' +
      'Manage your inventory of leather, hardware, thread, and other components. ' +
      'These materials connect to products through the Bill of Materials (BOM) system.',
    steps,
  };
}

// ─── Section 7: Products ──────────────────────────────────────────────

async function captureProducts(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 7: Creating & Managing Products');
  const steps: DocStep[] = [];

  // --- 7a: Product List ---
  await page.goto(ADMIN_URL + '/admin/products?status=active');
  await page.waitForSelector('h2');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Product List',
    description:
      'The Products page shows all your products with their status, SKU prefix, price, and stock. ' +
      'Use the status filter to view Active, Draft, or Archived products.',
    screenshot: await screenshot(page, 'Product list'),
    tips: [
      'Draft products are not visible in the storefront.',
      'Archive products you no longer sell instead of deleting them.',
      'Click a product name to open its full editor with all tabs.',
    ],
  });

  // --- 7b: Product Details Tab ---
  const productLink = page.locator('table tbody td a[href*="/admin/products/"]:not([href$="/new"])').first();
  let productHref = '';
  if (await productLink.isVisible().catch(() => false)) {
    productHref = await productLink.getAttribute('href') || '';
    await productLink.click();
    await page.waitForSelector('h2');

    steps.push({
      title: 'Product Details Tab',
      description:
        'The Details tab is where you set the product\'s basic information: name, description, pricing, ' +
        'weight, and SEO fields. Notice the tab bar at the top — each tab manages a different aspect.',
      screenshot: await screenshotFull(page, 'Product details tab'),
      tips: [
        'Base Price is your starting price. Variant options can add/subtract from this.',
        'Compare-at Price shows a "was €X" crossed-out price in the storefront.',
        'SKU Prefix is used when auto-generating variant SKUs.',
      ],
    });

    // --- 7c: Attributes Tab ---
    if (productHref) {
      await page.goto(ADMIN_URL + productHref + '/attributes');
      await page.waitForSelector('h2');
      await page.waitForTimeout(500);

      steps.push({
        title: 'Product Attributes Tab',
        description:
          'Attributes define the axes of variation for your product — Color, Size, Material, etc. ' +
          'Each attribute has a type that controls how it\'s displayed in the storefront ' +
          '(dropdown, color swatch, buttons, or image swatch).',
        screenshot: await screenshotFull(page, 'Product attributes'),
        tips: [
          'Attributes create the variation grid — 3 colors × 2 sizes = 6 variants.',
          'Color Swatch type shows visual color circles on the product page.',
          'Button Group type shows horizontal button-style selectors (ideal for sizes).',
          'Price Modifier on an option adds to/subtracts from the base price.',
        ],
      });

      // --- 7d: Variants Tab ---
      await page.goto(ADMIN_URL + productHref + '/variants');
      await page.waitForSelector('h2');
      await page.waitForTimeout(500);

      steps.push({
        title: 'Product Variants Tab',
        description:
          'Variants are the purchasable combinations of your attributes. A product with 3 colors and ' +
          '2 sizes generates 6 variants, each with its own SKU, stock level, and optional price override. ' +
          'Click "Generate Variants" to create all combinations automatically.',
        screenshot: await screenshotFull(page, 'Product variants'),
        tips: [
          'Auto-generated SKUs follow the pattern: {prefix}-{option1}-{option2}.',
          'Leave price empty to use the calculated price (base + option modifiers).',
          'Each variant tracks its own stock independently.',
          'Inactive variants are hidden from the storefront.',
        ],
      });

      // --- 7e: BOM Tab ---
      await page.goto(ADMIN_URL + productHref + '/bom');
      await page.waitForSelector('h2');
      await page.waitForTimeout(500);

      steps.push({
        title: 'Bill of Materials (BOM) Tab',
        description:
          'The BOM defines which raw materials are needed to produce this product. ' +
          'Layer 1 shows base materials needed for ALL variants. Layer 3 allows ' +
          'per-variant overrides (e.g., Brown/Large needs more leather than Black/Standard).',
        screenshot: await screenshotFull(page, 'Product BOM'),
        tips: [
          'Layer 1: Base materials needed for every variant.',
          'Layer 3: Per-variant overrides (replace, add, remove, or set quantity).',
          'Producibility: How many units you can produce with current stock.',
          'BOM cost = sum of (material cost × quantity) — useful for pricing decisions.',
        ],
        warning:
          'Keep your BOM up to date! Inaccurate BOMs lead to stock discrepancies ' +
          'and missed production deadlines.',
      });

      // --- 7f: Images Tab ---
      await page.goto(ADMIN_URL + productHref + '/images');
      await page.waitForSelector('h2');
      await page.waitForTimeout(500);

      steps.push({
        title: 'Product Images Tab',
        description:
          'Upload product images here. Supported formats: JPEG, PNG, WebP, GIF (max 10 MB each). ' +
          'Set a primary image for the catalog thumbnail, and optionally assign images to specific variants.',
        screenshot: await screenshot(page, 'Product images'),
        tips: [
          'The primary image is shown in product listings and search results.',
          'Assign images to variants so the right image shows when a customer selects a color.',
          'Alt text improves SEO and accessibility — describe what\'s in the image.',
          'Drag to reorder images. The order determines gallery display.',
        ],
      });

      // --- 7g: VAT Tab ---
      await page.goto(ADMIN_URL + productHref + '/vat');
      await page.waitForSelector('h2');
      await page.waitForTimeout(500);

      steps.push({
        title: 'Product VAT Tab',
        description:
          'Override the default VAT category for this specific product, and add per-country overrides. ' +
          'For example, food products may qualify for a reduced rate in some countries but standard rate in others.',
        screenshot: await screenshotFull(page, 'Product VAT tab'),
        tips: [
          'Most products use the store default (Standard Rate). Only override when needed.',
          'Per-country overrides handle EU quirks — e.g., children\'s clothing is zero-rated in some countries.',
          'Notes field: document WHY an override exists for audit purposes.',
        ],
        warning:
          'VAT category misclassification can result in tax penalties. ' +
          'Verify product classifications with your tax advisor.',
      });

      // --- 7h: Global Attributes Tab ---
      await page.goto(ADMIN_URL + productHref + '/global-attributes');
      await page.waitForSelector('h2');
      await page.waitForTimeout(500);

      steps.push({
        title: 'Global Attributes Tab',
        description:
          'Link reusable global attribute templates to this product. Unlike product-specific attributes, ' +
          'global attributes are defined once and shared across products — change an option globally, and ' +
          'every linked product gets the update.',
        screenshot: await screenshot(page, 'Product global attributes'),
        tips: [
          'Global attributes are ideal for standardized options like Color, Size, Material.',
          'Set the role: Variant Axis (generates variants), Filter Only, or Display Only.',
          'After linking, select which specific options this product offers.',
          'See Section 8 for creating and managing Global Attribute templates.',
        ],
      });
    }
  }

  // --- New product form ---
  await page.goto(ADMIN_URL + '/admin/products/new');
  await page.waitForSelector('h2');

  steps.push({
    title: 'Creating a New Product',
    description:
      'Click "+ New Product" to start creating. Fill in the name, price, and description ' +
      'on the Details tab. Once saved, the other tabs (Attributes, Variants, BOM, Images, VAT) become available.',
    screenshot: await screenshotFull(page, 'New product form'),
  });

  return {
    id: 'products',
    title: '7. Creating & Managing Products',
    intro:
      'Products are the heart of your store. ForgeCommerce gives you powerful tools for managing ' +
      'complex products with multiple attributes (Color, Size, Material), automatically generated variants, ' +
      'Bills of Materials for manufacturers, image galleries, and per-country VAT configuration. ' +
      'Each product has 7 tabs covering every aspect of its configuration.',
    steps,
  };
}

// ─── Section 8: Global Attributes ─────────────────────────────────────

async function captureGlobalAttributes(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 8: Global Attribute Templates');
  const steps: DocStep[] = [];

  await page.goto(ADMIN_URL + '/admin/global-attributes');
  await page.waitForSelector('h2');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Global Attributes List',
    description:
      'Global Attributes are reusable attribute templates. Instead of defining "Color" separately on ' +
      'every product, create it once here with all options, then link it to any product that needs it.',
    screenshot: await screenshot(page, 'Global attributes list'),
    tips: [
      'The "Used By" column shows how many products link this attribute.',
      'Types: Select, Color Swatch, Button Group, Image Swatch.',
      'Categories help organize attributes: Style, Physical, Material, etc.',
    ],
  });

  // Click on first attribute to show edit page
  const attrLink = page.locator('table tbody td a[href*="/admin/global-attributes/"]:not([href$="/new"])').first();
  if (await attrLink.isVisible().catch(() => false)) {
    await attrLink.click();
    await page.waitForSelector('h2');
    await page.waitForTimeout(500);

    steps.push({
      title: 'Editing a Global Attribute',
      description:
        'The edit page has three sections: basic settings at the top, Metadata Fields in the middle ' +
        '(for structured data like Pantone codes), and Options at the bottom (the actual values).',
      screenshot: await screenshotFull(page, 'Global attribute edit'),
    });
  }

  // Show creation form
  await page.goto(ADMIN_URL + '/admin/global-attributes/new');
  await page.waitForSelector('h2');

  steps.push({
    title: 'Creating a Global Attribute',
    description:
      'Define the attribute name (internal, snake_case), display name, type, and category. ' +
      'After creation, you can add metadata fields and options.',
    screenshot: await screenshot(page, 'New global attribute form'),
    tips: [
      'For detailed step-by-step guidance with screenshots of metadata fields and options, ' +
      'see the dedicated Global Attributes Guide (global-attributes-guide.html).',
    ],
  });

  return {
    id: 'global-attributes',
    title: '8. Global Attribute Templates',
    intro:
      'Global Attributes let you define reusable attribute templates — create "Color" once with ' +
      '20 options, then link it to 50 products. Each product can select which subset of options it offers. ' +
      'Changes to the template automatically propagate to all linked products.',
    steps,
  };
}

// ─── Section 9: Orders ────────────────────────────────────────────────

async function captureOrders(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 9: Processing Orders');
  const steps: DocStep[] = [];

  await page.goto(ADMIN_URL + '/admin/orders');
  await page.waitForSelector('h2');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Orders List',
    description:
      'The Orders page shows all orders with their number, status, customer email, total, and date. ' +
      'Orders flow through a lifecycle: Pending → Confirmed → Processing → Shipped → Delivered.',
    screenshot: await screenshot(page, 'Orders list'),
    tips: [
      'New orders from Stripe checkout arrive as "Pending" with payment "Paid".',
      'Click any order number to view full details and take actions.',
    ],
  });

  // Status filters
  steps.push({
    title: 'Status Filters',
    description:
      'Use the status filter buttons to quickly find orders in a specific state. ' +
      'This helps you focus on what needs attention — "Pending" orders need confirmation, ' +
      '"Processing" orders need to be packed and shipped.',
    screenshot: await screenshot(page, 'Order status filters'),
  });

  // Click on first order for detail
  const orderLink = page.locator('table tbody td a[href*="/admin/orders/"]:not([href$="/new"])').first();
  if (await orderLink.isVisible().catch(() => false)) {
    await orderLink.click();
    await page.waitForSelector('h2');
    await page.waitForTimeout(500);

    steps.push({
      title: 'Order Detail — Items & Totals',
      description:
        'The order detail shows the complete breakdown: ordered items with quantities and prices, ' +
        'shipping fees, VAT calculation, discount applied (if any), and the final total. ' +
        'All VAT information is snapshotted at the time of order — it won\'t change if rates update.',
      screenshot: await screenshotFull(page, 'Order detail'),
      tips: [
        'VAT rates are locked at order time — even if rates change, existing orders keep their original VAT.',
        'The "VAT Breakdown" section shows exactly how much tax was charged and at which rate.',
      ],
    });

    steps.push({
      title: 'Order Actions',
      description:
        'On the right side, you\'ll find status transition buttons. Move the order through its lifecycle: ' +
        'Confirm → Process → Ship (add tracking number) → Mark Delivered. ' +
        'You can also cancel an order at most stages.',
      screenshot: await screenshot(page, 'Order actions'),
      warning:
        'Cancelled orders cannot be un-cancelled. Use this action carefully.',
    });
  }

  // Look for a B2B reverse charge order
  await page.goto(ADMIN_URL + '/admin/orders');
  await page.waitForTimeout(500);
  const b2bOrder = page.locator('table tbody tr:has-text("Beispiel"), table tbody tr:has-text("DE123")').first();
  if (await b2bOrder.isVisible().catch(() => false)) {
    const b2bLink = b2bOrder.locator('a[href*="/admin/orders/"]').first();
    if (await b2bLink.isVisible().catch(() => false)) {
      await b2bLink.click();
      await page.waitForSelector('h2');
      await page.waitForTimeout(500);

      steps.push({
        title: 'B2B Order — Reverse Charge',
        description:
          'When a business customer provides a valid EU VAT number, the reverse charge mechanism applies: ' +
          'VAT is 0%, and the buyer is responsible for reporting VAT in their own country. ' +
          'The order shows the customer\'s VAT number and company name (validated via VIES).',
        screenshot: await screenshotFull(page, 'B2B reverse charge order'),
        warning:
          'The reverse charge mechanism only applies to intra-EU B2B transactions. ' +
          'Domestic B2B sales (same country as your store) still charge normal VAT.',
      });
    }
  }

  return {
    id: 'orders',
    title: '9. Processing Orders',
    intro:
      'Orders are the result of successful checkout. ForgeCommerce handles the full order lifecycle ' +
      'from pending through delivery, with built-in support for B2B reverse charge orders, ' +
      'VAT snapshots, and tracking number management.',
    steps,
  };
}

// ─── Section 10: Discounts & Coupons ──────────────────────────────────

async function captureDiscounts(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 10: Discounts & Coupons');
  const steps: DocStep[] = [];

  await page.goto(ADMIN_URL + '/admin/discounts');
  await page.waitForSelector('h2');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Discounts List',
    description:
      'Discounts define promotional rules: percentage off, fixed amount, applied to subtotal or shipping. ' +
      'Each discount has date ranges, minimum purchase amounts, and priority for stacking rules.',
    screenshot: await screenshot(page, 'Discounts list'),
    tips: [
      'Active discounts with valid dates are automatically applied at checkout.',
      'Use priority to control which discount takes precedence when multiple apply.',
      'Non-stackable discounts stop evaluation after they\'re applied.',
    ],
  });

  // Click on a discount to show edit
  const discountLink = page.locator('table tbody td a[href*="/admin/discounts/"]:not([href$="/new"])').first();
  if (await discountLink.isVisible().catch(() => false)) {
    await discountLink.click();
    await page.waitForSelector('h2');

    steps.push({
      title: 'Editing a Discount',
      description:
        'Configure the discount type (percentage or fixed), value, scope (subtotal, shipping, or total), ' +
        'minimum order amount, maximum discount cap, date range, and whether it stacks with other discounts.',
      screenshot: await screenshotFull(page, 'Discount edit'),
    });
  } else {
    // If no discounts exist, show the creation form
    await page.goto(ADMIN_URL + '/admin/discounts/new');
    await page.waitForSelector('h2');

    steps.push({
      title: 'Creating a Discount',
      description:
        'Set up your promotion: choose percentage or fixed amount, set the scope, and configure ' +
        'date ranges and conditions.',
      screenshot: await screenshotFull(page, 'New discount form'),
    });
  }

  // Coupons page
  await page.goto(ADMIN_URL + '/admin/coupons');
  await page.waitForSelector('h2');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Coupon Codes',
    description:
      'Coupons are codes that customers enter at checkout to activate a discount. ' +
      'Each coupon is linked to a discount rule and can have usage limits (total and per-customer).',
    screenshot: await screenshot(page, 'Coupons list'),
    tips: [
      'Coupon codes are case-insensitive (SUMMER15 = summer15).',
      'Usage count tracks how many times the coupon has been used.',
      'Per-customer limits prevent one customer from using the same code multiple times.',
    ],
  });

  return {
    id: 'discounts',
    title: '10. Discounts & Coupons',
    intro:
      'Run promotions with flexible discounts (percentage or fixed) and coupon codes. ' +
      'Discounts can target the subtotal, shipping, or total, with optional conditions ' +
      'like minimum purchase and date ranges.',
    steps,
  };
}

// ─── Section 11: Production Batches ───────────────────────────────────

async function captureProduction(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 11: Production Batches');
  const steps: DocStep[] = [];

  await page.goto(ADMIN_URL + '/admin/production');
  await page.waitForSelector('h2');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Production Batches List',
    description:
      'Track your manufacturing with production batches. Each batch produces a quantity of a specific product variant, ' +
      'consuming raw materials according to the BOM. The lifecycle goes: Planned → In Progress → Completed.',
    screenshot: await screenshot(page, 'Production batches list'),
    tips: [
      'Starting a batch consumes raw materials from inventory.',
      'Completing a batch adds finished product to variant stock.',
      'Use batches to plan production runs based on order demand.',
    ],
  });

  // Show create form
  await page.goto(ADMIN_URL + '/admin/production/new');
  await page.waitForSelector('h2');

  steps.push({
    title: 'Creating a Production Batch',
    description:
      'Select the product and variant to produce, set the quantity, and add optional notes. ' +
      'The system will calculate required materials based on the BOM.',
    screenshot: await screenshot(page, 'New production batch form'),
  });

  // Check if there's a batch to view
  await page.goto(ADMIN_URL + '/admin/production');
  await page.waitForTimeout(500);
  const batchLink = page.locator('table tbody td a[href*="/admin/production/"]:not([href$="/new"])').first();
  if (await batchLink.isVisible().catch(() => false)) {
    await batchLink.click();
    await page.waitForSelector('h2');

    steps.push({
      title: 'Batch Detail & Status Actions',
      description:
        'View batch details including the product, quantity, materials to be consumed, and current status. ' +
        'Use the action buttons to move the batch through its lifecycle.',
      screenshot: await screenshotFull(page, 'Batch detail'),
    });
  }

  return {
    id: 'production',
    title: '11. Production Batches',
    intro:
      'For manufacturers, production batches bridge the gap between raw materials and finished products. ' +
      'Plan batches, track material consumption, and update finished goods inventory — all in one place.',
    steps,
  };
}

// ─── Section 12: Reports ──────────────────────────────────────────────

async function captureReports(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 12: Reports & Tax Filing');
  const steps: DocStep[] = [];

  // Sales Report — uses h1 instead of h2
  await page.goto(ADMIN_URL + '/admin/reports/sales');
  await page.waitForSelector('h1');
  // Wait for HTMX-loaded report data
  await page.waitForTimeout(3000);

  steps.push({
    title: 'Sales Report',
    description:
      'The Sales Report shows your revenue over a selected period. View daily breakdowns, ' +
      'cumulative totals, order counts, and average order value. Use the date picker to change the period.',
    screenshot: await screenshotFull(page, 'Sales report'),
    tips: [
      'The default view shows the current month.',
      'Export to CSV for use in spreadsheets or accounting software.',
      'Compare periods to spot trends — is this month better than last?',
    ],
  });

  // VAT Report — uses h1 instead of h2
  await page.goto(ADMIN_URL + '/admin/reports/vat');
  await page.waitForSelector('h1');
  await page.waitForTimeout(3000);

  steps.push({
    title: 'VAT Report',
    description:
      'The VAT Report breaks down tax collected by country and rate type — exactly what you need ' +
      'for EU tax filing. It shows total VAT collected per country, split by rate type (standard, reduced, etc.).',
    screenshot: await screenshotFull(page, 'VAT report'),
    tips: [
      'Use quarterly periods for VAT return filing.',
      'The reverse charge section shows B2B transactions where no VAT was collected.',
      'Export to CSV for your accountant or tax software.',
    ],
    warning:
      'The VAT Report is a tool to assist your tax filing — it does not constitute tax advice. ' +
      'Always verify figures with your accountant.',
  });

  // Show export functionality
  steps.push({
    title: 'Exporting Reports',
    description:
      'Both reports offer CSV export for integration with your accounting tools. ' +
      'The CSV includes all the detail shown on screen plus additional fields for programmatic processing.',
    screenshot: await screenshot(page, 'Report export area'),
  });

  return {
    id: 'reports',
    title: '12. Reports & Tax Filing',
    intro:
      'Two essential reports for running your EU business: the Sales Report for tracking revenue, ' +
      'and the VAT Report for tax compliance. Both support date range selection and CSV export.',
    steps,
  };
}

// ─── Section 13: Import & Export ──────────────────────────────────────

async function captureImportExport(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 13: Import & Export');
  const steps: DocStep[] = [];

  await page.goto(ADMIN_URL + '/admin/import');
  await page.waitForSelector('h1, h2');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Import / Export Hub',
    description:
      'The Import/Export page is your data management center. Export your products, raw materials, ' +
      'or orders as CSV files, or import product and material data from CSV.',
    screenshot: await screenshot(page, 'Import export page'),
    tips: [
      'Export first to see the expected CSV column format before importing.',
      'Product CSV includes all fields: name, SKU, price, description, status, etc.',
      'Imports are validated — invalid rows are skipped with error messages.',
      'Use exports for backups or for migrating to/from other systems.',
    ],
  });

  steps.push({
    title: 'Full Import/Export Page',
    description:
      'The page shows all available export types at the top and import forms below. ' +
      'Each export type downloads a CSV immediately.',
    screenshot: await screenshotFull(page, 'Import export full page'),
  });

  return {
    id: 'import-export',
    title: '13. Import & Export',
    intro:
      'Bulk manage your data with CSV import and export. Export products, materials, and orders ' +
      'for reporting or backup. Import products and materials from CSV for bulk catalog updates.',
    steps,
  };
}

// ─── Section 14: Admin Users & Webhooks ───────────────────────────────

async function captureAdminAndWebhooks(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 14: Admin Users & Webhooks');
  const steps: DocStep[] = [];

  // Admin Users
  await page.goto(ADMIN_URL + '/admin/users');
  await page.waitForSelector('h2');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Admin Users',
    description:
      'Manage who has access to your admin panel. Each admin user has a role, 2FA status, ' +
      'and can be activated or deactivated. Add team members, warehouse staff, or your accountant.',
    screenshot: await screenshot(page, 'Admin users list'),
    tips: [
      'Two-factor authentication (2FA) is strongly recommended for all admin users.',
      'Deactivate users instead of deleting them to preserve the audit trail.',
      'Roles control what each user can see and do.',
    ],
  });

  // New user form
  await page.goto(ADMIN_URL + '/admin/users/new');
  await page.waitForSelector('h2');

  steps.push({
    title: 'Adding an Admin User',
    description:
      'Create a new admin account with email, name, password, and role. ' +
      'The user will be prompted to set up 2FA on their first login.',
    screenshot: await screenshot(page, 'New admin user form'),
  });

  // Webhooks
  await page.goto(ADMIN_URL + '/admin/webhooks');
  await page.waitForSelector('h2');
  await page.waitForTimeout(500);

  steps.push({
    title: 'Webhooks',
    description:
      'Webhooks send real-time notifications to external systems when events happen in your store ' +
      '(new order, payment received, stock change). This is how you integrate with ' +
      'shipping providers, accounting tools, or custom workflows.',
    screenshot: await screenshot(page, 'Webhooks list'),
    tips: [
      'Each endpoint URL receives POST requests with a JSON payload.',
      'Use the signing secret to verify webhook authenticity (HMAC-SHA256).',
      'Failed deliveries are retried automatically with exponential backoff.',
    ],
  });

  // New webhook form
  await page.goto(ADMIN_URL + '/admin/webhooks/new');
  await page.waitForSelector('h2');

  steps.push({
    title: 'Creating a Webhook Endpoint',
    description:
      'Configure the endpoint URL, select which events to subscribe to, and optionally set a signing secret. ' +
      'The webhook will fire for every selected event type.',
    screenshot: await screenshot(page, 'New webhook form'),
  });

  return {
    id: 'admin-webhooks',
    title: '14. Admin Users & Webhooks',
    intro:
      'Manage your team\'s access to the admin panel and set up integrations with external systems. ' +
      'Admin users control who can manage your store, while webhooks enable real-time integrations.',
    steps,
  };
}

// ─── HTML Generation ──────────────────────────────────────────────────

function generateHTML(sections: DocSection[]): string {
  const toc = sections
    .map(s => {
      const failedClass = s.failed ? ' class="toc-failed"' : '';
      return `        <li${failedClass}><a href="#${s.id}">${s.title}</a>${s.failed ? ' ⚠️' : ''}</li>`;
    })
    .join('\n');

  const sectionHTML = sections
    .map(section => {
      if (section.failed && section.steps.length === 0) {
        return `
      <section id="${section.id}" class="section-failed">
        <h2>${section.title} ⚠️</h2>
        <p class="section-intro">${section.intro}</p>
        <div class="error-box">
          <strong>Section generation failed:</strong> ${section.errorMsg || 'Unknown error'}
        </div>
      </section>`;
      }

      const stepsHTML = section.steps
        .map((step, i) => {
          const tipsHTML = step.tips
            ? `<div class="tips">
                <strong>💡 Tips:</strong>
                <ul>${step.tips.map(t => `<li>${t}</li>`).join('')}</ul>
              </div>`
            : '';
          const warningHTML = step.warning
            ? `<div class="warning-box">
                <strong>⚠️ Important:</strong> ${step.warning}
              </div>`
            : '';
          return `
          <div class="step">
            <h3>Step ${i + 1}: ${step.title}</h3>
            <p>${step.description}</p>
            <div class="screenshot-container">
              <img src="data:image/png;base64,${step.screenshot}" alt="${step.title}" loading="lazy" />
            </div>
            ${tipsHTML}
            ${warningHTML}
          </div>`;
        })
        .join('\n');

      return `
      <section id="${section.id}"${section.failed ? ' class="section-failed"' : ''}>
        <h2>${section.title}</h2>
        <p class="section-intro">${section.intro}</p>
        ${stepsHTML}
      </section>`;
    })
    .join('\n');

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Admin Guide | ForgeCommerce</title>
  <style>
    :root {
      --primary: #2563eb;
      --primary-dark: #1d4ed8;
      --gray-50: #f9fafb;
      --gray-100: #f3f4f6;
      --gray-200: #e5e7eb;
      --gray-300: #d1d5db;
      --gray-600: #4b5563;
      --gray-700: #374151;
      --gray-800: #1f2937;
      --gray-900: #111827;
      --green-50: #f0fdf4;
      --green-600: #16a34a;
      --blue-50: #eff6ff;
      --blue-600: #2563eb;
      --amber-50: #fffbeb;
      --amber-600: #d97706;
      --amber-700: #b45309;
      --red-50: #fef2f2;
      --red-600: #dc2626;
    }

    * { margin: 0; padding: 0; box-sizing: border-box; }

    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif;
      line-height: 1.6;
      color: var(--gray-800);
      background: var(--gray-50);
    }

    .container { max-width: 920px; margin: 0 auto; padding: 24px; }

    /* Header */
    .doc-header {
      background: linear-gradient(135deg, var(--primary), var(--primary-dark));
      color: white;
      padding: 56px 24px;
      text-align: center;
      margin-bottom: 32px;
    }
    .doc-header h1 { font-size: 2.2rem; font-weight: 700; margin-bottom: 8px; }
    .doc-header p { font-size: 1.1rem; opacity: 0.9; }
    .doc-header .badge {
      display: inline-block;
      background: rgba(255,255,255,0.2);
      padding: 4px 14px;
      border-radius: 12px;
      font-size: 0.85rem;
      margin-top: 12px;
    }

    /* Quick Start */
    .concept-grid {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 16px;
      margin: 24px 0;
    }
    .concept-box {
      background: var(--gray-50);
      border: 1px solid var(--gray-200);
      border-radius: 6px;
      padding: 16px;
    }
    .concept-box h4 { font-size: 0.95rem; color: var(--gray-800); margin-bottom: 4px; }
    .concept-box p { font-size: 0.9rem; color: var(--gray-600); margin: 0; }

    /* Table of Contents */
    .toc {
      background: white;
      border: 1px solid var(--gray-200);
      border-radius: 8px;
      padding: 24px;
      margin-bottom: 32px;
    }
    .toc h2 { font-size: 1.1rem; color: var(--gray-700); margin-bottom: 12px; }
    .toc ol { padding-left: 24px; }
    .toc li { margin-bottom: 6px; }
    .toc a { color: var(--primary); text-decoration: none; }
    .toc a:hover { text-decoration: underline; }
    .toc-failed a { color: var(--red-600); }

    /* Sections */
    section {
      background: white;
      border: 1px solid var(--gray-200);
      border-radius: 8px;
      padding: 32px;
      margin-bottom: 24px;
    }
    section h2 {
      font-size: 1.5rem;
      color: var(--gray-900);
      margin-bottom: 8px;
      padding-bottom: 12px;
      border-bottom: 2px solid var(--gray-100);
    }
    .section-intro { color: var(--gray-600); margin-bottom: 24px; font-size: 1.05rem; }

    .section-failed { border-color: var(--red-600); border-width: 2px; }

    /* Steps */
    .step {
      margin-bottom: 32px;
      padding-bottom: 32px;
      border-bottom: 1px solid var(--gray-100);
    }
    .step:last-child { margin-bottom: 0; padding-bottom: 0; border-bottom: none; }
    .step h3 { font-size: 1.15rem; color: var(--gray-800); margin-bottom: 8px; }
    .step > p { color: var(--gray-600); margin-bottom: 16px; }

    /* Screenshots */
    .screenshot-container {
      border: 1px solid var(--gray-200);
      border-radius: 8px;
      overflow: hidden;
      margin-bottom: 16px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    }
    .screenshot-container img { width: 100%; display: block; }

    /* Tips */
    .tips {
      background: var(--blue-50);
      border: 1px solid #bfdbfe;
      border-radius: 6px;
      padding: 16px;
      margin-top: 12px;
    }
    .tips strong { display: block; color: var(--blue-600); margin-bottom: 8px; }
    .tips ul { padding-left: 20px; margin: 0; }
    .tips li { color: var(--gray-700); margin-bottom: 4px; font-size: 0.95rem; }

    /* Warning */
    .warning-box {
      background: var(--amber-50);
      border: 1px solid #fbbf24;
      border-radius: 6px;
      padding: 16px;
      margin-top: 12px;
    }
    .warning-box strong { color: var(--amber-700); }

    /* Error */
    .error-box {
      background: var(--red-50);
      border: 1px solid #fca5a5;
      border-radius: 6px;
      padding: 16px;
      margin-top: 12px;
      color: var(--red-600);
    }

    /* Footer */
    .doc-footer {
      text-align: center;
      color: var(--gray-600);
      padding: 32px;
      font-size: 0.9rem;
    }
    .doc-footer a { color: var(--primary); }

    @media (max-width: 768px) {
      .concept-grid { grid-template-columns: 1fr; }
      .container { padding: 12px; }
      section { padding: 20px; }
    }

    @media print {
      .doc-header { background: var(--gray-800); -webkit-print-color-adjust: exact; }
      section { break-inside: avoid; page-break-inside: avoid; }
      .screenshot-container { box-shadow: none; }
    }
  </style>
</head>
<body>

  <div class="doc-header">
    <h1>ForgeCommerce Admin Guide</h1>
    <p>Complete guide to running your EU e-commerce store</p>
    <span class="badge">ForgeCommerce Documentation &mdash; Generated ${new Date().toISOString().split('T')[0]}</span>
  </div>

  <div class="container">

    <!-- Quick Start Concepts -->
    <section>
      <h2>Quick Start</h2>
      <p class="section-intro">
        ForgeCommerce is an EU-first e-commerce platform designed for small businesses and manufacturers.
        Here are the key areas you'll work with:
      </p>
      <div class="concept-grid">
        <div class="concept-box">
          <h4>🏪 Store Setup</h4>
          <p>Configure VAT, shipping zones, and selling countries before adding products.</p>
        </div>
        <div class="concept-box">
          <h4>📦 Products & Catalog</h4>
          <p>Create products with attributes, variants, images, and Bills of Materials.</p>
        </div>
        <div class="concept-box">
          <h4>🛒 Orders & Fulfillment</h4>
          <p>Process orders through their lifecycle: confirm, pack, ship, and deliver.</p>
        </div>
        <div class="concept-box">
          <h4>💰 VAT Compliance</h4>
          <p>Automatic EU VAT calculation, B2B reverse charge, and tax-filing reports.</p>
        </div>
        <div class="concept-box">
          <h4>🏭 Manufacturing</h4>
          <p>Track raw materials, BOMs, and production batches for your workshop.</p>
        </div>
        <div class="concept-box">
          <h4>📊 Reports & Integrations</h4>
          <p>Sales analytics, VAT reports for tax filing, CSV import/export, and webhooks.</p>
        </div>
      </div>
    </section>

    <!-- Table of Contents -->
    <div class="toc">
      <h2>📑 Table of Contents</h2>
      <ol>
${toc}
      </ol>
    </div>

${sectionHTML}

  </div>

  <div class="doc-footer">
    <p>
      Generated automatically with <a href="https://playwright.dev">Playwright</a> screenshots.<br>
      ForgeCommerce &mdash; EU-first e-commerce for small businesses and manufacturers.
    </p>
    <p style="margin-top: 8px; font-size: 0.8rem; color: var(--gray-300);">
      Generated on ${new Date().toISOString().split('T')[0]}
    </p>
  </div>

</body>
</html>`;
}

// ─── Main Entry Point ─────────────────────────────────────────────────

async function main() {
  console.log('🚀 ForgeCommerce Comprehensive Admin Documentation Generator');
  console.log(`   Admin URL: ${ADMIN_URL}`);
  console.log(`   Output: ${OUTPUT_FILE}`);
  console.log('');

  let browser: Browser | null = null;

  try {
    browser = await chromium.launch({
      headless: true,
      args: [
        '--unsafely-treat-insecure-origin-as-secure=' + ADMIN_URL,
      ],
    });
    const context = await browser.newContext({
      viewport: { width: 1280, height: 800 },
      ignoreHTTPSErrors: true,
      extraHTTPHeaders: {
        'X-Forwarded-For': `10.0.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}`,
      },
    });
    const page = await context.newPage();

    await adminLogin(page);

    // Capture all sections with error resilience
    const sections: DocSection[] = [];

    const sectionDefs: Array<{
      fn: (p: Page) => Promise<DocSection>;
      id: string;
      title: string;
      intro: string;
    }> = [
      { fn: captureLogin, id: 'login', title: '1. Logging In', intro: 'Access your admin panel.' },
      { fn: captureDashboard, id: 'dashboard', title: '2. Your Dashboard', intro: 'Your store overview.' },
      { fn: captureVATSettings, id: 'vat-settings', title: '3. Store Setup: VAT Configuration', intro: 'Configure EU VAT compliance.' },
      { fn: captureShippingSettings, id: 'shipping-settings', title: '4. Store Setup: Shipping', intro: 'Set up shipping rates.' },
      { fn: captureCategories, id: 'categories', title: '5. Managing Categories', intro: 'Organize your products.' },
      { fn: captureRawMaterials, id: 'raw-materials', title: '6. Raw Materials & Inventory', intro: 'Track your components.' },
      { fn: captureProducts, id: 'products', title: '7. Creating & Managing Products', intro: 'Your product catalog.' },
      { fn: captureGlobalAttributes, id: 'global-attributes', title: '8. Global Attribute Templates', intro: 'Reusable attribute templates.' },
      { fn: captureOrders, id: 'orders', title: '9. Processing Orders', intro: 'Order fulfillment.' },
      { fn: captureDiscounts, id: 'discounts', title: '10. Discounts & Coupons', intro: 'Promotions and coupons.' },
      { fn: captureProduction, id: 'production', title: '11. Production Batches', intro: 'Manufacturing management.' },
      { fn: captureReports, id: 'reports', title: '12. Reports & Tax Filing', intro: 'Analytics and VAT reports.' },
      { fn: captureImportExport, id: 'import-export', title: '13. Import & Export', intro: 'Bulk data management.' },
      { fn: captureAdminAndWebhooks, id: 'admin-webhooks', title: '14. Admin Users & Webhooks', intro: 'Team and integrations.' },
    ];

    for (const def of sectionDefs) {
      const section = await captureWithFallback(page, def.fn, def.id, def.title, def.intro);
      sections.push(section);
    }

    // Generate HTML
    console.log('\n📝 Generating HTML documentation...');
    const html = generateHTML(sections);

    // Ensure output directory exists
    if (!fs.existsSync(OUTPUT_DIR)) {
      fs.mkdirSync(OUTPUT_DIR, { recursive: true });
    }

    fs.writeFileSync(OUTPUT_FILE, html, 'utf-8');
    const sizeMB = (Buffer.byteLength(html) / 1024 / 1024).toFixed(1);

    const totalScreenshots = sections.reduce((sum, s) => sum + s.steps.length, 0);
    const failedSections = sections.filter(s => s.failed).length;

    console.log(`\n✅ Documentation generated: ${OUTPUT_FILE} (${sizeMB} MB)`);
    console.log(`   📸 ${totalScreenshots} screenshots across ${sections.length} sections`);
    if (failedSections > 0) {
      console.log(`   ⚠️  ${failedSections} section(s) had errors — check the output for details`);
    }
    console.log(`   Open in browser: file://${OUTPUT_FILE}`);

    await context.close();
  } catch (error) {
    console.error('\n❌ Fatal error generating documentation:', error);
    process.exit(1);
  } finally {
    if (browser) {
      await browser.close();
    }
  }
}

main();
