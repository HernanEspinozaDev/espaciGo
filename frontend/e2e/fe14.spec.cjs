const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const { test, expect } = require('@playwright/test');

const accounts = JSON.parse(fs.readFileSync(process.env.FE14_ACCOUNT_FILE || '/run/fe14/accounts.json', 'utf8'));
const artifactDir = process.env.FE14_ARTIFACT_DIR || '/run/fe14/artifacts';
fs.mkdirSync(artifactDir, { recursive: true });

async function login(page, account) {
  await page.goto('/acceso');
  await page.getByLabel('Correo electrónico').fill(account.email);
  await page.getByLabel('Contraseña').fill(account.password);
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();
  await expect(page).toHaveURL(/\/mi-cuenta$/);
  await expect(page.getByRole('heading', { name: 'Tu información y tus derechos' })).toBeVisible();
  await expect(page.locator('.role-list')).toContainText(account.roleLabel);
  expect(await page.evaluate(() => [localStorage.length, sessionStorage.length])).toEqual([0, 0]);
}

async function navigateSpa(page, target) {
  await page.evaluate((url) => {
    history.pushState(null, '', url);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }, target);
}

async function takeScreenshot(page, name) {
  await expect(page.locator('body')).toBeVisible();
  const measurements = await page.evaluate(() => ({
    viewport: { width: innerWidth, height: innerHeight },
    document: { width: document.documentElement.scrollWidth, height: document.documentElement.scrollHeight },
  }));
  expect(measurements.document.width).toBeLessThanOrEqual(measurements.viewport.width);
  await page.screenshot({ path: path.join(artifactDir, `${name}.jpg`), type: 'jpeg', quality: 86 });
  fs.writeFileSync(path.join(artifactDir, `${name}.json`), JSON.stringify(measurements, null, 2));
}

test('Go-backed role navigation, private JSON/ZIP exports, and responsive evidence', async ({ browser }) => {
  const mobile = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 1 });
  await login(mobile, accounts.renter);
  await mobile.getByRole('button', { name: 'Abrir menú' }).click();
  await mobile.getByRole('link', { name: 'Explorar', exact: true }).click();
  await expect(mobile.getByRole('heading', { name: 'Encuentra tu próximo espacio' })).toBeVisible();
  await expect(mobile).toHaveURL(/\/explorar$/);
  await navigateSpa(mobile, '/administracion');
  await expect(mobile).toHaveURL(/\/mi-cuenta$/);
  await expect(mobile.getByRole('heading', { name: 'Tu información y tus derechos' })).toBeVisible();
  await navigateSpa(mobile, '/mi-cuenta/privacidad');
  await expect(mobile.getByRole('heading', { name: 'Tu información y tus derechos' })).toBeVisible();

  const jsonDownloadPromise = mobile.waitForEvent('download', { timeout: 20_000 });
  await mobile.getByRole('button', { name: 'Descargar JSON' }).click();
  const jsonDownload = await jsonDownloadPromise;
  expect(jsonDownload.suggestedFilename()).toBe('mis-datos-identidad.json');
  const jsonPath = await jsonDownload.path();
  expect(jsonPath).toBeTruthy();
  const exported = JSON.parse(fs.readFileSync(jsonPath, 'utf8'));
  expect(exported.scope).toBe('identidad_local_v1');
  expect(exported.account.email).toBe(accounts.renter.email);
  expect(JSON.stringify(exported).toLowerCase()).not.toContain('bearer');
  expect(JSON.stringify(exported).toLowerCase()).not.toContain('password');

  const zipDownloadPromise = mobile.waitForEvent('download', { timeout: 20_000 });
  await mobile.getByRole('button', { name: 'Descargar ZIP' }).click();
  const zipDownload = await zipDownloadPromise;
  expect(zipDownload.suggestedFilename()).toBe('mis-datos-espacigo.zip');
  const zipPath = await zipDownload.path();
  expect([...fs.readFileSync(zipPath).subarray(0, 4)]).toEqual([0x50, 0x4b, 0x03, 0x04]);
  const zipContents = JSON.parse(execFileSync('python3', ['-c', "import json,sys,zipfile; z=zipfile.ZipFile(sys.argv[1]); print(json.dumps({n: z.read(n).decode() for n in z.namelist() if n in ('manifest.json','data.json')}))", zipPath], { encoding: 'utf8' }));
  expect(Object.keys(zipContents).sort()).toEqual(['data.json', 'manifest.json']);
  expect(JSON.parse(zipContents['data.json']).identity.account.email).toBe(accounts.renter.email);
  expect(JSON.parse(zipContents['manifest.json']).schema_version).toBe(1);
  await expect(mobile.getByRole('status')).toContainText('Descargado ZIP');
  await navigateSpa(mobile, '/mi-cuenta');
  await mobile.keyboard.press('Tab');
  const focused = await mobile.evaluate(() => {
    const element = document.activeElement;
    return element ? { visible: !!(element.getClientRects().length), outlineStyle: getComputedStyle(element).outlineStyle, outlineWidth: getComputedStyle(element).outlineWidth } : null;
  });
  expect(focused?.visible).toBeTruthy();
  expect(Number.parseFloat(focused?.outlineWidth || '0')).toBeGreaterThan(0);
  await takeScreenshot(mobile, 'fe14_renter_mobile');
  await mobile.setViewportSize({ width: 1365, height: 900 });
  await takeScreenshot(mobile, 'fe14_renter_desktop');
  await mobile.close();

  const desktop = await browser.newPage({ viewport: { width: 1365, height: 900 }, deviceScaleFactor: 1 });
  await login(desktop, accounts.host);
  await expect(desktop.getByRole('link', { name: 'Anfitrión' })).toBeVisible();
  await desktop.getByRole('link', { name: 'Anfitrión', exact: true }).click();
  await expect(desktop.getByRole('heading', { name: 'Mis espacios' })).toBeVisible();
  await takeScreenshot(desktop, 'fe14_host_desktop');
  await desktop.setViewportSize({ width: 390, height: 844 });
  await takeScreenshot(desktop, 'fe14_host_mobile');
  await desktop.close();

  const admin = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 1 });
  await login(admin, accounts.admin);
  await admin.getByRole('button', { name: 'Abrir menú' }).click();
  await expect(admin.getByRole('link', { name: 'Administración', exact: true })).toBeVisible();
  await admin.getByRole('link', { name: 'Administración', exact: true }).click();
  await expect(admin.getByRole('heading', { name: /administraci/i })).toBeVisible();
  await takeScreenshot(admin, 'fe14_admin_mobile');
  await admin.setViewportSize({ width: 1365, height: 900 });
  await takeScreenshot(admin, 'fe14_admin_desktop');
  await admin.close();

  const thirdParty = await browser.newPage();
  await login(thirdParty, accounts.thirdParty);
  await navigateSpa(thirdParty, '/panel/espacios');
  await expect(thirdParty).toHaveURL(/\/mi-cuenta$/);
  await thirdParty.close();
});

test('Go outage preserves the same in-memory session and restoration revalidates it in place', async ({ browser }) => {
  const page = await browser.newPage({ viewport: { width: 1365, height: 900 } });
  await login(page, accounts.renter);
  const ready = path.join(artifactDir, 'outage-session-ready');
  const stopped = path.join(artifactDir, 'outage-stopped');
  const restored = path.join(artifactDir, 'outage-restored');
  fs.writeFileSync(ready, 'ready');
  const deadline = Date.now() + 120_000;
  while (!fs.existsSync(stopped) && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 250));
  expect(fs.existsSync(stopped)).toBeTruthy();

  await navigateSpa(page, '/mi-cuenta/verificaciones');
  await expect(page.getByRole('heading', { name: 'Casos de verificación' })).toBeVisible();
  await navigateSpa(page, '/mi-cuenta');
  // The same Angular document stays mounted through the outage: a network error
  // must not turn into a logout or persist the Bearer outside memory.
  await expect(page.getByRole('heading', { name: 'Tu información y tus derechos' })).toBeVisible();
  await expect(page.locator('.role-list')).toContainText(accounts.renter.roleLabel);
  await expect(page).not.toHaveURL(/\/acceso/);
  fs.writeFileSync(path.join(artifactDir, 'outage-verified'), 'verified');
  while (!fs.existsSync(restored) && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 250));
  expect(fs.existsSync(restored)).toBeTruthy();

  await navigateSpa(page, '/mi-cuenta/verificaciones');
  await expect(page.getByRole('heading', { name: 'Casos de verificación' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Elegibilidad' })).toBeVisible();
  await expect(page.getByRole('alert')).toHaveCount(0);
  expect(await page.evaluate(() => [localStorage.length, sessionStorage.length])).toEqual([0, 0]);
  await page.screenshot({ path: path.join(artifactDir, 'fe14_restored_session_desktop.jpg'), type: 'jpeg', quality: 86 });
  await page.close();
});
