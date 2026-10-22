const { test, expect } = require('@playwright/test');

const accountId = '5e7d9f2a-631a-4c82-9d26-791225d19412';
const termItems = [
  { id: '00000000-0000-4000-8000-000000000001', code: 'terminos-v1', type: 'terminos', sha256: 'a'.repeat(64), published_at: '2026-01-01T00:00:00Z', synthetic: true },
  { id: '00000000-0000-4000-8000-000000000002', code: 'privacidad-v1', type: 'privacidad', sha256: 'b'.repeat(64), published_at: '2026-01-01T00:00:00Z', synthetic: true },
];

test('public landing reaches registration and EspaciGo login without a Google session or origin change', async ({ page, context, baseURL }) => {
  const publicOrigin = new URL(baseURL).origin;
  const navigatedOrigins = new Set();
  let registerRequestOrigin;
  let loginRequestOrigin;
  page.on('framenavigated', (frame) => {
    if (frame === page.mainFrame()) navigatedOrigins.add(new URL(frame.url()).origin);
  });
  await context.clearCookies();
  await page.addInitScript(() => {
    localStorage.clear();
    sessionStorage.clear();
  });
  await page.route('**/api/v1/auth/terms', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ items: termItems }),
  }));
  await page.route('**/api/v1/auth/register', async (route) => {
    registerRequestOrigin = route.request().headers().origin;
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ account_id: accountId, status: 'email_pending', use_preference: 'arrendar' }),
    });
  });
  await page.route('**/api/v1/auth/login', async (route) => {
    loginRequestOrigin = route.request().headers().origin;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        account_id: accountId,
        roles: ['arrendatario'],
        capabilities: [],
        access_token: 'synthetic-espacigo-application-bearer-token',
        token_type: 'Bearer',
        expires_at: '2027-01-01T00:00:00Z',
        restricted_mode: false,
      }),
    });
  });
  await page.route('**/api/v1/auth/session', async (route) => {
    expect(route.request().headers().authorization).toBe('Bearer synthetic-espacigo-application-bearer-token');
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ account_id: accountId, roles: ['arrendatario'], capabilities: [], restricted_mode: false }),
    });
  });
  await page.route('**/api/v1/profile', async (route) => {
    expect(route.request().headers().authorization).toBe('Bearer synthetic-espacigo-application-bearer-token');
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ display_name: 'Visitante de prueba', updated_at: '2026-10-22T00:00:00Z' }),
    });
  });
  await page.route('**/api/v1/profile/photo', (route) => route.fulfill({ status: 404, body: '' }));
  await page.route('**/api/v1/payout-account', (route) => route.fulfill({ status: 404, body: '' }));
  await page.route('**/api/v1/rights-requests', async (route) => {
    expect(route.request().headers().authorization).toBe('Bearer synthetic-espacigo-application-bearer-token');
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [] }),
    });
  });

  await page.goto('/');
  await expect(page.getByRole('link', { name: /iniciar sesión/i }).first()).toBeVisible();
  await page.getByRole('link', { name: /iniciar sesión/i }).first().click();
  await expect(page).toHaveURL(`${publicOrigin}/acceso`);
  await expect(page.getByRole('heading', { name: 'Inicia sesión' })).toBeVisible();
  if (process.env.PUBLIC_ENTRY_ACCESS_EVIDENCE_PATH) {
    await page.screenshot({ path: process.env.PUBLIC_ENTRY_ACCESS_EVIDENCE_PATH, fullPage: true });
  }

  await page.getByRole('link', { name: 'Crear cuenta' }).click();
  await expect(page).toHaveURL(`${publicOrigin}/registro`);
  await expect(page.getByRole('heading', { name: 'Crea tu cuenta' })).toBeVisible();
  await expect(page.getByText(/terminos-v1/)).toBeVisible();
  await page.locator('#register-email').fill('visitor@example.invalid');
  await page.locator('#register-password').fill('Visitor#123');
  await page.locator('#register-confirm').fill('Visitor#123');
  await page.getByLabel('Arrendar un espacio').check();
  await page.locator('.terms-fieldset input[type="checkbox"]').nth(0).check();
  await page.locator('.terms-fieldset input[type="checkbox"]').nth(1).check();
  await page.getByRole('button', { name: 'Crear cuenta' }).click();
  await expect(page.getByRole('status')).toContainText('Go creó la cuenta');
  expect(registerRequestOrigin).toBe(publicOrigin);

  await page.getByRole('link', { name: 'Inicia sesión' }).click();
  await expect(page).toHaveURL(`${publicOrigin}/acceso`);
  await page.locator('#login-email').fill('visitor@example.invalid');
  await page.locator('#login-password').fill('Visitor#123');
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();
  await expect(page).toHaveURL(`${publicOrigin}/mi-cuenta`);
  expect(loginRequestOrigin).toBe(publicOrigin);
  expect([...navigatedOrigins]).toEqual([publicOrigin]);
  expect(page.url()).not.toContain('accounts.google.com');
  expect(await context.cookies()).toEqual([]);
  if (process.env.PUBLIC_ENTRY_EVIDENCE_PATH) {
    await page.screenshot({ path: process.env.PUBLIC_ENTRY_EVIDENCE_PATH, fullPage: true });
  }
});
