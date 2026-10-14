const fs = require('node:fs');
const path = require('node:path');
const { test, expect } = require('@playwright/test');

const accounts = JSON.parse(fs.readFileSync(process.env.FE14_ACCOUNT_FILE || '/run/fe14/accounts.json', 'utf8'));
const artifactDir = process.env.FE14_ARTIFACT_DIR || '/run/fe14/artifacts';
fs.mkdirSync(artifactDir, { recursive: true });

async function accessToken(request, account) {
  const response = await request.post('http://backend:8080/api/v1/auth/login', {
    data: { email: account.email, password: account.password },
  });
  expect(response.status()).toBe(200);
  return (await response.json()).access_token;
}

async function signIn(page, account) {
  await page.goto('/acceso');
  await page.getByLabel('Correo electrónico').fill(account.email);
  await page.getByLabel('Contraseña').fill(account.password);
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();
  await expect(page).toHaveURL(/\/mi-cuenta$/);
}

async function navigateSpa(page, target) {
  await page.evaluate((url) => {
    history.pushState(null, '', url);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }, target);
}

async function takeEvidenceScreenshot(page, name) {
  const measurements = await page.evaluate(() => ({
    viewport: { width: innerWidth, height: innerHeight },
    document: { width: document.documentElement.scrollWidth, height: document.documentElement.scrollHeight },
  }));
  measurements.head = process.env.FE14_HEAD || 'unknown';
  expect(measurements.document.width).toBeLessThanOrEqual(measurements.viewport.width);
  await page.screenshot({ path: path.join(artifactDir, `${name}.jpg`), type: 'jpeg', quality: 90, fullPage: true });
  fs.writeFileSync(path.join(artifactDir, `${name}.json`), JSON.stringify(measurements, null, 2));
}

function contrastRatio(foreground, background) {
  const luminance = (color) => {
    const channels = color.match(/\d+/g).slice(0, 3).map((value) => Number(value) / 255);
    const linear = channels.map((value) => value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4);
    return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
  };
  const values = [luminance(foreground), luminance(background)].sort((a, b) => b - a);
  return (values[0] + 0.05) / (values[1] + 0.05);
}

test('LOCAL-CLOSE-01 deletes only the owner fixture through Angular→Go→PostgreSQL', async ({ page, request }) => {
  const owner = accounts.renter;
  const thirdParty = accounts.thirdParty;
  const ownerToken = await accessToken(request, owner);
  const thirdPartyToken = await accessToken(request, thirdParty);
  const headers = { Authorization: `Bearer ${ownerToken}` };
  const caseResponse = await request.post('http://backend:8080/api/v1/verifications', {
    headers: { ...headers, 'Idempotency-Key': `local-close-01-${crypto.randomUUID()}` },
    data: { type: 'kyb' },
  });
  expect(caseResponse.status()).toBe(201);
  const verification = await caseResponse.json();
  const evidenceResponse = await request.post(
    `http://backend:8080/api/v1/verifications/${verification.id}/evidence`,
    { headers, data: { fixture_code: 'synthetic-png-v1' } },
  );
  expect(evidenceResponse.status()).toBe(201);
  const evidence = await evidenceResponse.json();
  const evidencePath = `/api/v1/verifications/${verification.id}/evidence/${evidence.id}`;

  const foreignDelete = await request.delete(`http://backend:8080${evidencePath}`, {
    headers: { Authorization: `Bearer ${thirdPartyToken}` },
  });
  expect(foreignDelete.status()).toBe(404);
  const stillOwned = await request.get(
    `http://backend:8080/api/v1/verifications/${verification.id}/evidence`,
    { headers },
  );
  expect(stillOwned.status()).toBe(200);
  expect((await stillOwned.json()).items.map((item) => item.id)).toContain(evidence.id);

  await signIn(page, owner);
  await navigateSpa(page, '/mi-cuenta/verificaciones');
  await expect(page.getByRole('heading', { name: 'Casos de verificación' })).toBeVisible();
  await expect(page.getByText('synthetic-png-v1')).toBeVisible();
  const deleteResponse = page.waitForResponse(
    (response) => response.request().method() === 'DELETE' && response.url().endsWith(evidencePath),
  );
  const click = page.getByRole('button', { name: 'Retirar mi evidencia' }).click();
  const dialog = await page.waitForEvent('dialog');
  expect(dialog.type()).toBe('confirm');
  expect(dialog.message()).toContain('no elimina el volumen local');
  await dialog.accept();
  expect((await deleteResponse).status()).toBe(204);
  await click;
  await expect(page.getByText('Este caso todavía no tiene evidencia sintética.')).toBeVisible();

  const repeatedDelete = await request.delete(`http://backend:8080${evidencePath}`, { headers });
  expect(repeatedDelete.status()).toBe(404);
  const afterDelete = await request.get(
    `http://backend:8080/api/v1/verifications/${verification.id}/evidence`,
    { headers },
  );
  expect((await afterDelete.json()).items).toEqual([]);
  expect(await page.evaluate(() => [localStorage.length, sessionStorage.length])).toEqual([0, 0]);
});

test('LOCAL-CLOSE-01 missing covers use a solid high-contrast placeholder', async ({ page, request }) => {
  await page.setViewportSize({ width: 1365, height: 900 });
  const catalog = await request.get('http://backend:8080/api/v1/public/spaces?page_size=24');
  expect(catalog.status()).toBe(200);
  expect((await catalog.json()).items.length).toBeGreaterThan(0);

  await page.goto('/explorar?page_size=24');
  const placeholder = page.getByRole('img', { name: 'Sin fotografía pública' }).first();
  await expect(placeholder).toBeVisible();
  const visual = await placeholder.evaluate((element) => {
    const label = element.querySelector('span');
    const category = element.querySelector('b');
    const styles = getComputedStyle(element);
    return {
      backgroundColor: styles.backgroundColor,
      backgroundImage: styles.backgroundImage,
      labelColor: getComputedStyle(label).color,
      categoryColor: getComputedStyle(category).color,
      labelOpacity: getComputedStyle(label).opacity,
      categoryOpacity: getComputedStyle(category).opacity,
      label: label.textContent,
      category: category.textContent,
    };
  });
  expect(visual.backgroundImage).toBe('none');
  expect(visual.labelOpacity).toBe('1');
  expect(visual.categoryOpacity).toBe('1');
  const labelContrast = contrastRatio(visual.labelColor, visual.backgroundColor);
  const categoryContrast = contrastRatio(visual.categoryColor, visual.backgroundColor);
  expect(labelContrast).toBeGreaterThanOrEqual(4.5);
  expect(categoryContrast).toBeGreaterThanOrEqual(4.5);
  const evidence = {
    head: process.env.FE14_HEAD || 'unknown',
    viewport: { width: 1365, height: 900 },
    colors: visual,
    contrastRatio: { label: labelContrast, category: categoryContrast },
    method: 'Computed solid CSS token colors; WCAG relative luminance ratio, no image sampling.',
  };
  fs.writeFileSync(path.join(artifactDir, 'fe14_local_close_01_placeholder_contrast.json'), JSON.stringify(evidence, null, 2));
  await takeEvidenceScreenshot(page, 'fe14_local_close_01_placeholder_desktop');

  await page.setViewportSize({ width: 390, height: 844 });
  await expect(placeholder).toBeVisible();
  await takeEvidenceScreenshot(page, 'fe14_local_close_01_placeholder_mobile');
});
