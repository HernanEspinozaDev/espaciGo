const path = require('node:path');
const fs = require('node:fs');
const { test, expect } = require('@playwright/test');
const artifactDirectory = process.env.UX09_ARTIFACT_DIR;
if (artifactDirectory) fs.mkdirSync(artifactDirectory, { recursive: true });

const publicRoutes = [
  { path: '/', title: /Un buen espacio/ },
  { path: '/categorias', title: /El lugar adecuado/ },
  { path: '/como-funciona', title: /Espacios flexibles/ },
  { path: '/explorar', title: /Encuentra tu próximo espacio/ },
  { path: '/terminos-y-condiciones', title: /Documento no disponible|Términos/ },
  { path: '/politica-de-privacidad', title: /Documento no disponible|Privacidad/ },
  { path: '/acceso', title: /Bienvenido de nuevo/ },
  { path: '/registro', title: /Haz espacio/ },
  { path: '/verificar-correo', title: /Verifica tu correo/ },
  { path: '/recuperar-acceso', title: /contraseña/ },
];

function installSyntheticRouting(page, baseOrigin, externalRequests) {
  page.route('**/*', async (route) => {
    const url = new URL(route.request().url());
    if (url.origin !== baseOrigin) {
      externalRequests.push(`${url.origin}${url.pathname}`);
      return route.abort();
    }
    if (url.pathname === '/config.json') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ apiBaseUrl: '/api/v1', siteOrigin: baseOrigin }),
      });
    }
    if (url.pathname === '/health/ready') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'ready', synthetic: true }),
      });
    }
    if (url.pathname.startsWith('/api/v1/')) {
      return route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'synthetic_unavailable', message: 'Respuesta sintética no disponible.' } }),
      });
    }
    return route.continue();
  });
}

for (const routeCase of publicRoutes) {
  test(`UX09 public route ${routeCase.path} stays usable at desktop and mobile`, async ({ page }, testInfo) => {
    const errors = [];
    const externalRequests = [];
    const baseOrigin = new URL(testInfo.project.use.baseURL || 'http://angular-frontend:4000').origin;
    page.on('pageerror', (error) => errors.push(error.message));

    installSyntheticRouting(page, baseOrigin, externalRequests);

    for (const width of [1365, 390]) {
      await page.setViewportSize({ width, height: width < 500 ? 844 : 900 });
      await page.goto(routeCase.path);
      const heading = page.locator('#contenido h1');
      await expect(heading).toBeVisible();
      await expect(heading).toContainText(routeCase.title);

      const dimensions = await page.evaluate(() => ({
        viewport: innerWidth,
        document: document.documentElement.scrollWidth,
        body: document.body.scrollWidth,
      }));
      expect(dimensions.document, `${routeCase.path} at ${width}px`).toBeLessThanOrEqual(width);
      expect(dimensions.body, `${routeCase.path} at ${width}px`).toBeLessThanOrEqual(width);

      await page.addScriptTag({ path: require.resolve('axe-core') });
      const accessibility = await page.evaluate(async () => axe.run(document, {
        runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'] },
      }));
      expect(accessibility.violations.map(({ id }) => id), `${routeCase.path} at ${width}px`).toEqual([]);

      if (artifactDirectory) {
        await page.screenshot({
          path: path.join(artifactDirectory, `${routeCase.path === '/' ? 'inicio' : routeCase.path.slice(1).replaceAll('/', '-')}-${width}.png`),
          fullPage: true,
        });
      }
    }

    expect(errors, routeCase.path).toEqual([]);
    expect(externalRequests, routeCase.path).toEqual([]);
  });
}

test('UX09 never applies synthetic API responses to external origins', async ({ page }, testInfo) => {
  const externalRequests = [];
  const baseOrigin = new URL(testInfo.project.use.baseURL || 'http://angular-frontend:4000').origin;
  installSyntheticRouting(page, baseOrigin, externalRequests);

  const results = await page.evaluate(async () => Promise.all([
    fetch('https://outside.invalid/health/ready'),
    fetch('https://outside.invalid/api/v1/public/listings'),
  ].map(async (request) => {
    try {
      return (await request).status;
    } catch {
      return 'blocked';
    }
  })));

  expect(results).toEqual(['blocked', 'blocked']);
  expect(externalRequests).toEqual([
    'https://outside.invalid/health/ready',
    'https://outside.invalid/api/v1/public/listings',
  ]);
});
