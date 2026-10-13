import {
  AngularNodeAppEngine,
  createNodeRequestHandler,
  isMainModule,
  writeResponseToNodeResponse,
} from '@angular/ssr/node';
import express from 'express';
import { join } from 'node:path';
import { configuredApiBase, configuredPublicOrigin } from './app/core/public-runtime-config';

const apiServerBase = configuredApiBase(process.env['API_INTERNAL_BASE_URL']);
const apiPublicBase = configuredApiBase(process.env['API_PUBLIC_BASE_URL']);
const publicOrigin = configuredPublicOrigin(process.env['FRONTEND_PUBLIC_ORIGIN']);
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

const browserDistFolder = join(import.meta.dirname, '../browser');

const app = express();
const angularApp = new AngularNodeAppEngine();

app.get('/config.json', (_request, response) => {
  response.setHeader('Cache-Control', 'no-store');
  response.json({
    apiBaseUrl: apiPublicBase,
    siteOrigin: publicOrigin,
  });
});

app.get('/robots.txt', (_request, response) => {
  response.setHeader('Cache-Control', 'no-store');
  response.type('text/plain; charset=utf-8');
  response.send([
    'User-agent: *',
    'Allow: /',
    'Disallow: /acceso',
    'Disallow: /registro',
    'Disallow: /verificar-correo',
    'Disallow: /recuperar-acceso',
    'Disallow: /mi-cuenta',
    'Disallow: /reservas',
    'Disallow: /panel',
    'Disallow: /administracion',
    ...(publicOrigin ? [`Sitemap: ${publicOrigin}/sitemap.xml`] : []),
    '',
  ].join('\n'));
});

app.get('/sitemap.xml', async (_request, response) => {
  response.setHeader('Cache-Control', 'no-store');
  response.type('application/xml; charset=utf-8');
  if (new URL(_request.originalUrl, 'http://server.invalid').search) {
    response.status(400).send('');
    return;
  }
  if (!apiServerBase || !publicOrigin) {
    response.status(503).send('');
    return;
  }
  try {
    const spaces = await visibleSpacesForSitemap(apiServerBase);
    const staticPaths = ['/', '/categorias', '/como-funciona', '/explorar'];
    const locations = [
      ...staticPaths.map((path) => `${publicOrigin}${path}`),
      ...spaces.map((spaceId) => `${publicOrigin}/espacios/${spaceId}`),
    ];
    const body = [
      '<?xml version="1.0" encoding="UTF-8"?>',
      '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
      ...locations.map((location) => `<url><loc>${xmlEscape(location)}</loc></url>`),
      '</urlset>',
    ].join('');
    response.status(200).send(body);
  } catch {
    response.status(503).send('');
  }
});

// Open Graph images use a same-origin public path. Go remains the authority
// for publication visibility and returns no-store for every image read.
app.get('/public/spaces/:spaceID/images/:imageID', async (request, response) => {
  response.setHeader('Cache-Control', 'no-store');
  response.setHeader('X-Content-Type-Options', 'nosniff');
  const { spaceID, imageID } = request.params;
  if (!uuidPattern.test(spaceID) || !uuidPattern.test(imageID) || !apiServerBase) {
    response.status(apiServerBase ? 404 : 503).end();
    return;
  }
  try {
    const url = `${apiServerBase}/public/spaces/${encodeURIComponent(spaceID)}/images/${encodeURIComponent(imageID)}`;
    const image = await fetch(url, {
      method: 'GET',
      headers: { Accept: 'image/png' },
      cache: 'no-store',
      redirect: 'error',
      signal: AbortSignal.timeout(5000),
    });
    if (image.status === 404) {
      response.status(404).end();
      return;
    }
    if (!image.ok) {
      response.status(503).end();
      return;
    }
    const mime = image.headers.get('content-type');
    const length = Number(image.headers.get('content-length'));
    if (mime !== 'image/png' || !Number.isSafeInteger(length) || length < 1 || length > 1_048_576) {
      response.status(404).end();
      return;
    }
    const bytes = Buffer.from(await image.arrayBuffer());
    if (bytes.length !== length || bytes.length > 1_048_576) {
      response.status(404).end();
      return;
    }
    response.setHeader('Content-Type', 'image/png');
    response.setHeader('Content-Length', String(bytes.length));
    response.setHeader('Content-Disposition', 'inline; filename=espacigo-public-image.png');
    response.status(200).send(bytes);
  } catch {
    response.status(503).end();
  }
});

app.get('/healthz', (_request, response) => {
  response.json({ status: 'ok' });
});

app.use((request, response, next) => {
  const path = new URL(request.originalUrl, 'http://server.invalid').pathname;
  if (/^\/explorar\/?$/.test(path) || /^\/espacios\/[^/]+\/?$/.test(path)) {
    response.setHeader('Cache-Control', 'no-store');
  }
  if (/^\/(acceso|registro|verificar-correo|recuperar-acceso|mi-cuenta|reservas|panel|panel-anfitrion|administracion)(\/|$)/.test(path)) {
    response.setHeader('Cache-Control', 'no-store');
    response.setHeader('X-Robots-Tag', 'noindex, nofollow');
  }
  next();
});

/**
 * Example Express Rest API endpoints can be defined here.
 * Uncomment and define endpoints as necessary.
 *
 * Example:
 * ```ts
 * app.get('/api/{*splat}', (req, res) => {
 *   // Handle API request
 * });
 * ```
 */

/**
 * Serve static files from /browser
 */
app.use(
  express.static(browserDistFolder, {
    maxAge: '1y',
    index: false,
    redirect: false,
  }),
);

/**
 * Handle all other requests by rendering the Angular application.
 */
app.use((req, res, next) => {
  angularApp
    .handle(req)
    .then((response) => (response ? writeResponseToNodeResponse(response, res) : next()))
    .catch(next);
});

/**
 * Start the server if this module is the main entry point, or it is ran via PM2.
 * The server listens on the port defined by the `PORT` environment variable, or defaults to 4000.
 */
if (isMainModule(import.meta.url) || process.env['pm_id']) {
  const port = process.env['PORT'] || 4000;
  app.listen(port, (error) => {
    if (error) {
      throw error;
    }

    console.log(`Node Express server listening on http://localhost:${port}`);
  });
}

/**
 * Request handler used by the Angular CLI (for dev-server and during build) or Firebase Cloud Functions.
 */
export const reqHandler = createNodeRequestHandler(app);

async function visibleSpacesForSitemap(apiBase: string): Promise<string[]> {
  const maxPages = 2000;
  const maxSpaces = 49_996;
  const ids: string[] = [];
  const seenIDs = new Set<string>();
  const cursors = new Set<string>();
  let cursor = '';
  const controller = new AbortController();
  const deadline = setTimeout(() => controller.abort(), 25_000);
  try {
    for (let pageNumber = 0; pageNumber < maxPages; pageNumber++) {
      const url = new URL(`${apiBase}/public/spaces`);
      url.searchParams.set('page_size', '25');
      url.searchParams.set('sort', 'default');
      if (cursor) url.searchParams.set('cursor', cursor);
      const result = await fetch(url, {
        headers: { Accept: 'application/json' },
        cache: 'no-store',
        redirect: 'error',
        signal: controller.signal,
      });
      if (!result.ok) throw new Error('Public catalog is unavailable');
      const page: unknown = await result.json();
      if (!page || typeof page !== 'object' || !Array.isArray((page as { items?: unknown }).items)) {
        throw new Error('Invalid public catalog response');
      }
      const value = page as { items: Array<{ space_id?: unknown }>; next_cursor?: unknown };
      for (const item of value.items) {
        if (typeof item?.space_id !== 'string' || !uuidPattern.test(item.space_id)) {
          throw new Error('Invalid public space identifier');
        }
        const id = item.space_id.toLowerCase();
        if (seenIDs.has(id)) continue;
        seenIDs.add(id);
        ids.push(id);
        if (ids.length > maxSpaces) throw new Error('Sitemap size limit exceeded');
      }
      if (value.next_cursor === undefined || value.next_cursor === '') return ids;
      if (typeof value.next_cursor !== 'string' || value.next_cursor.length > 4096 || cursors.has(value.next_cursor)) {
        throw new Error('Invalid public catalog cursor');
      }
      cursor = value.next_cursor;
      cursors.add(cursor);
    }
    throw new Error('Sitemap page limit exceeded');
  } finally {
    clearTimeout(deadline);
  }
}

function xmlEscape(value: string): string {
  return value.replace(/[&<>"']/g, (character) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&apos;',
  })[character]!);
}
