const fs = require('node:fs');
const path = require('node:path');
const { test, expect } = require('@playwright/test');

const accounts = JSON.parse(fs.readFileSync(process.env.FE14_ACCOUNT_FILE || '/run/fe14/accounts.json', 'utf8'));
const artifacts = process.env.FE14_ARTIFACT_DIR || '/run/fe14/artifacts';
const head = process.env.FE14_HEAD || 'unknown';

test('GADM-02 serves account publication management through Angular, Go, and disposable PostGIS', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/acceso');
  await page.getByLabel('Correo electrónico').fill(accounts.globalAdmin.email);
  await page.getByLabel('Contraseña').fill(accounts.globalAdmin.password);
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();
  await expect(page).toHaveURL(/\/mi-cuenta$/);
  await page.getByRole('button', { name: 'Abrir menú' }).click();
  await page.getByRole('link', { name: 'Administración global' }).click();
  await expect(page.getByRole('heading', { name: 'Directorio de cuentas' })).toBeVisible();
  await page.getByRole('button', { name: 'Publicaciones' }).click();
  await expect(page.getByRole('heading', { name: 'Inventario global de publicaciones' })).toBeVisible();
  await page.getByLabel('Texto o correo de propietario').fill('Espacio sintético público FE-14');
  await page.getByRole('button', { name: 'Buscar' }).click();
  const row = page.locator('tr').filter({ hasText: 'Espacio sintético público FE-14' });
  await expect(row).toBeVisible();
  await row.getByRole('button', { name: 'Galería' }).click();
  await expect(page.getByRole('heading', { name: /Galería · Espacio sintético público FE-14/ })).toBeVisible();
  await expect(page.getByText('synthetic-png-v1')).toBeVisible();
  await row.getByRole('button', { name: 'Horario' }).click();
  await expect(page.getByRole('heading', { name: /Horario semanal · Espacio sintético público FE-14/ })).toBeVisible();
  await expect(page.getByText('Zona horaria: America/Santiago.')).toBeVisible();
  await page.getByRole('heading', { name: /Horario semanal · Espacio sintético público FE-14/ }).scrollIntoViewIfNeeded();

  const redactOwnerEmail = async () => page.locator('tbody td').evaluateAll((items) => {
    for (const item of items) if (item.textContent?.includes('@')) item.textContent = '[correo de ensayo oculto]';
  });
  await redactOwnerEmail();
  const mobileMeasures = await page.evaluate((commit) => ({ viewport: { width: innerWidth, height: innerHeight }, document: { width: document.documentElement.scrollWidth, height: document.documentElement.scrollHeight }, head: commit }), head);
  expect(mobileMeasures.document.width).toBeLessThanOrEqual(390);
  await page.screenshot({ path: path.join(artifacts, 'fe14_gadm_02_admin_mobile.jpg'), type: 'jpeg', quality: 86 });
  fs.writeFileSync(path.join(artifacts, 'fe14_gadm_02_admin_mobile.json'), JSON.stringify(mobileMeasures, null, 2));

  await page.setViewportSize({ width: 1365, height: 900 });
  const desktopMeasures = await page.evaluate((commit) => ({ viewport: { width: innerWidth, height: innerHeight }, document: { width: document.documentElement.scrollWidth, height: document.documentElement.scrollHeight }, head: commit }), head);
  expect(desktopMeasures.document.width).toBeLessThanOrEqual(1365);
  await page.screenshot({ path: path.join(artifacts, 'fe14_gadm_02_admin_desktop.jpg'), type: 'jpeg', quality: 86 });
  fs.writeFileSync(path.join(artifacts, 'fe14_gadm_02_admin_desktop.json'), JSON.stringify(desktopMeasures, null, 2));
});

test('GADM-02 persists account, gallery, schedule, and retirement changes through the real admin UI', async ({ page }) => {
  const api = process.env.FE14_API_BASE_URL || 'http://backend:8080/api/v1';
  const login = async (account) => {
    const response = await page.request.post(`${api}/auth/login`, { data: { email: account.email, password: account.password } });
    expect(response.status()).toBe(200);
    return (await response.json()).access_token;
  };
  const mutation = (fragment, method) => page.waitForResponse((response) => response.url().includes(fragment) && response.request().method() === method);
  const adminToken = await login(accounts.globalAdmin);
  const thirdPartyToken = await login(accounts.thirdParty);
  const denied = await page.request.get(`${api}/admin/global/accounts`, { headers: { Authorization: `Bearer ${thirdPartyToken}` } });
  expect(denied.status()).toBe(403);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/acceso');
  await page.getByLabel('Correo electrónico').fill(accounts.globalAdmin.email);
  await page.getByLabel('Contraseña').fill(accounts.globalAdmin.password);
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();
  await expect(page).toHaveURL(/\/mi-cuenta$/);
  await page.getByRole('button', { name: 'Abrir menú' }).click();
  await page.getByRole('link', { name: 'Administración global' }).click();
  await expect(page.getByRole('heading', { name: 'Directorio de cuentas' })).toBeVisible();

  await page.getByLabel('Correo comienza con').fill(accounts.renter.email);
  await page.getByRole('button', { name: 'Buscar' }).click();
  await page.getByRole('button', { name: new RegExp(accounts.renter.email) }).click();
  const profile = page.getByRole('heading', { name: 'Editar perfil permitido' }).locator('..');
  await profile.getByLabel('Nombre visible').fill('Arrendatario Actualizado');
  await profile.getByLabel('Teléfono').fill('987654321');
  let responsePromise = mutation('/admin/global/accounts/', 'PATCH');
  await profile.getByRole('button', { name: 'Guardar cambios' }).click();
  expect((await responsePromise).status()).toBe(200);
  await expect(page.getByText('Arrendatario Actualizado').first()).toBeVisible();

  const roles = page.getByRole('region', { name: 'Roles de marketplace' });
  responsePromise = mutation('/admin/global/accounts/', 'PUT');
  await roles.getByRole('button', { name: 'Conceder' }).click();
  expect((await responsePromise).ok()).toBeTruthy();
  await expect(roles).toContainText('arrendador: concedido');

  const renterToken = await login(accounts.renter);
  const sessionBefore = await page.request.get(`${api}/auth/session`, { headers: { Authorization: `Bearer ${renterToken}` } });
  expect(sessionBefore.status()).toBe(200);
  await page.getByRole('button', { name: 'Revocar todas las sesiones de esta cuenta' }).click();
  responsePromise = mutation('/sessions/revoke', 'POST');
  await page.getByRole('button', { name: 'Confirmar revocación' }).click();
  expect((await responsePromise).ok()).toBeTruthy();
  expect((await page.request.get(`${api}/auth/session`, { headers: { Authorization: `Bearer ${renterToken}` } })).status()).toBe(401);

  responsePromise = mutation('/admin/global/accounts/', 'POST');
  await page.getByRole('button', { name: 'Bloquear cuenta y revocar sesiones' }).click();
  expect((await responsePromise).ok()).toBeTruthy();
  await expect(page.getByText('bloqueada administrativamente')).toBeVisible();
  responsePromise = mutation('/admin/global/accounts/', 'POST');
  await page.getByRole('button', { name: 'Desbloquear cuenta' }).click();
  expect((await responsePromise).ok()).toBeTruthy();
  await expect(page.getByText('sin bloqueo administrativo')).toBeVisible();
  await page.getByRole('button', { name: 'Cerrar detalle' }).click();

  await page.getByRole('button', { name: 'Publicaciones' }).click();
  const searchPublications = async (title) => {
    await page.getByLabel('Texto o correo de propietario').fill(title);
    await page.getByRole('button', { name: 'Buscar' }).last().click();
    const row = page.locator('tr').filter({ hasText: title }).first();
    await expect(row).toBeVisible();
    return row;
  };
  let row = await searchPublications('Espacio sintético público FE-14');
  responsePromise = mutation('/content', 'PATCH');
  await row.getByRole('button', { name: 'Editar' }).click();
  await page.getByLabel('Título').fill('Espacio sintético público FE-14 editado');
  await page.getByLabel('Wi-Fi').uncheck();
  await page.getByRole('button', { name: 'Guardar edición' }).click();
  expect((await responsePromise).status()).toBe(200);
  await expect(page.getByText('Espacio sintético público FE-14 editado').first()).toBeVisible();

  row = await searchPublications('Espacio sintético público FE-14 editado');
  responsePromise = mutation('/visibility', 'PATCH');
  await row.getByRole('button', { name: 'Galería' }).click();
  const gallery = page.getByRole('region', { name: /Galería/ });
  await expect(gallery.getByText('synthetic-png-v1')).toBeVisible();
  await gallery.getByRole('button', { name: 'Mostrar foto' }).click();
  expect((await responsePromise).ok()).toBeTruthy();
  await expect(gallery.getByText('Visible en catálogo')).toBeVisible();

  row = await searchPublications('Horario FE14');
  responsePromise = mutation('/schedule', 'PUT');
  await row.getByRole('button', { name: 'Horario' }).click();
  const schedule = page.getByRole('region', { name: /Horario semanal/ });
  await expect(schedule).toContainText('Zona horaria: America/Santiago.');
  await schedule.getByLabel('Horario habilitado').check();
  await schedule.getByLabel('Tramos de Lunes').fill('09:00-12:00');
  await schedule.getByRole('button', { name: 'Guardar horario' }).click();
  expect((await responsePromise).ok()).toBeTruthy();
  await expect(schedule.getByLabel('Tramos de Lunes')).toHaveValue('09:00-12:00');

  row = await searchPublications('Retiro con obligación FE-14');
  await row.getByRole('button', { name: 'Retirar' }).click();
  responsePromise = mutation('/retire', 'POST');
  await page.getByRole('button', { name: 'Confirmar retiro' }).click();
  const blockedRetirement = await responsePromise;
  const blockedRetirementBody = await blockedRetirement.json();
  expect(blockedRetirement.status(), `blocked retirement error code: ${blockedRetirementBody.code || blockedRetirementBody.error?.code || 'unknown'}`).toBe(409);
  await expect(page.getByRole('alert')).toBeVisible();
  await page.getByRole('button', { name: 'Cancelar' }).last().click();

  row = await searchPublications('Retiro permitido FE-14');
  await row.getByRole('button', { name: 'Retirar' }).click();
  responsePromise = mutation('/retire', 'POST');
  await page.getByRole('button', { name: 'Confirmar retiro' }).click();
  expect((await responsePromise).status()).toBe(200);

  const audited = await page.request.get(`${api}/admin/global/accounts`, { headers: { Authorization: `Bearer ${adminToken}` } });
  expect(audited.status()).toBe(200);
});
