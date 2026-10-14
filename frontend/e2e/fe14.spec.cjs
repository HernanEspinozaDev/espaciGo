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
  measurements.head = process.env.FE14_HEAD || 'unknown';
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

test('MPV2-07 serves public catalog, detail, metadata, and sitemap from Go without JavaScript', async ({ request, page }) => {
  const origin = process.env.FE14_PUBLIC_ORIGIN;
  const frontend = process.env.FRONTEND_BASE_URL || 'http://angular-frontend:4000';
  expect(origin).toMatch(/^https?:\/\//);
  const goItems = [];
  let cursor;
  do {
    const query = new URLSearchParams({ page_size: '25' });
    if (cursor) query.set('cursor', cursor);
    const goCatalog = await request.get(`http://backend:8080/api/v1/public/spaces?${query}`);
    expect(goCatalog.status()).toBe(200);
    const pageData = await goCatalog.json();
    goItems.push(...pageData.items);
    cursor = pageData.next_cursor;
  } while (cursor);
  expect(goItems.length).toBeGreaterThan(0);
  const space = goItems[0];

  const spoofedHostResponse = await request.get(`${frontend}/explorar`, {
    headers: { host: 'host-header-attacker.invalid' },
  });
  expect(spoofedHostResponse.status()).toBe(400);

  const catalogResponse = await request.get(`${frontend}/explorar`);
  expect(catalogResponse.status()).toBe(200);
  expect(catalogResponse.headers()['cache-control']).toBe('no-store');
  const catalogHtml = await catalogResponse.text();
  expect(catalogHtml).toMatch(/<h1[^>]*>Encuentra tu próximo espacio<\/h1>/);
  expect(catalogHtml).toContain(space.title);
  expect(catalogHtml).toContain(`<link rel="canonical" href="${origin}/explorar">`);
  expect(catalogHtml).toContain('name="robots" content="index, follow"');
  expect(catalogHtml).not.toContain('http://backend:8080');
  expect(catalogHtml.toLowerCase()).not.toMatch(/bearer|access_token|password|owner_id/);

  const filteredResponse = await request.get(`${frontend}/explorar?q=publico`);
  expect(filteredResponse.status()).toBe(200);
  const filteredHtml = await filteredResponse.text();
  expect(filteredHtml).toContain('name="robots" content="noindex, follow"');
  expect(filteredHtml).toContain(`<link rel="canonical" href="${origin}/explorar">`);
  const presentationHtml = await (await request.get(`${frontend}/explorar?view=grid&desktop_columns=4&mobile_columns=1`)).text();
  expect(presentationHtml).toContain('name="robots" content="index, follow"');
  expect(presentationHtml).toContain(`<link rel="canonical" href="${origin}/explorar">`);
  expect(presentationHtml).not.toContain('Vista de resultados');
  expect(presentationHtml).not.toContain('Columnas en escritorio');

  const detailResponse = await request.get(`${frontend}/espacios/${space.space_id}`);
  expect(detailResponse.status()).toBe(200);
  expect(detailResponse.headers()['cache-control']).toBe('no-store');
  const detailHtml = await detailResponse.text();
  expect(detailHtml).toContain(space.title);
  expect(detailHtml).toContain(`<link rel="canonical" href="${origin}/espacios/${space.space_id}">`);
  expect(detailHtml).toContain(`<meta property="og:url" content="${origin}/espacios/${space.space_id}">`);
  const goDetail = await request.get(`http://backend:8080/api/v1/public/spaces/${space.space_id}`);
  const cover = (await goDetail.json()).data.images.find((image) => image.is_cover);
  if (cover) expect(detailHtml).toContain(`${origin}/public/spaces/${space.space_id}/images/${cover.image_id}`);
  else expect(detailHtml).not.toContain('property="og:image"');
  expect(detailHtml).not.toContain('http://backend:8080');
  expect(detailHtml.toLowerCase()).not.toMatch(/bearer|access_token|password|owner_id/);

  const contextualResponse = await request.get(`${frontend}/espacios/${space.space_id}?context=returned&returnTo=%2Fexplorar`);
  const contextualHtml = await contextualResponse.text();
  expect(contextualHtml).toContain('name="robots" content="noindex, follow"');
  expect(contextualHtml).toContain(`<link rel="canonical" href="${origin}/espacios/${space.space_id}">`);

  const missingResponse = await request.get(`${frontend}/espacios/00000000-0000-4000-8000-000000000000`);
  expect(missingResponse.status()).toBe(404);
  expect(await missingResponse.text()).toContain('Este espacio ya no está disponible.');

  const sitemapResponse = await request.get(`${frontend}/sitemap.xml`);
  expect(sitemapResponse.status()).toBe(200);
  expect(sitemapResponse.headers()['cache-control']).toBe('no-store');
  const sitemap = await sitemapResponse.text();
  const sitemapIds = [...sitemap.matchAll(/<loc>[^<]*\/espacios\/([0-9a-f-]{36})<\/loc>/g)].map((match) => match[1]);
  expect(new Set(sitemapIds)).toEqual(new Set(goItems.map((item) => item.space_id)));
  expect(sitemap).not.toMatch(/cursor|returnTo|\?q=/i);
  const robots = await request.get(`${frontend}/robots.txt`);
  expect(robots.status()).toBe(200);
  expect(await robots.text()).toContain(`Sitemap: ${origin}/sitemap.xml`);

  const privatePage = await request.get(`${frontend}/mi-cuenta`);
  expect(privatePage.headers()['x-robots-tag']).toBe('noindex, nofollow');
  expect(privatePage.headers()['cache-control']).toBe('no-store');

  const concurrentHtml = await Promise.all([
    ...goItems.map((item) => request.get(`${frontend}/espacios/${item.space_id}`)),
    request.get(`${frontend}/mi-cuenta`),
  ]);
  const renderedPages = await Promise.all(concurrentHtml.map((response) => response.text()));
  for (let index = 0; index < goItems.length; index++) {
    expect(concurrentHtml[index].status()).toBe(200);
    expect(renderedPages[index]).toContain(goItems[index].title);
    for (let other = 0; other < goItems.length; other++) {
      if (other !== index) expect(renderedPages[index]).not.toContain(goItems[other].title);
    }
    expect(renderedPages[index].toLowerCase()).not.toMatch(/bearer|access_token|password|owner_id/);
  }
  expect(concurrentHtml.at(-1).headers()['x-robots-tag']).toBe('noindex, nofollow');
  expect(renderedPages.at(-1).toLowerCase()).not.toMatch(/bearer|access_token|password|owner_id/);

  const browserCatalogRequests = [];
  page.on('request', (browserRequest) => {
    if (browserRequest.url().includes('/api/v1/public/spaces')) browserCatalogRequests.push(browserRequest.url());
  });
  await page.goto('/explorar');
  await expect(page.getByRole('link', { name: goItems[0].title })).toBeVisible();
  expect(browserCatalogRequests).toEqual([]);
  await page.setViewportSize({ width: 1365, height: 900 });
  await takeScreenshot(page, 'fe14_mpv2_07_catalog_desktop', true);
  await page.setViewportSize({ width: 390, height: 844 });
  await takeScreenshot(page, 'fe14_mpv2_07_catalog_mobile', true);
  await page.setViewportSize({ width: 1365, height: 900 });
  await page.getByRole('link', { name: goItems[0].title }).first().click();
  await expect(page.getByRole('heading', { name: goItems[0].title })).toBeVisible();
  await takeScreenshot(page, 'fe14_mpv2_07_detail_desktop', true);
  await page.setViewportSize({ width: 390, height: 844 });
  await takeScreenshot(page, 'fe14_mpv2_07_detail_mobile', true);
});

test('Anonymous visitor explores the Go-backed public catalog and minimized detail', async ({ page }) => {
  await page.setViewportSize({ width: 1365, height: 900 });
  await page.goto('/');
  await page.getByLabel('¿Qué espacio estás buscando?').fill('Espacio sintético público FE-14');
  await page.getByRole('button', { name: 'Buscar espacios' }).click();
  await expect(page).toHaveURL(/\/explorar\?q=/);
  await page.goto('/categorias');
  await page.getByRole('link', { name: /Oficina/ }).click();
  await expect(page).toHaveURL(/\/explorar\?category_code=oficina/);
  await expect(page.getByRole('link', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  const initialPublicRequests = [];
  page.on('request', request => {
    if (request.method() === 'GET' && /\/api\/v1\/public\/spaces(?:\?|$)/.test(request.url())) initialPublicRequests.push(request);
  });
  await page.goto('/explorar');
  await expect(page.getByRole('heading', { name: 'Encuentra tu próximo espacio' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  expect(initialPublicRequests).toEqual([]);

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
  const baseFilterResponse = page.waitForResponse(response => response.request().method() === 'GET' && /\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url()) && response.url().includes('min_total_clp=18000'));
  await page.getByRole('button', { name: 'Buscar espacios' }).click();
  const filteredResponseWithStatus = await baseFilterResponse;
  expect(filteredResponseWithStatus.status()).toBe(200);
  expect((await filteredResponseWithStatus.request().allHeaders()).authorization).toBeUndefined();
  const filteredResponse = page.waitForResponse(response => response.request().method() === 'GET' && /\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url()) && response.url().includes('estimated_total_asc'));
  await page.locator('.catalog-sort select').selectOption('estimated_total_asc');
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
  await takeScreenshot(page, 'fe14_mpv2_05_catalog_desktop', true);
  await takeScreenshot(page, 'fe14_public_search_desktop', true);

  // Keep the same filtered SPA state while capturing its narrow layout.
  await page.setViewportSize({ width: 390, height: 844 });
  const filterTrigger = page.getByRole('button', { name: 'Filtrar' });
  await filterTrigger.click();
  const filterDialog = page.getByRole('dialog', { name: 'Filtros de espacios' });
  await expect(filterDialog).toBeVisible();
  await expect(filterDialog.getByLabel('Región', { exact: false })).toHaveValue('13');
  await expect(filterDialog.getByLabel('Comuna', { exact: false })).toHaveValue('13123');
  await page.keyboard.press('Escape');
  await expect(filterDialog).toHaveCount(0);
  await expect(filterTrigger).toBeFocused();
  await filterTrigger.click();
  await page.getByRole('button', { name: 'Cerrar filtros' }).last().click();
  await filterTrigger.click();
  await takeScreenshot(page, 'fe14_mpv2_05_filters_mobile');
  await page.getByRole('button', { name: 'Cerrar filtros' }).last().click();
  await takeScreenshot(page, 'fe14_public_search_mobile', true);

  await page.getByRole('link', { name: 'Espacio sintético público FE-14' }).click();
  await expect(page.getByRole('heading', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  await expect(page.getByText('CLP 18.000 / hora')).toBeVisible();
  await expect(page.getByText('Fotografía no disponible')).toBeVisible();
  await expect(page.getByText(/Dirección privada de prueba/)).toHaveCount(0);
  await takeScreenshot(page, 'fe14_public_catalog_desktop');

  await page.goto('/explorar?category_code=oficina&page_size=1');
  await expect(page.getByRole('link', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  const secondPage = page.waitForResponse(response => response.request().method() === 'GET' && /\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url()) && response.url().includes('cursor='));
  await page.getByRole('button', { name: 'Ver más espacios' }).click();
  expect((await secondPage).status()).toBe(200);
  await expect(page.locator('.space-card')).toHaveCount(2);
  const nextPageUrl = page.url();
  await page.goBack();
  await expect(page).toHaveURL(/page_size=1/);
  await expect(page.locator('.space-card')).toHaveCount(1);
  await page.goForward();
  await expect(page).toHaveURL(nextPageUrl);
  await expect(page.locator('.space-card')).toHaveCount(2);
});

test('MPV2-13 serves real 48-item keyset pages and changes page size without stale cursors', async ({ page, request }) => {
  const firstResponse = await request.get('http://backend:8080/api/v1/public/spaces?page_size=48&sort=default');
  expect(firstResponse.status()).toBe(200);
  const first = await firstResponse.json();
  expect(first.items).toHaveLength(48);
  expect(first.next_cursor).toBeTruthy();
  const secondResponse = await request.get(`http://backend:8080/api/v1/public/spaces?page_size=48&sort=default&cursor=${encodeURIComponent(first.next_cursor)}`);
  expect(secondResponse.status()).toBe(200);
  const second = await secondResponse.json();
  expect(second.items.length).toBeGreaterThan(0);
  expect(new Set([...first.items, ...second.items].map(item => item.space_id)).size).toBe(first.items.length + second.items.length);
  expect((await request.get(`http://backend:8080/api/v1/public/spaces?page_size=24&sort=default&cursor=${encodeURIComponent(first.next_cursor)}`)).status()).toBe(422);

  await page.setViewportSize({ width: 1365, height: 900 });
  await page.goto('/explorar?page_size=12');
  await expect(page.locator('.space-card')).toHaveCount(12);
  const sort = page.getByLabel('Ordenar por');
  const size = page.getByLabel('Espacios por página');
  await expect(sort).toBeVisible();
  await expect(size).toBeVisible();
  await sort.focus();
  await page.keyboard.press('Tab');
  await expect(size).toBeFocused();
  const setSize = async (value, count, expectNetworkRequest = true) => {
    await expect(size).toBeEnabled();
    const response = expectNetworkRequest
      ? page.waitForResponse(r => r.request().method() === 'GET'
        && /\/api\/v1\/public\/spaces(?:\?|$)/.test(r.url())
        && new URL(r.url()).searchParams.get('page_size') === String(value))
      : null;
    await size.selectOption(String(value));
    if (response) expect((await response).status()).toBe(200);
    await expect(page.locator('.space-card')).toHaveCount(count);
    if (value === 24) await expect(page).not.toHaveURL(/page_size=/);
    else await expect(page).toHaveURL(new RegExp(`page_size=${value}`));
    await expect(page).not.toHaveURL(/cursor=/);
  };
  await setSize(24, 24);
  await setSize(48, 48);
  await page.getByRole('button', { name: 'Ver más espacios' }).click();
  await expect(page.locator('.space-card')).toHaveCount(first.items.length + second.items.length);
  // These criteria/page-size combinations are already cached from the first
  // requests. Returning from page two must restore the correct first page and
  // clear its cursor without forcing a duplicate API request.
  await setSize(12, 12, false);
  await setSize(24, 24, false);
  await setSize(48, 48, false);
});

test('MPV2-13 renders a fixed four-column desktop catalog and a one-column mobile list', async ({ page }) => {
  await page.setViewportSize({ width: 1365, height: 900 });
  await page.goto('/explorar?view=grid&desktop_columns=3&mobile_columns=2&page_size=24');
  await expect(page.locator('.space-card')).toHaveCount(24);
  await expect(page).not.toHaveURL(/view=|desktop_columns=|mobile_columns=/);
  for (const label of ['Vista de resultados', 'Columnas en escritorio', 'Columnas en móvil']) await expect(page.getByLabel(label)).toHaveCount(0);
  const columns = () => page.locator('.results').evaluate(el => getComputedStyle(el).gridTemplateColumns.split(' ').length);
  await expect.poll(columns).toBe(4);
  await page.evaluate(() => (document.activeElement instanceof HTMLElement) && document.activeElement.blur());
  await takeScreenshot(page, 'fe14_mpv2_13_catalog_desktop');
  let dimensions;
  for (const width of [768, 900, 1024]) {
    await page.setViewportSize({ width, height: 900 });
    await expect.poll(columns).toBe(4);
    dimensions = await page.evaluate(() => ({ viewport: document.documentElement.clientWidth, document: document.documentElement.scrollWidth }));
    expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport);
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await expect.poll(columns).toBe(1);
  const sort = page.getByLabel('Ordenar por');
  const size = page.getByLabel('Espacios por página');
  await expect(sort).toBeVisible();
  await expect(size).toBeVisible();
  await sort.focus();
  await page.keyboard.press('Tab');
  await expect(size).toBeFocused();
  dimensions = await page.evaluate(() => ({ viewport: document.documentElement.clientWidth, document: document.documentElement.scrollWidth }));
  expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport);
  await page.evaluate(() => (document.activeElement instanceof HTMLElement) && document.activeElement.blur());
  await takeScreenshot(page, 'fe14_mpv2_13_catalog_mobile');
});

test('MPV2-11 validates typed public filters and pagination across all eight categories',async({request,page})=>{
  const categories=await (await request.get('http://backend:8080/api/v1/public/space-categories')).json();
  expect(categories.items).toHaveLength(8);
  const cases=[
    ['oficina','tipo_uso_oficina:eq:privada'],
    ['sala_multiproposito','disposiciones_admitidas:in:aula'],
    ['bodega','altura_util_m:gte:3.50','altura_util_m:lte:3.50','tipo_almacenamiento:eq:seco'],
    ['estacionamiento','vehiculos_admitidos:in:automovil'],
    ['local_flexible','frente_m:gte:4.50'],
    ['stand','lados_abiertos:gte:2'],
    ['quincho','tipo_parrilla:eq:carbon'],
    ['parcela_eventos','superficie_exterior_util_m2:gte:100.00'],
  ];
  for(const [category,...attributes] of cases){
    const metadataResponse=await request.get(`http://backend:8080/api/v1/public/space-categories/${category}/filters`);
    expect(metadataResponse.status()).toBe(200);
    const metadata=(await metadataResponse.json()).items;
    expect(metadata.length).toBeGreaterThan(0);
    expect(metadata.every(item=>item.order>0&&item.profile_version===1&&item.label&&item.type)).toBeTruthy();
    const params=new URLSearchParams({category_code:category,page_size:'1'});
    for(const attribute of attributes)params.append('attribute',attribute);
    const first=await request.get(`http://backend:8080/api/v1/public/spaces?${params}`);
    expect(first.status(),`${category}: ${await first.text()}`).toBe(200);
    const firstPage=await first.json();
    expect(firstPage.items).toHaveLength(1);
    expect(firstPage.next_cursor).toBeTruthy();
    params.set('cursor',firstPage.next_cursor);
    const second=await request.get(`http://backend:8080/api/v1/public/spaces?${params}`);
    expect(second.status()).toBe(200);
    const secondPage=await second.json();
    expect(secondPage.items).toHaveLength(1);
    expect(secondPage.items[0].space_id).not.toBe(firstPage.items[0].space_id);
  }
  const legacyFalse=await request.get('http://backend:8080/api/v1/public/spaces?category_code=bodega&attribute=muelle_carga:false&page_size=10');
  expect(legacyFalse.status()).toBe(200);
  expect((await legacyFalse.json()).items).toHaveLength(4);
  const legacyTrue=await request.get('http://backend:8080/api/v1/public/spaces?category_code=bodega&attribute=muelle_carga:true&page_size=10');
  expect(legacyTrue.status()).toBe(200);
  expect((await legacyTrue.json()).items).toHaveLength(1);
  const booleanMiss=await request.get('http://backend:8080/api/v1/public/spaces?category_code=estacionamiento&attribute=carga_electrica:true&page_size=10');
  expect(booleanMiss.status()).toBe(200);
  expect((await booleanMiss.json()).items).toHaveLength(0);
  for(const query of [
    '?category_code=bodega&attribute=altura_util_m:gte:1.23&attribute=altura_util_m:gte:2.00',
    '?category_code=bodega&attribute=altura_util_m:gte:1e2',
    '?category_code=bodega&attribute=tipo_almacenamiento:eq:no_existe',
    '?category_code=oficina&attribute=altura_util_m:gte:1',
    '?category_code=bodega&attribute=altura_util_m:gte:9.00&attribute=altura_util_m:lte:2.00',
    '?attribute=wifi:true',
  ])expect((await request.get(`http://backend:8080/api/v1/public/spaces${query}`)).status()).toBe(422);

  await page.setViewportSize({width:1365,height:900});
  await page.goto('/explorar');
  await page.locator('.filters label').filter({hasText:/^Categoría/}).locator('select').selectOption('bodega');
  await expect(page.getByLabel('Altura útil (m) mínimo')).toBeVisible();
  await page.getByLabel('Altura útil (m) mínimo').fill('3.50');
  await page.getByLabel('Altura útil (m) máximo').fill('3.50');
  const uiSearch=page.waitForResponse(response=>response.request().method()==='GET'&&/\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url()));
  await page.getByRole('button',{name:'Buscar espacios'}).click();
  const uiSearchResponse=await uiSearch;
  expect(uiSearchResponse.status(),await uiSearchResponse.text()).toBe(200);
  await expect(page).toHaveURL(/attribute=altura_util_m:gte:3\.50/);
  await expect(page.getByText('MPV2-12 bodega fixture 1',{exact:true})).toBeVisible();
  await page.goBack();
  await page.goForward();
  await expect(page.getByLabel('Altura útil (m) mínimo')).toHaveValue('3.50');
  await takeScreenshot(page,'fe14_mpv2_11_typed_filters_desktop',true);
  await page.locator('.filters label').filter({hasText:/^Categoría/}).locator('select').selectOption('sala_multiproposito');
  await expect(page.getByLabel('Disposiciones admitidas (selección múltiple)')).toBeVisible();
  await page.getByLabel('Disposiciones admitidas (selección múltiple)').selectOption(['auditorio','aula']);
  const listSearch=page.waitForResponse(response=>response.request().method()==='GET'&&/\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url()));
  await page.getByRole('button',{name:'Buscar espacios'}).click();
  const listResponse=await listSearch;
  expect(listResponse.status()).toBe(200);
  expect(new URL(listResponse.url()).searchParams.getAll('attribute')).toEqual(['disposiciones_admitidas:in:auditorio,aula']);
  await expect(page).toHaveURL(/attribute=disposiciones_admitidas:in:auditorio,aula/);
  await page.goBack();
  await expect(page.getByLabel('Altura útil (m) mínimo')).toHaveValue('3.50');
  await page.goForward();
  await expect(page.getByLabel('Disposiciones admitidas (selección múltiple)')).toHaveValues(['auditorio','aula']);
  await expect(page.getByText('MPV2-12 sala_multiproposito fixture 1',{exact:true})).toBeVisible();
  const ssrCsvResponse = await request.get(`${process.env.FRONTEND_BASE_URL || 'http://angular-frontend:4000'}/explorar?category_code=sala_multiproposito&attribute=disposiciones_admitidas%3Ain%3Aauditorio%2Caula`);
  expect(ssrCsvResponse.status()).toBe(200);
  expect(await ssrCsvResponse.text()).toContain('MPV2-12 sala_multiproposito fixture 1');
  await page.setViewportSize({width:390,height:844});
  await page.getByRole('button',{name:'Filtrar'}).click();
  await expect(page.getByRole('dialog',{name:'Filtros de espacios'})).toBeVisible();
  await takeScreenshot(page,'fe14_mpv2_11_typed_filters_mobile',true);
});

test('MPV2-10 orders real Go results by title, resets cursor, and restores order through browser history', async ({ page }) => {
  await page.setViewportSize({ width: 1365, height: 900 });
  const initialResponse = await page.request.get('http://backend:8080/api/v1/public/spaces?category_code=oficina&page_size=1');
  expect(initialResponse.status()).toBe(200);
  await page.goto('/explorar?category_code=oficina&page_size=1');
  const firstItem = (await initialResponse.json()).items[0];
  await expect(page.getByRole('heading', { name: 'Encuentra tu próximo espacio' })).toBeVisible();
  await expect(page.getByRole('link', { name: firstItem.title })).toBeVisible();

  const ascResponse = page.waitForResponse(response => response.request().method() === 'GET'
    && /\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url())
    && new URL(response.url()).searchParams.get('sort') === 'title_asc');
  await page.locator('.catalog-sort select').selectOption('title_asc');
  const asc = await ascResponse;
  expect(asc.status()).toBe(200);
  const ascFirst = (await asc.json()).items[0];
  await expect(page.getByRole('link', { name: ascFirst.title })).toBeVisible();

  const nextPage = page.waitForResponse(response => response.request().method() === 'GET'
    && /\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url())
    && new URL(response.url()).searchParams.has('cursor'));
  await page.getByRole('button', { name: 'Ver más espacios' }).click();
  const secondPageResponse = await nextPage;
  expect(secondPageResponse.status()).toBe(200);
  const ascSecond = (await secondPageResponse.json()).items[0];
  await expect(page.getByRole('link', { name: ascSecond.title })).toBeVisible();
  expect(new URL(page.url()).searchParams.has('cursor')).toBe(true);

  const descResponse = page.waitForResponse(response => response.request().method() === 'GET'
    && /\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url())
    && new URL(response.url()).searchParams.get('sort') === 'title_desc');
  await page.locator('.catalog-sort select').selectOption('title_desc');
  const desc = await descResponse;
  expect(desc.status()).toBe(200);
  const descFirst = (await desc.json()).items[0];
  expect(new URL(desc.url()).searchParams.has('cursor')).toBe(false);
  await expect(page).toHaveURL(/sort=title_desc/);
  await expect(page.getByRole('link', { name: descFirst.title })).toBeVisible();
  expect((await desc.request().allHeaders()).authorization).toBeUndefined();

  await page.evaluate(() => {
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
    window.scrollTo(0, 0);
  });
  await takeScreenshot(page, 'fe14_mpv2_10_sort_desktop', true);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole('link', { name: descFirst.title })).toBeVisible();
  await page.evaluate(() => {
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
    window.scrollTo(0, 0);
  });
  await takeScreenshot(page, 'fe14_mpv2_10_sort_mobile', true);

  await page.goBack();
  await expect(page).toHaveURL(/sort=title_asc.*cursor=|cursor=.*sort=title_asc/);
  await expect(page.getByRole('link', { name: ascSecond.title })).toBeVisible();
  await page.goForward();
  await expect(page).toHaveURL(/sort=title_desc/);
  await expect(page.getByRole('link', { name: descFirst.title })).toBeVisible();
});

test('MPV2-10 applies a pending price order after completing its tariff requirement', async ({ page }) => {
  await page.setViewportSize({ width: 1365, height: 900 });
  await page.goto('/explorar?category_code=oficina&page_size=20');
  await expect(page.getByRole('link', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Segundo espacio público FE-14' })).toBeVisible();

  await page.locator('.catalog-sort select').selectOption('unit_price_asc');
  await expect(page.getByText(/elige explícitamente hora, día o mes/)).toBeVisible();
  await page.locator('#rate-unit').selectOption('hora');
  const applyPrice = page.getByRole('button', { name: 'Aplicar y ordenar' });
  await expect(applyPrice).toBeVisible();
  const ascendingResponse = page.waitForResponse(response => response.request().method() === 'GET'
    && /\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url())
    && new URL(response.url()).searchParams.get('sort') === 'unit_price_asc');
  await applyPrice.click();
  const ascending = await ascendingResponse;
  expect(ascending.status()).toBe(200);
  const ascendingUrl = new URL(ascending.url());
  expect(ascendingUrl.searchParams.get('rate_unit')).toBe('hora');
  expect(ascendingUrl.searchParams.get('category_code')).toBe('oficina');
  expect(ascendingUrl.searchParams.has('cursor')).toBe(false);
  const ascendingItems = (await ascending.json()).items;
  expect(ascendingItems.length).toBeGreaterThan(1);
  expect(new Set(ascendingItems.map(item => item.base_price_clp)).size).toBeGreaterThan(1);
  expect(ascendingItems.every(item => item.rate_unit === 'hora')).toBe(true);
  expect(ascendingItems[0].base_price_clp).toBeLessThan(ascendingItems.at(-1).base_price_clp);
  await expect(page.getByRole('link', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Segundo espacio público FE-14' })).toBeVisible();
  await expect(page).toHaveURL(/sort=unit_price_asc.*rate_unit=hora|rate_unit=hora.*sort=unit_price_asc/);

  const descendingResponse = page.waitForResponse(response => response.request().method() === 'GET'
    && /\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url())
    && new URL(response.url()).searchParams.get('sort') === 'unit_price_desc');
  await page.locator('.catalog-sort select').selectOption('unit_price_desc');
  const descending = await descendingResponse;
  expect(descending.status()).toBe(200);
  const descendingItems = (await descending.json()).items;
  expect(descendingItems.map(item => item.space_id)).toEqual(ascendingItems.map(item => item.space_id).reverse());
});

test('Removing applied filter chips normalizes dependencies and supports browser history', async ({ page }) => {
  await page.setViewportSize({ width: 1365, height: 900 });
  const publicSearch = response => response.request().method() === 'GET' && /\/api\/v1\/public\/spaces(?:\?|$)/.test(response.url());

  await page.goto('/explorar?q=oficina&sort=relevance');
  await expect(page.getByRole('button', { name: 'Quitar filtro Búsqueda: oficina' })).toBeVisible();
  let responsePromise = page.waitForResponse(response => publicSearch(response) && !new URL(response.url()).searchParams.has('q'));
  await page.getByRole('button', { name: 'Quitar filtro Búsqueda: oficina' }).click();
  let response = await responsePromise;
  expect(response.status()).toBe(200);
  let url = new URL(response.url());
  expect(url.searchParams.has('q')).toBe(false);
  expect(new URL(page.url()).searchParams.has('sort')).toBe(false);
  expect((await response.request().allHeaders()).authorization).toBeUndefined();
  await expect(page).toHaveURL('/explorar');
  await page.goBack();
  await expect(page).toHaveURL(/q=oficina.*sort=relevance|sort=relevance.*q=oficina/);
  await expect(page.getByRole('button', { name: 'Quitar filtro Búsqueda: oficina' })).toBeVisible();
  await page.goForward();
  await expect(page).toHaveURL('/explorar');

  await page.goto('/explorar?rate_unit=hora&min_unit_price_clp=10000&sort=unit_price_asc');
  await expect(page.getByRole('button', { name: 'Quitar filtro Tarifa: Hora' })).toBeVisible();
  responsePromise = page.waitForResponse(response => publicSearch(response) && !new URL(response.url()).searchParams.has('rate_unit'));
  await page.getByRole('button', { name: 'Quitar filtro Tarifa: Hora' }).click();
  response = await responsePromise;
  expect(response.status()).toBe(200);
  url = new URL(response.url());
  for (const key of ['rate_unit', 'min_unit_price_clp', 'cursor']) expect(url.searchParams.has(key)).toBe(false);
  for (const key of ['rate_unit', 'min_unit_price_clp', 'sort', 'cursor']) expect(new URL(page.url()).searchParams.has(key)).toBe(false);
  await expect(page).toHaveURL('/explorar');
  await page.goBack();
  await expect(page).toHaveURL(/rate_unit=hora/);
  await page.goForward();
  await expect(page).toHaveURL('/explorar');

  await page.goto('/explorar?rate_unit=hora&start_at=2026-10-20T10%3A00%3A00.000Z&end_at=2026-10-20T11%3A00%3A00.000Z&min_total_clp=18000&sort=estimated_total_asc');
  await expect(page.getByRole('button', { name: /Quitar filtro Desde:/ })).toBeVisible();
  responsePromise = page.waitForResponse(response => publicSearch(response) && !new URL(response.url()).searchParams.has('start_at'));
  await page.getByRole('button', { name: /Quitar filtro Desde:/ }).click();
  response = await responsePromise;
  expect(response.status()).toBe(200);
  url = new URL(response.url());
  for (const key of ['start_at', 'end_at', 'min_total_clp', 'cursor']) expect(url.searchParams.has(key)).toBe(false);
  for (const key of ['start_at', 'end_at', 'min_total_clp', 'sort', 'cursor']) expect(new URL(page.url()).searchParams.has(key)).toBe(false);
  expect(url.searchParams.get('rate_unit')).toBe('hora');
  await expect(page).toHaveURL(/rate_unit=hora/);
  await page.goBack();
  await expect(page).toHaveURL(/start_at=/);
  await page.goForward();
  await expect(page).toHaveURL(/rate_unit=hora/);
});

test('Mobile filter dialog preserves applied chips, focus, and closes on desktop resize', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/explorar?q=publico');
  const trigger = page.getByRole('button', { name: 'Filtrar' });
  await trigger.click();
  const dialog = page.getByRole('dialog', { name: 'Filtros de espacios' });
  await expect(dialog).toBeVisible();
  await expect(page.locator('.filter-close')).toBeFocused();
  await page.keyboard.press('Shift+Tab');
  await expect(page.getByRole('button', { name: 'Limpiar filtros' })).toBeFocused();
  await page.keyboard.press('Tab');
  await expect(page.locator('.filter-close')).toBeFocused();

  await dialog.getByLabel('Buscar', { exact: true }).fill('borrador de búsqueda');
  await expect(page.getByText('Tienes cambios sin aplicar.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Quitar filtro Búsqueda: publico' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expect(trigger).toBeFocused();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

  await trigger.click();
  await page.setViewportSize({ width: 1365, height: 900 });
  await expect(dialog).toHaveCount(0);
  await expect(page.locator('.filters input').first()).toBeFocused();
  const bounds = await page.evaluate(() => ({
    filters: document.querySelector('.filter-sidebar').getBoundingClientRect(),
    results: document.querySelector('.catalog-results').getBoundingClientRect(),
  }));
  expect(bounds.filters.right).toBeLessThanOrEqual(bounds.results.left);
  expect(bounds.results.top).toBeLessThan(bounds.filters.bottom);
});

test('Contextual registration and verification preserve the pending return until login', async ({ page }) => {
  await page.goto('/explorar?q=publico&category_code=oficina&attribute=wifi:true&attribute=puestos_trabajo:gte:4&attribute=climatizacion:in:calefaccion,aire_acondicionado&page_size=20');
  await page.getByRole('link', { name: 'Espacio sintético público FE-14' }).first().click();
  await expect(page).toHaveURL(/\/espacios\/[0-9a-f-]+/);
  const spaceId = new URL(page.url()).pathname.split('/').at(-1);
  await page.locator('.booking a.button').click();
  await expect(page).toHaveURL(/\/acceso\?returnUrl=/);
  const pendingReturn = new URL(page.url()).searchParams.get('returnUrl');
  await page.getByRole('link', { name: 'Crear cuenta' }).click();
  await expect(page).toHaveURL(/\/registro\?returnUrl=/);
  expect(new URL(page.url()).searchParams.get('returnUrl')).toBe(pendingReturn);

  const email = `mpv2-06-${Date.now()}@ejemplo.invalid`;
  const password = 'Context#12345';
  await expect(page.getByRole('heading', { name: 'Crea tu cuenta' })).toBeVisible();
  await page.getByLabel('Correo electrónico').fill(email);
  await page.getByLabel('Contraseña', { exact: true }).fill(password);
  await page.getByLabel('Repite la contraseña').fill(password);
  await page.getByRole('radio', { name: 'Arrendar un espacio' }).check();
  for (const checkbox of await page.getByRole('checkbox').all()) await checkbox.check();
  await page.getByRole('button', { name: 'Crear cuenta' }).click();
  await expect(page.getByRole('status')).toContainText('Go creó la cuenta');
  await expect(page).toHaveURL(/\/registro\?returnUrl=/);

  await page.getByRole('link', { name: 'Verificar el correo' }).click();
  await expect(page).toHaveURL(/\/verificar-correo\?returnUrl=/);
  expect(new URL(page.url()).searchParams.get('returnUrl')).toBe(pendingReturn);
  const mailListResponse = await page.request.get('http://mailpit:8025/api/v1/messages');
  expect(mailListResponse.ok()).toBeTruthy();
  const messages = (await mailListResponse.json()).messages || [];
  const message = messages.find(item => (item.To || []).some(recipient => (recipient.Address || '').toLowerCase() === email.toLowerCase()));
  expect(message).toBeTruthy();
  const messageResponse = await page.request.get(`http://mailpit:8025/api/v1/message/${message.ID}`);
  expect(messageResponse.ok()).toBeTruthy();
  const messageBody = (await messageResponse.json()).Text || '';
  const tokenId = /^Token ID: ([0-9a-f-]+)$/m.exec(messageBody)?.[1];
  const token = /^Token: ([A-Za-z0-9_-]+)$/m.exec(messageBody)?.[1];
  expect(tokenId).toBeTruthy();
  expect(token).toBeTruthy();
  await page.getByLabel('Identificador del correo').fill(tokenId);
  await page.getByLabel('Código de verificación').fill(token);
  await page.getByRole('button', { name: 'Verificar correo' }).click();
  await expect(page.getByRole('status')).toContainText('Correo verificado por Go');
  await expect(page).toHaveURL(/\/verificar-correo\?returnUrl=/);

  await page.getByRole('link', { name: 'Inicia sesión para volver al espacio' }).click();
  await page.getByLabel('Correo electrónico').fill(email);
  await page.getByLabel('Contraseña').fill(password);
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();
  await expect(page).toHaveURL(new RegExp(`/espacios/${spaceId}\\?context=returned&returnTo=`));
  await expect(page.getByText(/Volviste a esta ficha/)).toBeVisible();
  expect(await page.evaluate(() => [localStorage.length, sessionStorage.length])).toEqual([0, 0]);
  expect(page.url()).not.toMatch(/bearer|token|password/i);
});

test('Contextual login returns to the selected space and requires explicit quote and reservation confirmation', async ({ page }) => {
  await page.setViewportSize({ width: 1365, height: 900 });
  const bookingWrites = [];
  page.on('request', request => {
    if (request.method() !== 'POST') return;
    const pathname = new URL(request.url()).pathname;
    if (/\/local\/booking-trial\/(quotes|reservations)$/.test(pathname)) bookingWrites.push(pathname);
  });

  await page.goto('/explorar?q=publico&category_code=oficina&attribute=wifi:true&attribute=puestos_trabajo:gte:4&attribute=climatizacion:in:calefaccion,aire_acondicionado&page_size=20');
  const listing = page.getByRole('link', { name: 'Espacio sintético público FE-14' }).first();
  await expect(listing).toBeVisible();
  await listing.click();
  await expect(page.getByRole('heading', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  await expect(page).toHaveURL(/\/espacios\/[0-9a-f-]+/);
  const detailId = new URL(page.url()).pathname.split('/').at(-1);
  const access = page.locator('.booking a.button');
  const accessUrl = new URL(await access.getAttribute('href'), page.url());
  const returnUrl = accessUrl.searchParams.get('returnUrl');
  expect(returnUrl).toContain(`/espacios/${detailId}?action=reserve&returnTo=`);
  expect(returnUrl).toContain('q%3Dpublico');
  expect(returnUrl).not.toContain('view%3D');
  expect(returnUrl).not.toContain('desktop_columns');
  expect(returnUrl).not.toContain('mobile_columns');
  expect(returnUrl).not.toContain('view%3D');
  expect(returnUrl).toContain('page_size%3D20');
  expect(returnUrl).toContain('attribute%3Dclimatizacion%253Ain%253Aaire_acondicionado%252Ccalefaccion');
  expect(returnUrl).toContain('attribute%3Dpuestos_trabajo%253Agte%253A4');
  expect(returnUrl).toContain('attribute%3Dwifi%253Atrue');
  expect(returnUrl).not.toMatch(/bearer|token|password/i);

  await access.click();
  await expect(page.getByRole('heading', { name: 'Inicia sesión' })).toBeVisible();
  const cancelListingHref = await page.getByRole('link', { name: 'Cancelar y volver al catálogo' }).getAttribute('href');
  const cancelListingUrl = new URL(cancelListingHref, page.url());
  expect(cancelListingUrl.pathname).toBe('/explorar');
  expect(cancelListingUrl.searchParams.has('view')).toBe(false);
  expect(cancelListingUrl.searchParams.has('desktop_columns')).toBe(false);
  expect(cancelListingUrl.searchParams.has('mobile_columns')).toBe(false);
  expect(cancelListingUrl.searchParams.get('page_size')).toBe('20');
  await page.getByRole('link', { name: 'Cancelar y volver al catálogo' }).click();
  await expect(page).toHaveURL(/\/explorar\?/);
  expect(new URL(page.url()).searchParams.has('view')).toBe(false);
  await page.getByRole('link', { name: 'Espacio sintético público FE-14' }).first().click();
  await expect(page.getByRole('heading', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`/espacios/${detailId}`));
  await page.locator('.booking a.button').click();
  await expect(page).toHaveURL(/\/acceso\?returnUrl=/);
  await page.getByLabel('Correo electrónico').fill(accounts.renter.email);
  await page.getByLabel('Contraseña').fill(accounts.renter.password);
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();

  await expect(page).toHaveURL(new RegExp(`/espacios/${detailId}\\?context=returned&returnTo=`));
  await expect(page.getByText(/Volviste a esta ficha/)).toBeVisible();
  await expect(page.getByLabel('Horario disponible')).toBeVisible();
  await expect.poll(() => page.getByLabel('Horario disponible').locator('option').count()).toBeGreaterThan(1);
  expect(bookingWrites).toEqual([]);
  expect(await page.evaluate(() => [localStorage.length, sessionStorage.length])).toEqual([0, 0]);
  const activeUrl = new URL(page.url());
  expect(activeUrl.search).not.toMatch(/bearer|token|password/i);
  await takeScreenshot(page, 'fe14_mpv2_06_context_desktop');

  await page.setViewportSize({ width: 390, height: 844 });
  await takeScreenshot(page, 'fe14_mpv2_06_context_mobile');
  await page.setViewportSize({ width: 1365, height: 900 });

  await page.getByLabel('Horario disponible').selectOption('0');
  const quoteResponsePromise = page.waitForResponse(response => response.request().method() === 'POST' && /\/api\/v1\/local\/booking-trial\/quotes$/.test(new URL(response.url()).pathname));
  await page.getByRole('button', { name: 'Cotizar horario' }).click();
  const quoteResponse = await quoteResponsePromise;
  expect(quoteResponse.status()).toBe(200);
  expect(bookingWrites).toEqual(['/api/v1/local/booking-trial/quotes']);
  await expect(page.getByRole('button', { name: 'Solicitar reserva' })).toBeVisible();

  const reservationResponsePromise = page.waitForResponse(response => response.request().method() === 'POST' && /\/api\/v1\/local\/booking-trial\/reservations$/.test(new URL(response.url()).pathname));
  await expect(page.getByRole('button', { name: 'Solicitar reserva' })).toBeEnabled();
  await page.getByRole('button', { name: 'Solicitar reserva' }).click();
  const reservationResponse = await reservationResponsePromise;
  expect(reservationResponse.status()).toBe(201);
  await expect(page).toHaveURL(/\/reservas\/[0-9a-f-]+$/);
  expect(bookingWrites).toEqual([
    '/api/v1/local/booking-trial/quotes',
    '/api/v1/local/booking-trial/reservations',
  ]);
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
  const searchHttpResponse = await searchResponse;
  expect(searchHttpResponse.status()).toBe(200);
  expect(searchRequest).toBeTruthy();
  expect((await searchRequest.allHeaders()).authorization).toBeUndefined();
  const publicSearchItems = (await searchHttpResponse.json()).items;
  expect(publicSearchItems.find(item => item.space_id === draft.id).cover_image_id).toBe(photo2Id);
  await expect(visitor.getByRole('link', { name: title })).toBeVisible();
  const cardCover = visitor.getByRole('img', { name: `Fotografía de portada de ${title}` });
  await expect(cardCover).toBeVisible();
  await expect.poll(() => cardCover.evaluate(image => image.naturalWidth)).toBeGreaterThan(0);
  await expect(visitor.getByText('Providencia, Metropolitana de Santiago')).toBeVisible();
  await takeScreenshot(visitor, 'fe14_mpv2_04_visitor_search_desktop', true);
  await visitor.setViewportSize({ width: 390, height: 844 });
  await takeScreenshot(visitor, 'fe14_mpv2_04_visitor_search_mobile', true);
  await visitor.setViewportSize({ width: 1365, height: 900 });
  const imageResponsePromise=visitor.waitForResponse(response=>response.request().method()==='GET'&&response.url().includes(`/public/spaces/${draft.id}/images/${photo2Id}`));
  const publicDetailResponsePromise=visitor.waitForResponse(response=>response.request().method()==='GET'&&response.url().endsWith(`/api/v1/public/spaces/${draft.id}`));
  await visitor.getByRole('link', { name: title }).click();
  const publicDetailResponse=await publicDetailResponsePromise;
  expect(publicDetailResponse.status()).toBe(200);
  const publicDetailPayload = await visitor.request.get(`http://backend:8080/api/v1/public/spaces/${draft.id}`);
  expect(publicDetailPayload.status()).toBe(200);
  const detailDTO=(await publicDetailPayload.json()).data;
  expect(detailDTO.images).toEqual([{image_id:photo2Id,is_cover:true},{image_id:photoId,is_cover:false}]);
  expect(JSON.stringify(detailDTO.images)).not.toContain(privatePhotoId);
  expect(detailDTO.usage_rules).toBe('Prueba de publicación sintética.');
  expect(JSON.stringify(detailDTO)).not.toContain('Dirección confidencial');
  const origin = process.env.FE14_PUBLIC_ORIGIN;
  const frontendBase = process.env.FRONTEND_BASE_URL || 'http://angular-frontend:4000';
  const ssrDetailResponse = await visitor.request.get(`${frontendBase}/espacios/${draft.id}`);
  expect(ssrDetailResponse.status()).toBe(200);
  const ssrDetailHtml = await ssrDetailResponse.text();
  expect(ssrDetailHtml).toContain(title);
  expect(ssrDetailHtml).toContain(`<link rel="canonical" href="${origin}/espacios/${draft.id}">`);
  expect(ssrDetailHtml).toContain(`${origin}/public/spaces/${draft.id}/images/${photo2Id}`);
  expect(ssrDetailHtml).not.toContain('http://backend:8080');
  expect(ssrDetailHtml).not.toContain('Dirección privada de prueba');
  const socialImagePath = new URL(`${origin}/public/spaces/${draft.id}/images/${photo2Id}`).pathname;
  const socialImageResponse = await visitor.request.get(`${frontendBase}${socialImagePath}`);
  expect(socialImageResponse.status()).toBe(200);
  expect(socialImageResponse.headers()['content-type']).toContain('image/png');
  expect(socialImageResponse.headers()['cache-control']).toBe('no-store');

  // Exercise the documented `npm start` path against the same live Go fixture,
  // using its local development config and explicit SSR-only overrides.
  const localSsr = process.env.FE14_LOCAL_SSR_ORIGIN;
  const localCanonicalOrigin = process.env.FE14_LOCAL_SSR_CANONICAL_ORIGIN;
  expect(localSsr).toBeTruthy();
  expect(localCanonicalOrigin).toBeTruthy();
  const localConfigResponse = await visitor.request.get(`${localSsr}/config.json`);
  expect(localConfigResponse.status()).toBe(200);
  expect(await localConfigResponse.json()).toEqual({
    apiBaseUrl: 'http://localhost:8080/api/v1',
    siteOrigin: 'http://localhost:4200',
  });
  const localSpoofedHost = await visitor.request.get(`${localSsr}/explorar`, {
    headers: { host: 'host-header-attacker.invalid' },
  });
  expect(localSpoofedHost.status()).toBe(403);
  const localCatalogResponse = await visitor.request.get(`${localSsr}/explorar`);
  expect(localCatalogResponse.status()).toBe(200);
  expect(await localCatalogResponse.text()).toContain(title);
  const localDetailResponse = await visitor.request.get(`${localSsr}/espacios/${draft.id}`);
  expect(localDetailResponse.status()).toBe(200);
  const localDetailHtml = await localDetailResponse.text();
  expect(localDetailHtml).toContain(title);
  expect(localDetailHtml).toContain(`<link rel="canonical" href="${localCanonicalOrigin}/espacios/${draft.id}">`);
  expect(localDetailHtml).toContain(`${localCanonicalOrigin}/public/spaces/${draft.id}/images/${photo2Id}`);
  expect(localDetailHtml).not.toContain('http://backend:8080');
  expect(localDetailHtml).not.toContain('Dirección confidencial MPV2-03');
  const localImage = await visitor.request.get(
    `${localSsr}/public/spaces/${draft.id}/images/${photo2Id}`,
  );
  expect(localImage.status()).toBe(200);
  expect(localImage.headers()['content-type']).toContain('image/png');
  expect(localImage.headers()['cache-control']).toBe('no-store');

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
  const coverWithdrawalResponse = host.waitForResponse(response=>response.request().method()==='PUT'&&response.url().endsWith(`/api/v1/spaces/${draft.id}/gallery/${photo2Id}/publication`));
  await host.getByRole('button',{name:`Ver imagen sintética ${photo2Id}`}).locator('xpath=ancestor::article').getByRole('button',{name:'Retirar de ficha pública'}).click();
  expect((await coverWithdrawalResponse).status()).toBe(200);
  const withdrawn=await visitor.request.get(imageResponse.url());
  expect(withdrawn.status()).toBe(404);
  const withdrawnSsr = await visitor.request.get(`${frontendBase}/espacios/${draft.id}`);
  expect(withdrawnSsr.status()).toBe(200);
  expect(await withdrawnSsr.text()).not.toContain(`${origin}/public/spaces/${draft.id}/images/${photo2Id}`);
  await visitor.reload();
  await expect(visitor.getByRole('heading',{name:title})).toBeVisible();
  await expect(visitor.locator('.gallery-image img')).toHaveCount(1);
  await expect.poll(()=>visitor.locator('.gallery-image img').evaluate(image=>image.naturalWidth)).toBeGreaterThan(0);
  await visitor.goBack();
  await expect(visitor).toHaveURL(/\/explorar/);
  await expect(visitor.getByRole('link', { name: title })).toBeVisible();
  await expect(visitor.locator('.space-card .space-cover')).toHaveCount(0);
  await expect(visitor.getByRole('img', { name: 'Sin fotografía pública' })).toBeVisible();
  await host.getByRole('button', { name: 'Datos', exact: true }).click();
  const hideResponsePromise = host.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith(`/api/v1/spaces/${draft.id}/publication`));
  await host.getByRole('button', { name: 'Ocultar', exact: true }).click();
  expect((await hideResponsePromise).status()).toBe(200);
  const unavailableSsr = await visitor.request.get(`${frontendBase}/espacios/${draft.id}`);
  expect(unavailableSsr.status()).toBe(404);
  const sitemapAfterWithdrawal = await visitor.request.get(`${frontendBase}/sitemap.xml`);
  expect(sitemapAfterWithdrawal.status()).toBe(200);
  expect(await sitemapAfterWithdrawal.text()).not.toContain(`/espacios/${draft.id}`);
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
  test.setTimeout(150_000);
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
