const { test, expect } = require('@playwright/test');

const accountId = '5e7d9f2a-631a-4c82-9d26-791225d19412';
const termItems = require('./fixtures/consent-documents.cjs');

test('public landing reaches registration and EspaciGo login without a Google session or origin change', async ({ page, context, baseURL }) => {
  const publicOrigin = new URL(baseURL).origin;
  const navigatedOrigins = new Set();
  let registerRequestOrigin;
  let loginRequestOrigin;
  let verified = false;
  page.on('framenavigated', (frame) => {
    if (frame === page.mainFrame()) navigatedOrigins.add(new URL(frame.url()).origin);
  });
  await page.route('**/health/ready',r=>r.fulfill({json:{status:'ready'}}));
 await page.route('**/config.json', (route) => route.fulfill({ json: { apiBaseUrl: `${publicOrigin}/api/v1`, siteOrigin: publicOrigin } }));
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
    expect(route.request().postDataJSON()).not.toHaveProperty('use_preference');
    expect(route.request().postDataJSON()).not.toHaveProperty('roles');
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ account_id: accountId, status: 'email_pending' }),
    });
  });
  await page.route('**/api/v1/auth/verification', async (route) => {
    expect(route.request().postDataJSON()).toEqual({ token_id: accountId, token: 'synthetic-verification-token' });
    verified = true;
    await route.fulfill({ status: 204, body: '' });
  });
  await page.route('**/api/v1/auth/login', async (route) => {
    loginRequestOrigin = route.request().headers().origin;
    expect(verified).toBe(true);
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        account_id: accountId,
        roles: [],
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
      body: JSON.stringify({ account_id: accountId, roles: [], capabilities: [], restricted_mode: false }),
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
  await expect(page.getByRole('heading', { name: 'Bienvenido de nuevo' })).toBeVisible();
  if (process.env.PUBLIC_ENTRY_ACCESS_EVIDENCE_PATH) {
    await page.screenshot({ path: process.env.PUBLIC_ENTRY_ACCESS_EVIDENCE_PATH, fullPage: true });
  }

  await page.getByRole('link', { name: 'Crear cuenta' }).click();
  await expect(page).toHaveURL(`${publicOrigin}/registro`);
  await expect(page.getByRole('heading', { name: /Haz espacio/ })).toBeVisible();
  await expect(page.getByRole('link', {name:/Términos y condiciones/})).toBeVisible();
  if (process.env.PUBLIC_ENTRY_REGISTER_EVIDENCE_PATH) await page.screenshot({ path: process.env.PUBLIC_ENTRY_REGISTER_EVIDENCE_PATH, fullPage: true });
  await page.locator('#register-email').fill('visitor@example.invalid');
  await page.locator('#register-password').fill('Visitor#123');
  await page.locator('#register-confirm').fill('Visitor#123');
  await expect(page.getByRole('radio')).toHaveCount(0);
  await page.locator('.terms-fieldset input[type="checkbox"]').nth(0).check();
  await page.locator('.terms-fieldset input[type="checkbox"]').nth(1).check();
  await page.getByRole('button', { name: 'Crear cuenta' }).click();
  await expect(page.getByRole('heading',{name:'Revisa tu correo',exact:true})).toBeVisible();
  expect(registerRequestOrigin).toBe(publicOrigin);

  await page.goto('/verificar-correo#'+new URLSearchParams({token_id:accountId,token:'synthetic-verification-token'}));
  await expect(page).toHaveURL(`${publicOrigin}/verificar-correo`);
  await page.getByRole('button', { name: 'Verificar correo', exact: true }).click();
  await expect(page).toHaveURL(`${publicOrigin}/acceso?verificado=1`);
  await expect(page.getByRole('status')).toContainText('Correo verificado. Ya puedes iniciar sesión.');
  await page.locator('#login-email').fill('visitor@example.invalid');
  await page.locator('#login-password').fill('Visitor#123');
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();
  await expect(page).toHaveURL(`${publicOrigin}/mi-cuenta`);
  expect(loginRequestOrigin).toBe(publicOrigin);
  await expect(page.getByRole('link', { name: 'Ofrecer espacio', exact: true })).toBeVisible();
  expect([...navigatedOrigins]).toEqual([publicOrigin]);
  expect(page.url()).not.toContain('accounts.google.com');
  expect(await context.cookies()).toEqual([]);
  if (process.env.PUBLIC_ENTRY_EVIDENCE_PATH) {
    await page.screenshot({ path: process.env.PUBLIC_ENTRY_EVIDENCE_PATH, fullPage: true });
  }
});
