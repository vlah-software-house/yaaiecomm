import { test, expect } from '@playwright/test';

const API_URL = process.env.API_URL || 'http://localhost:8080';

test.describe('API Cart @critical', () => {
  test('POST /api/v1/cart creates a new cart', async ({ request }) => {
    const response = await request.post(API_URL + '/api/v1/cart');
    expect(response.status()).toBe(201);
    const body = await response.json();
    expect(body).toHaveProperty('id');
    expect(body.id).toBeTruthy();
  });

  test('GET /api/v1/cart/:id retrieves the cart', async ({ request }) => {
    // Create a cart first
    const createResponse = await request.post(API_URL + '/api/v1/cart');
    expect(createResponse.status()).toBe(201);
    const cart = await createResponse.json();

    // Get the cart
    const getResponse = await request.get(API_URL + `/api/v1/cart/${cart.id}`);
    expect(getResponse.status()).toBe(200);
    const body = await getResponse.json();
    expect(body).toHaveProperty('id');
    expect(body.id).toBe(cart.id);
  });

  test('POST /api/v1/cart/:id/items adds item to cart', async ({ request }) => {
    // Create a cart
    const createResponse = await request.post(API_URL + '/api/v1/cart');
    expect(createResponse.status()).toBe(201);
    const cart = await createResponse.json();

    // Get a product variant to add
    const productsResponse = await request.get(API_URL + '/api/v1/products');
    const products = await productsResponse.json();

    if (products.data && products.data.length > 0) {
      const slug = products.data[0].slug;
      const productResponse = await request.get(API_URL + `/api/v1/products/${slug}`);
      const product = await productResponse.json();

      if (product.variants && product.variants.length > 0) {
        const variantId = product.variants[0].id;
        const addResponse = await request.post(API_URL + `/api/v1/cart/${cart.id}/items`, {
          data: { variant_id: variantId, quantity: 1 },
        });
        // Should succeed (200 or 201)
        expect([200, 201]).toContain(addResponse.status());
        const updatedCart = await addResponse.json();
        expect(updatedCart).toHaveProperty('items');
      }
    }
  });

  test('PATCH /api/v1/cart/:id/items/:itemId updates quantity', async ({ request }) => {
    // Create cart and add an item
    const createResponse = await request.post(API_URL + '/api/v1/cart');
    const cart = await createResponse.json();

    const productsResponse = await request.get(API_URL + '/api/v1/products');
    const products = await productsResponse.json();

    if (products.data && products.data.length > 0) {
      const slug = products.data[0].slug;
      const productResponse = await request.get(API_URL + `/api/v1/products/${slug}`);
      const product = await productResponse.json();

      if (product.variants && product.variants.length > 0) {
        const variantId = product.variants[0].id;
        const addResponse = await request.post(API_URL + `/api/v1/cart/${cart.id}/items`, {
          data: { variant_id: variantId, quantity: 1 },
        });

        if (addResponse.ok()) {
          const updatedCart = await addResponse.json();
          if (updatedCart.items && updatedCart.items.length > 0) {
            const itemId = updatedCart.items[0].id;
            const patchResponse = await request.patch(
              API_URL + `/api/v1/cart/${cart.id}/items/${itemId}`,
              { data: { quantity: 3 } }
            );
            expect(patchResponse.ok()).toBeTruthy();
            const patchedCart = await patchResponse.json();
            const item = patchedCart.items?.find((i: { id: string }) => i.id === itemId);
            if (item) {
              expect(item.quantity).toBe(3);
            }
          }
        }
      }
    }
  });

  test('DELETE /api/v1/cart/:id/items/:itemId removes item', async ({ request }) => {
    // Create cart and add an item
    const createResponse = await request.post(API_URL + '/api/v1/cart');
    const cart = await createResponse.json();

    const productsResponse = await request.get(API_URL + '/api/v1/products');
    const products = await productsResponse.json();

    if (products.data && products.data.length > 0) {
      const slug = products.data[0].slug;
      const productResponse = await request.get(API_URL + `/api/v1/products/${slug}`);
      const product = await productResponse.json();

      if (product.variants && product.variants.length > 0) {
        const variantId = product.variants[0].id;
        const addResponse = await request.post(API_URL + `/api/v1/cart/${cart.id}/items`, {
          data: { variant_id: variantId, quantity: 1 },
        });

        if (addResponse.ok()) {
          const updatedCart = await addResponse.json();
          if (updatedCart.items && updatedCart.items.length > 0) {
            const itemId = updatedCart.items[0].id;
            const deleteResponse = await request.delete(
              API_URL + `/api/v1/cart/${cart.id}/items/${itemId}`
            );
            expect(deleteResponse.ok()).toBeTruthy();
          }
        }
      }
    }
  });

  test('GET /api/v1/cart/:id returns 404 for nonexistent cart', async ({ request }) => {
    const response = await request.get(
      API_URL + '/api/v1/cart/00000000-0000-0000-0000-000000000000'
    );
    expect(response.status()).toBe(404);
  });
});
