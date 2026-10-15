const fs = require('node:fs');
const path = require('node:path');
const { test, expect } = require('@playwright/test');

const accounts = JSON.parse(fs.readFileSync(process.env.FE14_ACCOUNT_FILE || '/run/fe14/accounts.json', 'utf8'));
const artifacts = process.env.FE14_ARTIFACT_DIR || '/run/fe14/artifacts';
const head = process.env.FE14_HEAD || 'unknown';

test('GADM-02 serves account publication management through Angular, Go, and disposable PostGIS', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/acceso');
  await page.getByLabel('Correo electrónico').fill(accounts.admin.email);
  await page.getByLabel('Contraseña').fill(accounts.admin.password);
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
