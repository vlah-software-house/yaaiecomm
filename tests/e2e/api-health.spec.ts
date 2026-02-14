import { test, expect } from '@playwright/test';

const ADMIN_URL = process.env.ADMIN_URL || 'http://localhost:8081';
const API_URL = process.env.API_URL || 'http://localhost:8080';

test.describe('API Health Checks', () => {
  test('admin health check returns ok', async ({ request }) => {
    const response = await request.get(ADMIN_URL + '/admin/health');
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.status).toBe('ok');
  });

  test('API health check returns ok', async ({ request }) => {
    const response = await request.get(API_URL + '/api/v1/health');
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.status).toBe('ok');
  });

  test('countries endpoint returns array', async ({ request }) => {
    const response = await request.get(API_URL + '/api/v1/countries');
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(Array.isArray(body)).toBeTruthy();
  });

  test('countries have expected fields', async ({ request }) => {
    const response = await request.get(API_URL + '/api/v1/countries');
    expect(response.status()).toBe(200);
    const body = await response.json();
    if (body.length > 0) {
      expect(body[0]).toHaveProperty('country_code');
      expect(body[0]).toHaveProperty('name');
    }
  });
});
