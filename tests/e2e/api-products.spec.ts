import { test, expect } from '@playwright/test';

const API_URL = process.env.API_URL || 'http://localhost:8080';

test.describe('API Products @critical', () => {
  test('GET /api/v1/products returns 200 with pagination', async ({ request }) => {
    const response = await request.get(API_URL + '/api/v1/products');
    expect(response.status()).toBe(200);
    const body = await response.json();
    // Should have pagination structure
    expect(body).toHaveProperty('data');
    expect(body).toHaveProperty('total');
    expect(body).toHaveProperty('total_pages');
    expect(Array.isArray(body.data)).toBeTruthy();
  });

  test('GET /api/v1/products supports page parameter', async ({ request }) => {
    const response = await request.get(API_URL + '/api/v1/products?page=1&limit=10');
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body).toHaveProperty('data');
    expect(Array.isArray(body.data)).toBeTruthy();
  });

  test('GET /api/v1/categories returns 200', async ({ request }) => {
    const response = await request.get(API_URL + '/api/v1/categories');
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(Array.isArray(body)).toBeTruthy();
  });

  test('GET /api/v1/products/:slug returns product detail', async ({ request }) => {
    // First get the product list to find a slug
    const listResponse = await request.get(API_URL + '/api/v1/products');
    expect(listResponse.status()).toBe(200);
    const listBody = await listResponse.json();

    if (listBody.data && listBody.data.length > 0) {
      const slug = listBody.data[0].slug;
      const detailResponse = await request.get(API_URL + `/api/v1/products/${slug}`);
      expect(detailResponse.status()).toBe(200);
      const product = await detailResponse.json();
      expect(product).toHaveProperty('id');
      expect(product).toHaveProperty('name');
      expect(product).toHaveProperty('slug');
      expect(product.slug).toBe(slug);
    }
  });

  test('GET /api/v1/products/:slug returns 404 for nonexistent product', async ({ request }) => {
    const response = await request.get(API_URL + '/api/v1/products/nonexistent-product-slug-999');
    expect(response.status()).toBe(404);
  });

  test('product data has expected fields', async ({ request }) => {
    const response = await request.get(API_URL + '/api/v1/products');
    expect(response.status()).toBe(200);
    const body = await response.json();

    if (body.data && body.data.length > 0) {
      const product = body.data[0];
      expect(product).toHaveProperty('id');
      expect(product).toHaveProperty('name');
      expect(product).toHaveProperty('slug');
      expect(product).toHaveProperty('base_price');
      expect(product).toHaveProperty('status');
    }
  });
});
