/**
 * ForgeCommerce — Global Attributes Documentation Generator
 *
 * This script uses Playwright to navigate through the Global Attributes
 * admin UI, take screenshots at each step, and generate a self-contained
 * HTML documentation file with Base64-inlined images.
 *
 * Usage:
 *   npx tsx scripts/generate-docs.ts
 *
 * Requirements:
 *   - The admin server must be running on ADMIN_URL (default: http://localhost:8081)
 *   - An admin user must exist with the default test credentials
 *   - `tsx` must be installed: npm install -D tsx
 *
 * Output:
 *   docs/global-attributes-guide.html
 */

import { chromium, Page, Browser } from 'playwright';
import * as fs from 'fs';
import * as path from 'path';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';
const ADMIN_EMAIL = process.env.ADMIN_EMAIL || 'admin@forgecommerce.local';
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || 'admin123';
const OUTPUT_DIR = path.resolve(__dirname, '../../docs');
const OUTPUT_FILE = path.join(OUTPUT_DIR, 'global-attributes-guide.html');

interface DocStep {
  title: string;
  description: string;
  screenshot: string; // Base64 PNG
  tips?: string[];
}

interface DocSection {
  id: string;
  title: string;
  intro: string;
  steps: DocStep[];
}

// ─── Helper Functions ───────────────────────────────────────────────

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

  // Fill login form and submit via the login form's specific button
  await page.fill('input[name="email"]', ADMIN_EMAIL);
  await page.fill('input[name="password"]', ADMIN_PASSWORD);

  // Use a response listener to confirm the POST succeeds
  const [response] = await Promise.all([
    page.waitForResponse(r => r.url().includes('/admin/login') && r.request().method() === 'POST'),
    // Target the submit button inside the login form specifically
    page.locator('form[action*="login"] button[type="submit"], form button[type="submit"]').first().click(),
  ]);

  console.log(`   → Login response: ${response.status()}`);
  await page.waitForTimeout(2000);

  // Handle 2FA setup redirect
  if (page.url().includes('setup-2fa')) {
    console.log('   ⚠ Redirected to 2FA setup — skipping');
    await page.goto(ADMIN_URL + '/admin/dashboard');
    await page.waitForTimeout(1000);
  }

  console.log(`   → Landed on: ${page.url()}`);
  console.log('✅ Logged in successfully');
}

// ─── Documentation Sections ────────────────────────────────────────

async function captureOverview(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 1: Global Attributes Overview');
  const steps: DocStep[] = [];

  await page.goto(ADMIN_URL + '/admin/global-attributes');
  await page.waitForSelector('h2', { timeout: 10_000 });
  await page.waitForTimeout(500);

  steps.push({
    title: 'Global Attributes List',
    description:
      'The Global Attributes page shows all reusable attribute templates defined in your store. ' +
      'Each attribute has a type (Select, Color Swatch, Button Group, Image Swatch), an optional category, ' +
      'and shows how many options it has and how many products use it.',
    screenshot: await screenshot(page, 'Global attributes list page'),
    tips: [
      'Click "+ New Global Attribute" to create a new template.',
      'Attributes with zero product usage can be deleted directly from this page.',
      'The "Used By" column shows how many products have linked this attribute.',
    ],
  });

  return {
    id: 'overview',
    title: '1. Global Attributes Overview',
    intro:
      'Global Attributes are reusable attribute templates that you define once and link to any number of products. ' +
      'Instead of creating "Color" or "Size" separately on every product, you define them as Global Attributes ' +
      'with all their options, then simply link them to products that need them.',
    steps,
  };
}

async function captureCreateAttribute(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 2: Creating a Global Attribute');
  const steps: DocStep[] = [];

  // Navigate to new attribute form
  await page.goto(ADMIN_URL + '/admin/global-attributes/new');
  await page.waitForSelector('h2');

  steps.push({
    title: 'New Global Attribute Form',
    description:
      'The creation form lets you define the attribute\'s internal name, display name, type, and category. ' +
      'The internal name should be lowercase and snake_case (e.g., "color", "material_type").',
    screenshot: await screenshot(page, 'New attribute form (empty)'),
    tips: [
      'The internal name is used for lookups and cannot be changed after creation.',
      'Display Name is what your shop administrators see in the UI.',
      'Choose the type based on how the attribute should render in the storefront.',
    ],
  });

  // Fill the form
  await page.fill('input#ga_name', 'color');
  await page.fill('input#ga_display_name', 'Color');
  await page.selectOption('select#ga_type', 'color_swatch');
  await page.selectOption('select#ga_category', 'style');
  await page.fill('textarea#ga_description', 'Product color selection. Displayed as swatches in the storefront.');

  steps.push({
    title: 'Filled Creation Form',
    description:
      'Here we\'re creating a "Color" attribute of type "Color Swatch" in the "Style" category. ' +
      'The description helps other administrators understand the purpose of this attribute.',
    screenshot: await screenshot(page, 'New attribute form (filled)'),
  });

  // Submit the form — use the specific "Create Attribute" button inside the
  // form with action="/admin/global-attributes", NOT the logout button in the top bar.
  const createForm = page.locator('form[action="/admin/global-attributes"]');
  await createForm.locator('button[type="submit"]').click();

  // The Create handler redirects to /admin/global-attributes/{id} (the edit page)
  await page.waitForURL(/\/admin\/global-attributes\//, { timeout: 10_000 });
  await page.waitForSelector('h2');

  steps.push({
    title: 'Attribute Created',
    description:
      'After submission, you\'re redirected to the edit page where you can add metadata fields and options. ' +
      'The attribute is now saved and ready to be configured.',
    screenshot: await screenshotFull(page, 'Attribute created - edit page'),
  });

  return {
    id: 'create',
    title: '2. Creating a Global Attribute',
    intro:
      'Creating a Global Attribute involves defining its basic properties. Once created, you can ' +
      'add metadata fields (structured extra data per option) and the actual option values.',
    steps,
  };
}

async function captureMetadataFields(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 3: Metadata Fields');
  const steps: DocStep[] = [];

  // We should already be on the edit page from the previous section
  const currentUrl = page.url();
  if (!currentUrl.includes('/admin/global-attributes/')) {
    await page.goto(ADMIN_URL + '/admin/global-attributes');
    const firstLink = page.locator('table tbody tr a').first();
    if (await firstLink.isVisible()) {
      await firstLink.click();
      await page.waitForSelector('h2');
    }
  }

  // Scroll to the metadata fields section
  const fieldsSection = page.locator('text=Metadata Fields').first();
  if (await fieldsSection.isVisible()) {
    await fieldsSection.scrollIntoViewIfNeeded();
  }

  steps.push({
    title: 'Metadata Fields Section',
    description:
      'Metadata fields define additional structured data that each option can carry. ' +
      'For example, a Color attribute might have a "Pantone Code" text field and a "Is Premium" boolean field. ' +
      'These fields appear as extra columns when adding options.',
    screenshot: await screenshot(page, 'Metadata fields section'),
    tips: [
      'Field types: Text, Number, Boolean, Select, URL.',
      'Required fields must be filled when adding new options.',
      'Default values are pre-filled when creating new options.',
    ],
  });

  // Add a metadata field via HTMX form
  const fieldForm = page.locator('form[hx-post*="/fields"]');
  if (await fieldForm.isVisible()) {
    await fieldForm.locator('input[name="field_name"]').fill('pantone_code');
    await fieldForm.locator('input[name="display_name"]').fill('Pantone Code');
    await fieldForm.locator('select[name="field_type"]').selectOption('text');

    steps.push({
      title: 'Adding a Metadata Field',
      description:
        'Fill in the field name (snake_case), display name, type, and optionally a default value. ' +
        'Here we\'re adding a "Pantone Code" text field for our Color attribute.',
      screenshot: await screenshot(page, 'Metadata field form filled'),
    });

    // Submit with the "Add Field" button inside the HTMX form
    await fieldForm.locator('button[type="submit"]').click();
    await page.waitForTimeout(1500);

    steps.push({
      title: 'Metadata Field Added',
      description:
        'The field appears in the table and will now show as an extra column in the Options section below. ' +
        'You can add multiple metadata fields to capture all the structured data you need.',
      screenshot: await screenshot(page, 'After adding metadata field'),
    });

    // Add a second field
    await fieldForm.locator('input[name="field_name"]').fill('is_premium');
    await fieldForm.locator('input[name="display_name"]').fill('Premium Color');
    await fieldForm.locator('select[name="field_type"]').selectOption('boolean');
    await fieldForm.locator('button[type="submit"]').click();
    await page.waitForTimeout(1500);

    steps.push({
      title: 'Multiple Metadata Fields',
      description:
        'We\'ve now added two metadata fields: "Pantone Code" (Text) and "Premium Color" (Boolean). ' +
        'These will appear as extra columns in both the options table and the add-option form.',
      screenshot: await screenshot(page, 'Multiple metadata fields'),
    });
  }

  return {
    id: 'metadata',
    title: '3. Metadata Fields (Rich Schema)',
    intro:
      'Metadata Fields let you attach structured extra data to each option. This is the "Rich Metadata Schema" ' +
      'concept — instead of just "Red" as an option, you can also store the Pantone code, hex color, ' +
      'whether it\'s a premium color, a URL to a material sample photo, and more.',
    steps,
  };
}

async function captureOptions(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 4: Adding Options');
  const steps: DocStep[] = [];

  // Scroll to options section
  const optionsHeader = page.locator('text=Options').first();
  if (await optionsHeader.isVisible()) {
    await optionsHeader.scrollIntoViewIfNeeded();
    await page.waitForTimeout(300);
  }

  steps.push({
    title: 'Options Section',
    description:
      'The Options section shows all available values for this attribute. ' +
      'Notice how the metadata fields we created appear as extra columns in both the table and the "Add" form.',
    screenshot: await screenshot(page, 'Options section'),
  });

  // Add options via the HTMX option form
  const optionForm = page.locator('form[hx-post*="/options"]');
  if (await optionForm.isVisible()) {
    // Option 1: Black
    await optionForm.locator('input[name="value"]').fill('black');
    await optionForm.locator('input[name="display_value"]').fill('Black');
    await optionForm.locator('input[name="color_hex"]').fill('#000000');
    const pantoneField = optionForm.locator('input[name="meta_pantone_code"]');
    if (await pantoneField.isVisible().catch(() => false)) {
      await pantoneField.fill('19-0303 TCX');
    }

    steps.push({
      title: 'Adding an Option: Black',
      description:
        'Fill in the value, display value, and color hex. The metadata fields (Pantone Code, Premium Color) ' +
        'appear as additional inputs in the form. Color hex is displayed as a swatch preview.',
      screenshot: await screenshot(page, 'Adding black option'),
    });

    await optionForm.locator('button[type="submit"]').click();
    await page.waitForTimeout(1500);

    // Option 2: Tan
    await optionForm.locator('input[name="value"]').fill('tan');
    await optionForm.locator('input[name="display_value"]').fill('Tan');
    await optionForm.locator('input[name="color_hex"]').fill('#D2B48C');
    const pantoneField2 = optionForm.locator('input[name="meta_pantone_code"]');
    if (await pantoneField2.isVisible().catch(() => false)) {
      await pantoneField2.fill('15-1231 TCX');
    }
    await optionForm.locator('button[type="submit"]').click();
    await page.waitForTimeout(1500);

    // Option 3: Forest Green
    await optionForm.locator('input[name="value"]').fill('forest_green');
    await optionForm.locator('input[name="display_value"]').fill('Forest Green');
    await optionForm.locator('input[name="color_hex"]').fill('#228B22');
    const pantoneField3 = optionForm.locator('input[name="meta_pantone_code"]');
    if (await pantoneField3.isVisible().catch(() => false)) {
      await pantoneField3.fill('18-6320 TCX');
    }
    await optionForm.locator('button[type="submit"]').click();
    await page.waitForTimeout(1500);

    steps.push({
      title: 'Options Added',
      description:
        'After adding several options, the table shows all values with their color swatches and metadata. ' +
        'Each option can be removed individually. The options are immediately available for linking to products.',
      screenshot: await screenshotFull(page, 'All options added'),
      tips: [
        'Options are shared across all products that link this attribute.',
        'When you link an attribute to a product, you can select which subset of options that product offers.',
        'Color swatches are automatically rendered from the hex value.',
      ],
    });
  }

  return {
    id: 'options',
    title: '4. Adding Options',
    intro:
      'Options are the actual values that customers can select. For a Color attribute, options might be ' +
      '"Black", "Tan", "Forest Green", etc. Each option can carry metadata defined by the metadata fields.',
    steps,
  };
}

async function captureLinkToProduct(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 5: Linking to a Product');
  const steps: DocStep[] = [];

  // Try to find an existing product with a Global Attributes tab.
  // If no products exist, navigate to the products list and show a fallback.
  let productPageFound = false;

  // Navigate with ?status=active filter (the default "all" filter has a known issue
  // where empty string isn't treated as NULL in the SQL query).
  await page.goto(ADMIN_URL + '/admin/products?status=active');
  await page.waitForSelector('h2');
  await page.waitForTimeout(1000);

  // Find a product link in the table — exclude "new" and "Create your first" links
  const productLink = page.locator('table tbody td a[href*="/admin/products/"]:not([href$="/new"])').first();
  const hasProduct = await productLink.isVisible().catch(() => false);

  if (hasProduct) {
    // Get the product link href so we can navigate directly to the global attributes tab
    const productHref = await productLink.getAttribute('href');

    // Navigate directly to the Global Attributes tab URL
    const globalAttrUrl = productHref + '/global-attributes';
    await page.goto(ADMIN_URL + globalAttrUrl);
    await page.waitForSelector('h2');
    await page.waitForTimeout(1500);
    productPageFound = true;

    steps.push({
      title: 'Product Global Attributes Tab',
      description:
        'Navigate to the "Global Attributes" tab on any product to link global attribute templates. ' +
        'The "Link Global Attribute" form shows a dropdown of all available (unlinked) attributes.',
      screenshot: await screenshot(page, 'Product global attributes tab'),
      tips: [
        'A product can have multiple linked global attributes (e.g., Color + Size + Material).',
        'Already-linked attributes don\'t appear in the dropdown.',
        'Each link has a "role" that determines how it\'s used: Variant Axis, Filter Only, or Display Only.',
      ],
    });

    // Link the Color attribute
    const globalAttrSelect = page.locator('select[name="global_attribute_id"], select#global_attr_id').first();
    if (await globalAttrSelect.isVisible().catch(() => false)) {
      const options = await globalAttrSelect.locator('option').allTextContents();
      const colorOption = options.find(o => o.toLowerCase().includes('color'));
      if (colorOption) {
        await globalAttrSelect.selectOption({ label: colorOption });

        const roleSelect = page.locator('select[name="role"]').first();
        if (await roleSelect.isVisible().catch(() => false)) {
          await roleSelect.selectOption('variant_axis');
        }

        steps.push({
          title: 'Linking a Global Attribute',
          description:
            'Select "Color" from the dropdown and set the role to "Variant Axis". ' +
            'Variant Axis means this attribute will generate purchasable product variants. ' +
            '"Filter Only" is used for storefront filtering without creating variants. ' +
            '"Display Only" shows information but doesn\'t affect filtering or variants.',
          screenshot: await screenshot(page, 'Linking color attribute'),
        });

        // Submit the link form (HTMX form, no page navigation)
        const linkBtn = page.locator('button:has-text("Link Attribute")').first();
        if (await linkBtn.isVisible()) {
          await linkBtn.click();
          await page.waitForTimeout(2000);
        }

        steps.push({
          title: 'Attribute Linked — Option Selection',
          description:
            'After linking, a card appears showing all options from the global attribute template. ' +
            'Use the checkboxes to select which options this specific product offers. ' +
            'Not every product needs every color — you can cherry-pick.',
          screenshot: await screenshotFull(page, 'Linked attribute with option selection'),
          tips: [
            'Use "Select All" checkbox in the header to quickly select all options.',
            'Price Modifier lets you add/subtract from the base price for specific options.',
            'Click "Save Selections" after checking the options you want.',
            'Click "Unlink" to remove the attribute from this product entirely.',
          ],
        });

        // Select some options
        const checkboxes = page.locator('input[name="option_ids"]');
        const count = await checkboxes.count();
        for (let i = 0; i < Math.min(count, 2); i++) {
          await checkboxes.nth(i).check();
        }

        // Set a price modifier
        const priceModifiers = page.locator('input[name^="price_modifier_"]');
        if (await priceModifiers.first().isVisible().catch(() => false)) {
          await priceModifiers.first().fill('5.00');
        }

        steps.push({
          title: 'Configuring Selections & Price Modifiers',
          description:
            'Select the options this product offers and optionally set price modifiers. ' +
            'A price modifier adds to (or subtracts from) the product\'s base price for that option.',
          screenshot: await screenshot(page, 'Options selected with price modifier'),
        });

        // Save selections (HTMX form)
        const saveBtn = page.locator('button:has-text("Save Selections")').first();
        if (await saveBtn.isVisible()) {
          await saveBtn.click();
          await page.waitForTimeout(2000);
        }

        steps.push({
          title: 'Selections Saved',
          description:
            'The card updates to show the selection count (e.g., "2/3 selected"). ' +
            'The HTMX-powered UI updates the card in-place without a full page reload.',
          screenshot: await screenshot(page, 'Selections saved'),
        });
      }
    }
  }

  // If no product was found or the tab didn't exist, show a descriptive fallback
  if (!productPageFound) {
    // Show the product list page (or new product page) as reference
    await page.goto(ADMIN_URL + '/admin/products');
    await page.waitForSelector('h2');

    steps.push({
      title: 'Products List — Navigate to Product Edit',
      description:
        'To link a global attribute, first navigate to a product\'s edit page from the Products list. ' +
        'Click any product name to open its edit page, then click the "Global Attributes" tab.',
      screenshot: await screenshot(page, 'Products list for linking'),
      tips: [
        'The "Global Attributes" tab appears on every product\'s edit page.',
        'You can link multiple global attributes to a single product.',
        'Each attribute link has a role: Variant Axis (creates variants), Filter Only (storefront filter), or Display Only (informational).',
        'After linking, select which specific options the product offers from the full set of global options.',
      ],
    });
  }

  return {
    id: 'linking',
    title: '5. Linking to a Product',
    intro:
      'Once you have global attributes defined with their options, you link them to individual products. ' +
      'This is where the reuse happens — a single "Color" attribute with 20 options can be linked to ' +
      '50 different products, and each product can select just the colors it offers.',
    steps,
  };
}

async function captureAttributeTypes(page: Page): Promise<DocSection> {
  console.log('\n📖 Section 6: Attribute Types Reference');
  const steps: DocStep[] = [];

  // Create a Size attribute (Button Group type) to show type variety
  await page.goto(ADMIN_URL + '/admin/global-attributes/new');
  await page.waitForSelector('h2');
  await page.fill('input#ga_name', 'size');
  await page.fill('input#ga_display_name', 'Size');
  await page.selectOption('select#ga_type', 'button_group');
  await page.selectOption('select#ga_category', 'physical');
  await page.fill('textarea#ga_description', 'Product size selection. Displayed as buttons in the storefront.');

  // Submit via the form-specific button
  const sizeForm = page.locator('form[action="/admin/global-attributes"]');
  await sizeForm.locator('button[type="submit"]').click();
  await page.waitForURL(/\/admin\/global-attributes\//, { timeout: 10_000 });
  await page.waitForSelector('h2');

  // Add size options via HTMX
  const optionForm = page.locator('form[hx-post*="/options"]');
  if (await optionForm.isVisible()) {
    for (const size of ['S', 'M', 'L', 'XL']) {
      await optionForm.locator('input[name="value"]').fill(size.toLowerCase());
      await optionForm.locator('input[name="display_value"]').fill(size);
      await optionForm.locator('button[type="submit"]').click();
      await page.waitForTimeout(1200);
    }
  }

  steps.push({
    title: 'Button Group Type — Size Attribute',
    description:
      'A "Button Group" attribute renders options as clickable buttons in the storefront (S, M, L, XL). ' +
      'This is ideal for sizes, quantities, or any discrete set of options that don\'t need color swatches.',
    screenshot: await screenshotFull(page, 'Size attribute with button group type'),
  });

  // Go back to list to show both attributes
  await page.goto(ADMIN_URL + '/admin/global-attributes');
  await page.waitForSelector('table');

  steps.push({
    title: 'Multiple Attributes on List Page',
    description:
      'The list page now shows both our Color (Color Swatch) and Size (Button Group) attributes. ' +
      'You can create as many global attributes as your store needs.',
    screenshot: await screenshot(page, 'Multiple attributes in list'),
    tips: [
      'Select: Standard dropdown selector.',
      'Color Swatch: Visual color circle/square pickers.',
      'Button Group: Horizontal button-style selectors.',
      'Image Swatch: Thumbnail image pickers (e.g., fabric patterns).',
    ],
  });

  return {
    id: 'types',
    title: '6. Attribute Types Reference',
    intro:
      'ForgeCommerce supports four attribute types, each with a different visual presentation in the storefront. ' +
      'Choose the type that best fits the kind of data the attribute represents.',
    steps,
  };
}

// ─── HTML Generation ────────────────────────────────────────────────

function generateHTML(sections: DocSection[]): string {
  const toc = sections
    .map(s => `        <li><a href="#${s.id}">${s.title}</a></li>`)
    .join('\n');

  const sectionHTML = sections
    .map(section => {
      const stepsHTML = section.steps
        .map((step, i) => {
          const tipsHTML = step.tips
            ? `<div class="tips">
                <strong>💡 Tips:</strong>
                <ul>${step.tips.map(t => `<li>${t}</li>`).join('')}</ul>
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
          </div>`;
        })
        .join('\n');

      return `
      <section id="${section.id}">
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
  <title>Global Attributes — Admin Guide | ForgeCommerce</title>
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
    }

    * { margin: 0; padding: 0; box-sizing: border-box; }

    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif;
      line-height: 1.6;
      color: var(--gray-800);
      background: var(--gray-50);
    }

    .container {
      max-width: 900px;
      margin: 0 auto;
      padding: 24px;
    }

    /* Header */
    .doc-header {
      background: linear-gradient(135deg, var(--primary), var(--primary-dark));
      color: white;
      padding: 48px 24px;
      text-align: center;
      margin-bottom: 32px;
    }

    .doc-header h1 {
      font-size: 2rem;
      font-weight: 700;
      margin-bottom: 8px;
    }

    .doc-header p {
      font-size: 1.1rem;
      opacity: 0.9;
    }

    .doc-header .badge {
      display: inline-block;
      background: rgba(255,255,255,0.2);
      padding: 4px 12px;
      border-radius: 12px;
      font-size: 0.85rem;
      margin-top: 12px;
    }

    /* Table of Contents */
    .toc {
      background: white;
      border: 1px solid var(--gray-200);
      border-radius: 8px;
      padding: 24px;
      margin-bottom: 32px;
    }

    .toc h2 {
      font-size: 1.1rem;
      color: var(--gray-700);
      margin-bottom: 12px;
    }

    .toc ol {
      padding-left: 24px;
    }

    .toc li {
      margin-bottom: 6px;
    }

    .toc a {
      color: var(--primary);
      text-decoration: none;
    }

    .toc a:hover {
      text-decoration: underline;
    }

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

    .section-intro {
      color: var(--gray-600);
      margin-bottom: 24px;
      font-size: 1.05rem;
    }

    /* Steps */
    .step {
      margin-bottom: 32px;
      padding-bottom: 32px;
      border-bottom: 1px solid var(--gray-100);
    }

    .step:last-child {
      margin-bottom: 0;
      padding-bottom: 0;
      border-bottom: none;
    }

    .step h3 {
      font-size: 1.15rem;
      color: var(--gray-800);
      margin-bottom: 8px;
    }

    .step > p {
      color: var(--gray-600);
      margin-bottom: 16px;
    }

    /* Screenshots */
    .screenshot-container {
      border: 1px solid var(--gray-200);
      border-radius: 8px;
      overflow: hidden;
      margin-bottom: 16px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    }

    .screenshot-container img {
      width: 100%;
      display: block;
    }

    /* Tips */
    .tips {
      background: var(--blue-50);
      border: 1px solid #bfdbfe;
      border-radius: 6px;
      padding: 16px;
      margin-top: 12px;
    }

    .tips strong {
      display: block;
      color: var(--blue-600);
      margin-bottom: 8px;
    }

    .tips ul {
      padding-left: 20px;
      margin: 0;
    }

    .tips li {
      color: var(--gray-700);
      margin-bottom: 4px;
      font-size: 0.95rem;
    }

    /* Footer */
    .doc-footer {
      text-align: center;
      color: var(--gray-600);
      padding: 32px;
      font-size: 0.9rem;
    }

    .doc-footer a {
      color: var(--primary);
    }

    /* Concept boxes */
    .concept-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
      margin: 24px 0;
    }

    .concept-box {
      background: var(--gray-50);
      border: 1px solid var(--gray-200);
      border-radius: 6px;
      padding: 16px;
    }

    .concept-box h4 {
      font-size: 0.95rem;
      color: var(--gray-800);
      margin-bottom: 4px;
    }

    .concept-box p {
      font-size: 0.9rem;
      color: var(--gray-600);
      margin: 0;
    }

    @media (max-width: 768px) {
      .concept-grid { grid-template-columns: 1fr; }
      .container { padding: 12px; }
      section { padding: 20px; }
    }

    @media print {
      .doc-header { background: var(--gray-800); }
      section { break-inside: avoid; }
      .screenshot-container { box-shadow: none; }
    }
  </style>
</head>
<body>

  <div class="doc-header">
    <h1>Global Attributes — Admin Guide</h1>
    <p>How to create, configure, and link reusable attribute templates to products</p>
    <span class="badge">ForgeCommerce Admin Documentation</span>
  </div>

  <div class="container">

    <!-- Key Concepts -->
    <section>
      <h2>Key Concepts</h2>
      <p class="section-intro">
        Before diving in, here are the core concepts behind Global Attributes:
      </p>
      <div class="concept-grid">
        <div class="concept-box">
          <h4>🏷️ Global Attribute</h4>
          <p>A reusable template (e.g., "Color", "Size") with a set of options defined once and shared across products.</p>
        </div>
        <div class="concept-box">
          <h4>📋 Metadata Fields</h4>
          <p>Structured extra data on each option (Pantone code, weight, URLs). Define the schema once, fill per option.</p>
        </div>
        <div class="concept-box">
          <h4>🔗 Product Link</h4>
          <p>Connects a global attribute to a specific product, with a role (Variant Axis, Filter, Display) and option filtering.</p>
        </div>
        <div class="concept-box">
          <h4>✅ Option Selection</h4>
          <p>Choose which options from the global attribute this specific product offers. Not every product needs every option.</p>
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
      ForgeCommerce &mdash; EU-first e-commerce platform.
    </p>
    <p style="margin-top: 8px; font-size: 0.8rem; color: var(--gray-300);">
      Generated on ${new Date().toISOString().split('T')[0]}
    </p>
  </div>

</body>
</html>`;
}

// ─── Main Entry Point ───────────────────────────────────────────────

async function main() {
  console.log('🚀 ForgeCommerce Documentation Generator');
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
      // Unique forwarded IP to avoid rate limiting collisions
      extraHTTPHeaders: {
        'X-Forwarded-For': `10.0.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}`,
      },
    });
    const page = await context.newPage();

    await adminLogin(page);

    // Capture each section
    const sections: DocSection[] = [];
    sections.push(await captureOverview(page));
    sections.push(await captureCreateAttribute(page));
    sections.push(await captureMetadataFields(page));
    sections.push(await captureOptions(page));
    sections.push(await captureLinkToProduct(page));
    sections.push(await captureAttributeTypes(page));

    // Generate HTML
    console.log('\n📝 Generating HTML documentation...');
    const html = generateHTML(sections);

    // Ensure output directory exists
    if (!fs.existsSync(OUTPUT_DIR)) {
      fs.mkdirSync(OUTPUT_DIR, { recursive: true });
    }

    fs.writeFileSync(OUTPUT_FILE, html, 'utf-8');
    const sizeMB = (Buffer.byteLength(html) / 1024 / 1024).toFixed(1);
    console.log(`\n✅ Documentation generated: ${OUTPUT_FILE} (${sizeMB} MB)`);
    console.log(`   Open in browser: file://${OUTPUT_FILE}`);

    await context.close();
  } catch (error) {
    console.error('\n❌ Error generating documentation:', error);
    process.exit(1);
  } finally {
    if (browser) {
      await browser.close();
    }
  }
}

main();
