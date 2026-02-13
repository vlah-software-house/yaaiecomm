# ForgeCommerce VAT Model

> This document explains how ForgeCommerce handles EU VAT for business owners,
> accountants, and developers integrating with the platform.

---

## Overview

ForgeCommerce provides comprehensive EU VAT compliance:

- Automatic VAT rate updates from the European Commission
- Per-country, per-product VAT category assignment
- B2B reverse charge for intra-EU cross-border sales
- VIES VAT number validation
- VAT-inclusive and VAT-exclusive pricing modes
- VAT reports suitable for quarterly tax filing

---

## VAT Configuration

### Enabling VAT

VAT is **optional**. If your business is not VAT-registered, leave VAT disabled
and all prices are treated as net amounts with no tax applied.

When enabled, configure:

| Setting                   | Description                                               |
|--------------------------|-----------------------------------------------------------|
| **Store VAT Number**     | Your business VAT number (e.g., `ES12345678A`)           |
| **Store Country**        | Your country of establishment (ISO 3166-1, e.g., `ES`)   |
| **Prices Include VAT**   | Whether prices in the admin are entered with VAT included |
| **Default VAT Category** | Applied to all products unless overridden                 |
| **B2B Reverse Charge**   | Enable 0% VAT for validated intra-EU B2B purchases       |

### Selling Countries

Select which EU countries you sell and ship to. Only enabled countries:
- Appear in the storefront checkout country selector
- Are considered for VAT calculations
- Can be assigned shipping zones

---

## VAT Rate Types

The EU VAT Directive defines these rate types:

| Rate Type       | Description                          | Example               |
|----------------|--------------------------------------|-----------------------|
| **Standard**   | Default rate (minimum 15%)           | DE: 19%, FR: 20%     |
| **Reduced**    | First reduced rate (minimum 5%)      | DE: 7%, FR: 5.5%     |
| **Reduced Alt** | Second reduced rate                 | FR: 10%, BE: 12%     |
| **Super Reduced** | Below 5% (grandfathered countries) | ES: 4%, FR: 2.1%    |
| **Parking**    | Transitional rate (12%+)             | BE: 12%, LU: 12%     |
| **Zero**       | 0% with right of deduction           | Some basic goods      |
| **Exempt**     | No VAT, no input deduction           | Medical, financial    |

Not all countries have all rate types. Denmark has only a standard rate (25%).

---

## VAT Categories

Products are assigned a **VAT category** which maps to the rate type used in
each destination country:

| Category        | Maps To        | Typical Use                        |
|----------------|----------------|------------------------------------|
| Standard Rate  | `standard`     | Most goods and services            |
| Reduced Rate   | `reduced`      | Food, books, medical supplies      |
| Reduced Alt    | `reduced_alt`  | Hospitality, cultural events       |
| Super Reduced  | `super_reduced`| Basic necessities (select countries)|
| Zero Rate      | `zero`         | Exports, specific exempt goods     |
| Exempt         | `exempt`       | Financial services, education      |

### Category Resolution Priority

When calculating VAT for a product going to a specific country:

```
1. Product + Country Override (ProductVATOverride table)
   ↓ (if not set)
2. Product-level VAT Category
   ↓ (if not set)
3. Store Default VAT Category
```

**Example:** A food product might be "Standard Rate" by default but "Reduced Rate"
in France and Germany, where food qualifies for reduced VAT. You would set the
product to "Standard Rate" and add country overrides for FR and DE.

---

## VAT Calculation

### For VAT-Inclusive Prices

When the store's prices include VAT:

```
Net Price = Product Price / (1 + VAT Rate / 100)
VAT Amount = Product Price - Net Price
```

**Example:** Product price = EUR 121.00, Spanish standard rate = 21%
- Net Price = 121.00 / 1.21 = EUR 100.00
- VAT Amount = 121.00 - 100.00 = EUR 21.00

### For VAT-Exclusive Prices

When the store's prices are net (excluding VAT):

```
VAT Amount = Net Price * (VAT Rate / 100)
Gross Price = Net Price + VAT Amount
```

**Example:** Net price = EUR 100.00, French standard rate = 20%
- VAT Amount = 100.00 * 0.20 = EUR 20.00
- Gross Price = 100.00 + 20.00 = EUR 120.00

### Fallback Logic

If a product's VAT category maps to a rate type that doesn't exist in the
destination country (e.g., "super_reduced" in Denmark), the system falls back
to the **standard rate** for that country.

---

## B2B Reverse Charge

For intra-EU B2B transactions, the **reverse charge mechanism** shifts VAT
liability from the seller to the buyer. This means:

- The seller charges 0% VAT
- The buyer self-accounts for VAT in their own country

### When Reverse Charge Applies

All conditions must be met:

1. **B2B reverse charge is enabled** in store settings
2. **Customer provides a VAT number** during checkout
3. **VAT number is validated** via the EU VIES service
4. **Different country**: customer's country differs from the store's country

If the customer is in the **same country** as the store, normal domestic VAT
applies regardless of B2B status.

### VIES Validation

VAT numbers are validated in real-time against the EU's VIES (VAT Information
Exchange System) service. The validation:

- Confirms the number is active and registered
- Returns the company name and address
- Generates a consultation number (for audit)
- Results are cached for 24 hours to avoid excessive API calls

---

## VAT Rate Sync

VAT rates are automatically synchronized from official EU sources:

### Data Sources

1. **Primary: EC TEDB** (European Commission Tax Database)
   - Official SOAP service maintained by EU member states
   - Endpoint: `ec.europa.eu/taxation_customs/tedb`

2. **Fallback: euvatrates.com**
   - Community-maintained JSON API (MIT licensed)
   - Used when TEDB is unavailable

### Sync Schedule

| Trigger              | When                           |
|---------------------|--------------------------------|
| **Startup**         | On application boot            |
| **Daily automatic** | Midnight UTC                   |
| **Manual**          | Admin > Settings > "Sync Now"  |

### Sync Logic

```
1. Try EC TEDB SOAP service
2. If TEDB fails → try euvatrates.com JSON
3. If both fail → keep existing cached rates
4. If rates changed → update database + refresh cache + log changes
5. On failure → retry up to 3 times (1-hour intervals)
```

Rate changes are logged in the admin audit log with before/after values.

---

## VAT on Orders

When an order is placed, all VAT information is **snapshotted** on the order.
This ensures the order always reflects what was charged, even if rates change
later.

### Order-Level VAT Fields

| Field               | Description                                |
|--------------------|--------------------------------------------|
| `vat_country_code` | Destination country used for calculation   |
| `vat_number`       | Customer's VAT number (B2B)               |
| `vat_company_name` | Company name from VIES validation          |
| `vat_reverse_charge` | Whether reverse charge was applied       |
| `vat_total`        | Total VAT amount on the order              |

### Per-Item VAT Fields

| Field              | Description                                |
|-------------------|--------------------------------------------|
| `vat_rate`        | Rate applied (e.g., 21.00)                 |
| `vat_rate_type`   | Rate type (standard, reduced, etc.)        |
| `vat_amount`      | VAT amount for this line item              |
| `price_includes_vat` | Whether the recorded price includes VAT |
| `net_unit_price`  | Price without VAT                          |
| `gross_unit_price`| Price with VAT                             |

---

## VAT Reports

ForgeCommerce generates VAT reports for tax filing purposes.

### Sales VAT Report

Available at **Admin > Reports > VAT Report** with period selection
(monthly, quarterly, yearly).

The report shows:

```
Country     Rate Type    Net Sales    VAT Collected    Gross Sales
Spain       Standard     EUR 12,450   EUR 2,614.50     EUR 15,064.50
Spain       Reduced      EUR 3,200    EUR 320.00       EUR 3,520.00
France      Standard     EUR 5,100    EUR 1,020.00     EUR 6,120.00
Germany     Reduced      EUR 1,200    EUR 84.00        EUR 1,284.00
```

### B2B Reverse Charge Report

Separate section showing:
- Number of reverse charge orders
- Total net revenue from reverse charge orders
- List of customer VAT numbers and company names

### Export

Reports can be exported as CSV for importing into accounting software or
submitting to tax authorities.

---

## Frequently Asked Questions

**Q: Do I need to enable VAT?**
No. If your business is not VAT-registered, leave it disabled. All prices are
treated as net amounts.

**Q: What happens if the VAT rate sync fails?**
The system uses cached rates from the database. Rates are updated once daily
and changes are rare, so a 24-hour stale cache has minimal impact.

**Q: Can I override VAT rates manually?**
Yes. In Admin > Settings > VAT, you can edit individual rates. Manual rates
are marked with source "manual" and are not overwritten by automatic sync.

**Q: How does VAT work for digital goods?**
Digital goods sold to EU consumers are subject to the destination country's
VAT rate (MOSS/OSS rules). ForgeCommerce calculates VAT based on the
customer's country, which handles this correctly.

**Q: What about non-EU countries?**
ForgeCommerce currently supports EU member states only. Non-EU country
support is planned for a future release.
