const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const { test, expect } = require('@playwright/test');

const accounts = JSON.parse(fs.readFileSync(process.env.FE14_ACCOUNT_FILE || '/run/fe14/accounts.json', 'utf8'));
const artifactDir = process.env.FE14_ARTIFACT_DIR || '/run/fe14/artifacts';
fs.mkdirSync(artifactDir, { recursive: true });
const resultsDir = process.env.FE14_RESULTS_DIR || '/run/fe14/results';
const resultsRoot = path.dirname(resultsDir);

async function waitForMarker(marker, timeoutMs = 30_000) {
  const deadline = Date.now() + timeoutMs;
  while (!fs.existsSync(marker) && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  expect(fs.existsSync(marker)).toBeTruthy();
}

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

async function takeScreenshot(page, name, fullPage = false) {
  await expect(page.locator('body')).toBeVisible();
  const measurements = await page.evaluate(() => ({
    viewport: { width: innerWidth, height: innerHeight },
    document: { width: document.documentElement.scrollWidth, height: document.documentElement.scrollHeight },
  }));
  expect(measurements.document.width).toBeLessThanOrEqual(measurements.viewport.width);
  await page.screenshot({ path: path.join(artifactDir, `${name}.jpg`), type: 'jpeg', quality: 86, fullPage });
  fs.writeFileSync(path.join(artifactDir, `${name}.json`), JSON.stringify(measurements, null, 2));
}

test('E2E runner can read only the accounts fixture and write artifacts/results, never backend secrets', async () => {
  const accountFile = process.env.FE14_ACCOUNT_FILE || '/run/fe14/accounts.json';
  expect(fs.existsSync(accountFile)).toBeTruthy();
  expect(() => fs.accessSync(accountFile, fs.constants.R_OK)).not.toThrow();
  expect(() => fs.accessSync(accountFile, fs.constants.W_OK)).toThrow();
  expect(() => fs.accessSync(artifactDir, fs.constants.W_OK)).not.toThrow();
  expect(() => fs.accessSync(resultsRoot, fs.constants.W_OK)).not.toThrow();
  expect(fs.readdirSync('/run/fe14').sort()).toEqual(['accounts.json', 'artifacts', 'results']);
  expect(process.env.LOCAL_DOCKER_STATE_DIR).toBeUndefined();

  for (const forbiddenPath of [
    '/run/fe14/secrets',
    '/run/fe14/private-evidence',
    '/run/fe14/privacy-replay',
    '/run/fe14/database-tls',
    '/run/secrets',
    '/run/secrets/db_admin_password',
    '/run/secrets/runtime_password',
    '/run/secrets/local_payment_webhook_secret',
    '/run/secrets/local_contract_encryption_key',
  ]) {
    expect(fs.existsSync(forbiddenPath)).toBe(false);
    expect(() => fs.accessSync(forbiddenPath, fs.constants.R_OK)).toThrow();
  }
});

test('Anonymous visitor explores the Go-backed public catalog and minimized detail', async ({ page }) => {
  await page.setViewportSize({ width: 1365, height: 900 });
  let publicRequest;
  page.on('request', request => {
    if (request.method() === 'GET' && /\/api\/v1\/public\/spaces(?:\?|$)/.test(request.url())) publicRequest = request;
  });
  await page.goto('/explorar');
  await expect(page.getByRole('heading', { name: 'Encuentra tu próximo espacio' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  expect(publicRequest).toBeTruthy();
  expect((await publicRequest.allHeaders()).authorization).toBeUndefined();

  // Follow the rendered controls into the real Go API. The public item has a
  // deliberately assigned synthetic zone and boolean attribute in this stack.
  await page.getByLabel('Buscar', { exact: true }).fill('publico providencia');
  await page.locator('.filters label').filter({ hasText: /^Categoría/ }).locator('select').selectOption('oficina');
  await page.getByLabel('Región').selectOption('13');
  await page.getByLabel('Comuna').selectOption('13123');
  await page.getByLabel('Wi-Fi').selectOption('true');
  await page.getByLabel('Desde (fecha y hora)', { exact: true }).fill('2026-10-20T10:00');
  await page.getByLabel('Hasta (fecha y hora)', { exact: true }).fill('2026-10-20T11:00');
  await page.getByLabel('Total estimado mínimo (CLP)', { exact: true }).fill('18000');
  await page.getByLabel('Ordenar por').selectOption('estimated_total_asc');
  const filteredResponse = page.waitForResponse(response => response.request().method() === 'GET' && /\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url()) && response.url().includes('estimated_total_asc'));
  await page.getByRole('button', { name: 'Buscar espacios' }).click();
  const filtered = await filteredResponse;
  expect(filtered.status()).toBe(200);
  const filteredUrl = new URL(filtered.url());
  expect(filteredUrl.searchParams.get('q')).toBe('publico providencia');
  expect(filteredUrl.searchParams.get('region_code')).toBe('13');
  expect(filteredUrl.searchParams.get('comuna_code')).toBe('13123');
  expect(filteredUrl.searchParams.getAll('attribute')).toEqual(['wifi:true']);
  expect(filteredUrl.searchParams.get('start_at')).toBeTruthy();
  expect(filteredUrl.searchParams.get('end_at')).toBeTruthy();
  expect((await filtered.allHeaders()).authorization).toBeUndefined();
  await expect(page.getByRole('link', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  await expect(page.getByText('Total estimado: CLP 18.000')).toBeVisible();
  await takeScreenshot(page, 'fe14_public_search_desktop', true);

  // Keep the same filtered SPA state while capturing its narrow layout.
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByLabel('Región')).toHaveValue('13');
  await expect(page.getByLabel('Comuna')).toHaveValue('13123');
  await takeScreenshot(page, 'fe14_public_search_mobile', true);

  await page.getByRole('link', { name: 'Espacio sintético público FE-14' }).click();
  await expect(page.getByRole('heading', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  await expect(page.getByText('CLP 18.000 / hora')).toBeVisible();
  await expect(page.getByText('Fotografía no disponible')).toBeVisible();
  await expect(page.getByText(/Dirección privada de prueba/)).toHaveCount(0);
  await takeScreenshot(page, 'fe14_public_catalog_desktop');
});

test('Typed hosting form creates and edits a synthetic draft through Go', async ({ page }) => {
  await page.setViewportSize({ width: 1365, height: 900 });
  await login(page, accounts.host);
  await navigateSpa(page, '/panel/espacios');
  await expect(page.getByRole('heading', { name: 'Mis espacios' })).toBeVisible();
  await page.getByRole('combobox', { name: 'Categoría', exact: true }).selectOption('oficina');
  await expect(page.locator('.attributes legend')).toContainText('perfil versión');
  await page.getByLabel('Título', { exact: true }).fill('Oficina sintética · arquitectura');
  await page.getByLabel('Descripción', { exact: true }).fill('Fixture desechable para comprobar los formularios tipados, el aislamiento del dominio y la confirmación del borrador por Go.');
  await page.getByLabel('Superficie (m²)', { exact: true }).fill('40');
  await page.getByLabel('Capacidad', { exact: true }).fill('4');
  await page.getByLabel('Tarifa base inicial (CLP)', { exact: false }).fill('15000');
  await page.getByLabel('Dirección privada', { exact: false }).fill('Dirección sintética 100, Santiago');
  await page.getByLabel('Reglas de uso', { exact: true }).fill('Solo fixture de prueba.');
  const createdResponse = page.waitForResponse(response => response.request().method() === 'POST' && response.url().endsWith('/api/v1/spaces'));
  await page.getByRole('button', { name: 'Crear borrador', exact: true }).click();
  const created = await createdResponse;
  expect(created.status()).toBe(201);
  const draft = await created.json();
  await expect(page).toHaveURL(new RegExp(`/panel/espacios/${draft.id}/editar$`));
  await expect(page.getByLabel('Título', { exact: true })).toHaveValue('Oficina sintética · arquitectura');
  await page.getByLabel('Título', { exact: true }).fill('Oficina sintética · edición confirmada');
  const updatedResponse = page.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith(`/api/v1/spaces/${draft.id}`));
  await page.getByRole('button', { name: 'Guardar cambios del borrador', exact: true }).click();
  expect((await updatedResponse).status()).toBe(200);
  await expect(page.getByLabel('Título', { exact: true })).toHaveValue('Oficina sintética · edición confirmada');
  await takeScreenshot(page, 'fe14_architecture_host_desktop');
  await page.setViewportSize({ width: 390, height: 844 });
  await takeScreenshot(page, 'fe14_architecture_host_mobile');
});

test('MPV2-04 host declares a photo public; anonymous visitor reads it and withdrawal returns 404', async ({ browser }) => {
  const title = `Sala MPV2-04 ${Date.now()}`;
  const host = await browser.newPage({ viewport: { width: 1365, height: 900 }, deviceScaleFactor: 1 });
  await login(host, accounts.host);
  await host.getByRole('link', { name: 'Anfitrión', exact: true }).click();
  await expect(host.getByRole('heading', { name: 'Mis espacios' })).toBeVisible();
  await host.getByRole('button', { name: '+ Nuevo borrador', exact: true }).click();
  await host.getByRole('combobox', { name: 'Categoría', exact: true }).selectOption('oficina');
  await expect(host.locator('[name="publicRegion"]')).toBeEnabled();
  await host.getByLabel('Título', { exact: true }).fill('Borrador MPV2-03');
  await host.getByLabel('Descripción', { exact: true }).fill('Descripción sintética suficientemente extensa para publicar y validar la búsqueda pública en la región seleccionada.');
  await host.getByLabel('Superficie (m²)', { exact: true }).fill('35');
  await host.getByLabel('Capacidad', { exact: true }).fill('5');
  await host.getByLabel('Tarifa base inicial (CLP)', { exact: false }).fill('12000');
  await host.getByLabel('Dirección privada', { exact: false }).fill('Dirección confidencial MPV2-03 987');
  await host.locator('[name="publicRegion"]').selectOption('13');
  await host.locator('[name="publicCommune"]').selectOption('13123');
  await host.getByLabel('Reglas de uso', { exact: true }).fill('Prueba de publicación sintética.');
  const createdResponse = host.waitForResponse(response => response.request().method() === 'POST' && response.url().endsWith('/api/v1/spaces'));
  await host.getByRole('button', { name: 'Crear borrador', exact: true }).click();
  const created = await createdResponse;
  expect(created.status()).toBe(201);
  const draft = await created.json();
  expect(draft.region_code).toBe('13');
  expect(draft.comuna_code).toBe('13123');
  await expect(host.getByLabel('Título', { exact: true })).toHaveValue('Borrador MPV2-03');
  await host.getByLabel('Título', { exact: true }).fill(title);
  const updatedResponse = host.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith(`/api/v1/spaces/${draft.id}`));
  await host.getByRole('button', { name: 'Guardar cambios del borrador', exact: true }).click();
  expect((await updatedResponse).status()).toBe(200);
  await host.getByRole('button', { name: 'Calendario', exact: true }).click();
  await host.getByLabel('Zona horaria IANA').fill('America/Santiago');
  const zoneResponse = host.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith(`/api/v1/spaces/${draft.id}/availability`));
  await host.getByRole('button', { name: 'Guardar zona', exact: true }).click();
  expect((await zoneResponse).status()).toBe(200);
  await host.getByRole('button', { name: 'Datos', exact: true }).click();
  const publishResponse = host.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith(`/api/v1/spaces/${draft.id}/publication`));
  await host.getByRole('button', { name: 'Publicar', exact: true }).click();
  expect((await publishResponse).status()).toBe(200);
  await expect(host.getByText('Publicada', { exact: true })).toBeVisible();
  await host.getByRole('button', { name: 'Galería', exact: true }).click();
  const photoResponsePromise=host.waitForResponse(response=>response.request().method()==='POST'&&response.url().endsWith(`/api/v1/spaces/${draft.id}/gallery`));
  await host.getByRole('button',{name:'Generar imagen sintética'}).click();
  const photoResponse=await photoResponsePromise;expect(photoResponse.status()).toBe(201);
  const photoId=(await photoResponse.json()).photo.id;
  await expect(host.getByText('Privada',{exact:true})).toBeVisible();
  const publicResponse=host.waitForResponse(response=>response.request().method()==='PUT'&&response.url().endsWith(`/api/v1/spaces/${draft.id}/gallery/${photoId}/publication`));
  await host.getByRole('button',{name:'Mostrar en ficha pública'}).click();
  expect((await publicResponse).status()).toBe(200);
  const photo2ResponsePromise=host.waitForResponse(response=>response.request().method()==='POST'&&response.url().endsWith(`/api/v1/spaces/${draft.id}/gallery`));
  await host.getByRole('button',{name:'Generar imagen sintética'}).click();
  const photo2Response=await photo2ResponsePromise;expect(photo2Response.status()).toBe(201);
  const photo2Id=(await photo2Response.json()).photo.id;
  const public2ResponsePromise=host.waitForResponse(response=>response.request().method()==='PUT'&&response.url().endsWith(`/api/v1/spaces/${draft.id}/gallery/${photo2Id}/publication`));
  await host.getByRole('button',{name:'Mostrar en ficha pública'}).last().click();
  expect((await public2ResponsePromise).status()).toBe(200);
  await host.getByRole('button',{name:`Ver imagen sintética ${photo2Id}`}).locator('xpath=ancestor::article').getByRole('button',{name:'Usar como portada'}).click();
  const photo3ResponsePromise=host.waitForResponse(response=>response.request().method()==='POST'&&response.url().endsWith(`/api/v1/spaces/${draft.id}/gallery`));
  await host.getByRole('button',{name:'Generar imagen sintética'}).click();
  const photo3Response=await photo3ResponsePromise;expect(photo3Response.status()).toBe(201);
  const privatePhotoId=(await photo3Response.json()).photo.id;
  await expect(host.getByText('Privada',{exact:true}).last()).toBeVisible();
  await takeScreenshot(host, 'fe14_mpv2_04_host_desktop');
  await host.setViewportSize({ width: 390, height: 844 });
  await takeScreenshot(host, 'fe14_mpv2_04_host_mobile', true);

  const visitor = await browser.newPage({ viewport: { width: 1365, height: 900 }, deviceScaleFactor: 1 });
  let searchRequest;
  visitor.on('request', request => {
    if (request.method() === 'GET' && /\/api\/v1\/public\/spaces(?:\?|$)/.test(request.url())) searchRequest = request;
  });
  await visitor.goto('/explorar');
  await visitor.getByLabel('Buscar', { exact: true }).fill(title);
  await visitor.getByLabel('Región').selectOption('13');
  await visitor.getByLabel('Comuna').selectOption('13123');
  const searchResponse = visitor.waitForResponse(response => response.request().method() === 'GET' && /\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url()));
  await visitor.getByRole('button', { name: 'Buscar espacios' }).click();
  expect((await searchResponse).status()).toBe(200);
  expect(searchRequest).toBeTruthy();
  expect((await searchRequest.allHeaders()).authorization).toBeUndefined();
  await expect(visitor.getByRole('link', { name: title })).toBeVisible();
  await expect(visitor.getByText('Providencia, Metropolitana de Santiago')).toBeVisible();
  await takeScreenshot(visitor, 'fe14_mpv2_04_visitor_search_desktop', true);
  await visitor.setViewportSize({ width: 390, height: 844 });
  await takeScreenshot(visitor, 'fe14_mpv2_04_visitor_search_mobile', true);
  await visitor.setViewportSize({ width: 1365, height: 900 });
  const imageResponsePromise=visitor.waitForResponse(response=>response.request().method()==='GET'&&response.url().includes(`/api/v1/public/spaces/${draft.id}/images/${photoId}`));
  const publicDetailResponsePromise=visitor.waitForResponse(response=>response.request().method()==='GET'&&response.url().endsWith(`/api/v1/public/spaces/${draft.id}`));
  await visitor.getByRole('link', { name: title }).click();
  const publicDetailResponse=await publicDetailResponsePromise;
  expect(publicDetailResponse.status()).toBe(200);
  const detailDTO=(await publicDetailResponse.json()).data;
  expect(detailDTO.images).toEqual([{image_id:photo2Id,is_cover:true},{image_id:photoId,is_cover:false}]);
  expect(JSON.stringify(detailDTO.images)).not.toContain(privatePhotoId);
  expect(detailDTO.usage_rules).toBe('Prueba de publicación sintética.');
  expect(JSON.stringify(detailDTO)).not.toContain('Dirección confidencial');
  await expect(visitor.getByRole('heading', { name: title })).toBeVisible();
  await expect(visitor.getByText(/Dirección confidencial MPV2-03/)).toHaveCount(0);
  await expect(visitor.getByRole('img',{name:'Fotografía de portada del espacio'})).toBeVisible();
  await expect(visitor.getByRole('img',{name:'Fotografía pública 2'})).toBeVisible();
  const imageResponse=await imageResponsePromise;expect(imageResponse.status()).toBe(200);
  expect((await imageResponse.request().allHeaders()).authorization).toBeUndefined();
  const imageHeaders=await imageResponse.allHeaders();
  expect(imageHeaders['content-type']).toContain('image/png');
  expect(imageHeaders['cache-control']).toBe('no-store');
  expect(imageHeaders['x-content-type-options']).toBe('nosniff');
  await takeScreenshot(visitor,'fe14_mpv2_04_detail_desktop',true);
  await visitor.setViewportSize({width:390,height:844});
  await takeScreenshot(visitor,'fe14_mpv2_04_detail_mobile',true);
  await host.setViewportSize({width:1365,height:900});
  await host.getByRole('button',{name:`Ver imagen sintética ${photoId}`}).locator('xpath=ancestor::article').getByRole('button',{name:'Retirar de ficha pública'}).click();
  const withdrawn=await visitor.request.get(imageResponse.url());
  expect(withdrawn.status()).toBe(404);
  await visitor.reload();
  await expect(visitor.getByRole('heading',{name:title})).toBeVisible();
  await expect(visitor.locator('.gallery-image img')).toHaveCount(1);
  await expect.poll(()=>visitor.locator('.gallery-image img').evaluate(image=>image.naturalWidth)).toBeGreaterThan(0);
  await host.close();
  await visitor.close();

  const thirdParty = await browser.newPage();
  await login(thirdParty, accounts.thirdParty);
  await navigateSpa(thirdParty, `/panel/espacios/${draft.id}/editar`);
  await expect(thirdParty).toHaveURL(/\/mi-cuenta$/);
  await thirdParty.close();
});

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
  let loginRequests = 0;
  page.on('request', (request) => {
    if (/\/auth\/login(?:\?|$)/.test(request.url())) loginRequests++;
  });
  await login(page, accounts.renter);
  expect(loginRequests).toBe(1);
  const accountId = await page.locator('.identity-card .identity-copy > p').nth(1).innerText();
  expect(accountId).toMatch(/^[0-9a-f-]{36}$/i);

  const ready = path.join(artifactDir, 'outage-session-ready');
  const stopped = path.join(artifactDir, 'outage-stopped');
  const restored = path.join(artifactDir, 'outage-restored');
  const restoreRequestStarted = path.join(artifactDir, 'restore-request-started');
  const releaseRestoreRequest = path.join(artifactDir, 'release-restore-request');
  fs.writeFileSync(ready, 'ready');

  await waitForMarker(stopped, 120_000);
  const failedEligibility = page.waitForEvent('requestfailed', {
    predicate: (request) => request.url().includes('/verifications/eligibility'),
  });
  await navigateSpa(page, '/mi-cuenta/verificaciones');
  await expect(page.getByRole('heading', { name: 'Casos de verificación' })).toBeVisible();
  const failedRequest = await failedEligibility;
  expect(Boolean(failedRequest.failure()?.errorText)).toBe(true);
  const failedHeaders = await failedRequest.allHeaders();
  expect(Boolean(failedHeaders.authorization?.startsWith('Bearer '))).toBe(true);
  await expect(page.getByRole('alert')).toBeVisible();
  await expect(page.getByRole('status').filter({ hasText: 'Consultando casos y elegibilidad con Go' })).toHaveCount(0);
  await expect(page.locator('.eligibility-card')).toHaveCount(0);

  // A genuine network failure must not log the current account out.
  await expect(page).not.toHaveURL(/\/acceso/);
  expect(loginRequests).toBe(1);
  fs.writeFileSync(path.join(artifactDir, 'outage-verified'), 'verified');
  await waitForMarker(restored, 120_000);

  // Same Angular document and IdentitySession: after Go is healthy again, the
  // account ID and role list must still match the original authenticated user.
  await navigateSpa(page, '/mi-cuenta');
  await expect(page.getByRole('heading', { name: 'Tu información y tus derechos' })).toBeVisible();
  await expect(page.locator('.identity-card .identity-copy > p').nth(1)).toHaveText(accountId);
  await expect(page.locator('.role-list')).toContainText(accounts.renter.roleLabel);
  await expect(page).not.toHaveURL(/\/acceso/);
  expect(loginRequests).toBe(1);

  await page.route('**/verifications/eligibility', async (route) => {
    fs.writeFileSync(restoreRequestStarted, 'started');
    await waitForMarker(releaseRestoreRequest);
    await route.continue();
  });
  const restoredEligibilityResponse = page.waitForResponse((response) =>
    response.url().includes('/verifications/eligibility'),
  );

  await navigateSpa(page, '/mi-cuenta/verificaciones');
  await expect(page.getByRole('heading', { name: 'Casos de verificación' })).toBeVisible();
  await expect(page.getByRole('status').filter({ hasText: 'Consultando casos y elegibilidad con Go' })).toBeVisible();
  await waitForMarker(restoreRequestStarted);
  // A delayed authenticated request must remain visibly loading and cannot be
  // mistaken for the response-less eligibility heading in the shell.
  await expect(page.locator('.eligibility-card')).toHaveCount(0);
  await expect(page.getByRole('status').filter({ hasText: 'Consultando casos y elegibilidad con Go' })).toBeVisible();
  await expect(page.getByRole('alert')).toHaveCount(0);
  fs.writeFileSync(releaseRestoreRequest, 'continue');

  const eligibilityResponse = await restoredEligibilityResponse;
  expect(eligibilityResponse.status()).toBe(200);
  const authorization = (await eligibilityResponse.request().allHeaders()).authorization;
  expect(Boolean(authorization?.startsWith('Bearer '))).toBe(true);
  const eligibilityBody = await eligibilityResponse.json();
  expect(eligibilityBody.scope).toBe('synthetic_local');
  expect(eligibilityBody.items).toHaveLength(2);
  await expect(page.locator('.eligibility-card')).toHaveCount(2);
  for (const item of eligibilityBody.items) {
    const label = item.type === 'kyc' ? 'Persona (KYC)' : 'Organización (KYB)';
    const card = page.locator('.eligibility-card').filter({ hasText: label });
    await expect(card).toContainText(item.eligible ? 'Elegible' : 'No elegible');
    await expect(card).toContainText(item.verification_id || 'sin caso vigente');
  }
  await expect(page.getByRole('status').filter({ hasText: 'Consultando casos y elegibilidad con Go' })).toHaveCount(0);
  await expect(page.getByRole('alert')).toHaveCount(0);
  expect(loginRequests).toBe(1);
  expect(await page.evaluate(() => [localStorage.length, sessionStorage.length])).toEqual([0, 0]);
  await page.screenshot({ path: path.join(artifactDir, 'fe14_restored_session_desktop.jpg'), type: 'jpeg', quality: 86 });
  await navigateSpa(page, '/mi-cuenta');
  await expect(page.getByRole('heading', { name: 'Tu información y tus derechos' })).toBeVisible();
  await expect(page.locator('.identity-card .identity-copy > p').nth(1)).toHaveText(accountId);
  await expect(page.locator('.role-list')).toContainText(accounts.renter.roleLabel);
  await expect(page).not.toHaveURL(/\/acceso/);
  expect(loginRequests).toBe(1);
  await page.close();
});
