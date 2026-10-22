import {
  AngularNodeAppEngine,
  createNodeRequestHandler,
  isMainModule,
  writeResponseToNodeResponse,
} from '@angular/ssr/node';
import express from 'express';
import { Readable } from 'node:stream';
import type { ReadableStream as NodeReadableStream } from 'node:stream/web';
import { pipeline } from 'node:stream/promises';
import { join } from 'node:path';
import { readBoundedRequestBody } from './bounded-request-body.mjs';
import { configuredApiBase, configuredPublicApiBase, configuredPublicOrigin } from './app/core/public-runtime-config';
import { apiRequestAllowed, clientApiHeaders, iapAssertionRequired, iapRequestValid, privateEntryAudienceConfigured, publicCacheControl, publicIdentityPostLimit, requestUrlWithinLimit, serviceAudienceMatches } from './server-api-gateway.mjs';
import { cloudRunIdentityHeaders } from './cloud-run-identity.mjs';

const apiServerBase = configuredApiBase(process.env['API_INTERNAL_BASE_URL']);
const apiPublicBase = configuredPublicApiBase(process.env['API_PUBLIC_BASE_URL']);
const publicOrigin = configuredPublicOrigin(process.env['FRONTEND_PUBLIC_ORIGIN']);
const entryMode = process.env['FRONTEND_ENTRY_MODE'] ?? 'local';
const apiServiceAudience = process.env['API_SERVICE_AUDIENCE']?.trim() ?? '';
const iapJwtAudience = process.env['IAP_JWT_AUDIENCE']?.trim() ?? '';
if (!['public', 'private', 'local'].includes(entryMode)) throw new Error('Invalid frontend entry mode');
if (process.env['K_SERVICE'] && !apiServiceAudience && apiServerBase) {
  throw new Error('API_SERVICE_AUDIENCE is required for Cloud Run service-to-service authentication');
}
if (process.env['K_SERVICE'] && !['public', 'private'].includes(entryMode)) {
  throw new Error('FRONTEND_ENTRY_MODE must be public or private on Cloud Run');
}
if (!privateEntryAudienceConfigured(entryMode, iapJwtAudience)) {
  throw new Error('IAP_JWT_AUDIENCE must be the signed-header audience for the private Cloud Run service');
}
if (process.env['K_SERVICE']) {
  if (!apiServerBase || !apiPublicBase || apiPublicBase !== '/api/v1') {
    throw new Error('Cloud Run requires an internal API and the same-origin /api/v1 gateway');
  }
  if (!serviceAudienceMatches(apiServerBase, apiServiceAudience)) {
    throw new Error('Cloud Run API service audience must match the HTTPS API service origin');
  }
}
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

app.use(async (request, response, next) => {
  const pathname = new URL(request.originalUrl, 'http://server.invalid').pathname;
  if (iapAssertionRequired(entryMode, pathname)) {
    const assertion = request.get('x-goog-iap-jwt-assertion');
    if (!await iapRequestValid(entryMode, pathname, assertion, iapJwtAudience)) {
      response.status(401).end();
      return;
    }
  }
  return next();
});

app.get('/health/ready', async (_request, response) => {
  if (!apiServerBase) {
    response.status(503).json({ status: 'unavailable' });
    return;
  }
  try {
    const apiResponse = await fetchApi(`${apiServerBase.replace(/\/api\/v1\/?$/, '')}/health/ready`, {
      method: 'GET',
      headers: { Accept: 'application/json' },
      redirect: 'error',
      signal: AbortSignal.timeout(5000),
    });
    response.status(apiResponse.ok ? 200 : 503).json({ status: apiResponse.ok ? 'ready' : 'unavailable' });
  } catch {
    response.status(503).json({ status: 'unavailable' });
  }
});

app.use('/api/v1', async (request, response) => {
  if (!requestUrlWithinLimit(entryMode, request.originalUrl)) {
    response.status(414).end();
    return;
  }
  if (!apiServerBase || !apiRequestAllowed(entryMode, request.method, request.originalUrl, request.headers, publicOrigin)) {
    response.status(entryMode === 'public' ? 404 : 503).end();
    return;
  }
  try {
    const target = new URL(apiServerBase);
    const incoming = new URL(request.originalUrl, 'http://frontend.invalid');
    const basePath = target.pathname.replace(/\/$/, '');
    target.pathname = `${basePath}${incoming.pathname.slice('/api/v1'.length)}`;
    target.search = incoming.search;
    const headers = clientApiHeaders(entryMode, request.headers, request.originalUrl);
    if (entryMode === 'public') headers.set('accept', 'application/json');
    const serverIdentity = await serverIdentityHeaders();
    for (const [name, value] of Object.entries(serverIdentity)) headers.set(name, value);
    const method = request.method.toUpperCase();
    const init: RequestInit & { duplex?: 'half' } = {
      method,
      headers,
      redirect: 'manual',
      signal: AbortSignal.timeout(entryMode === 'public' ? 10_000 : 120_000),
    };
    if (!['GET', 'HEAD'].includes(method)) {
      const publicBodyLimit = entryMode === 'public'
        ? publicIdentityPostLimit(method, request.originalUrl)
        : null;
      if (publicBodyLimit !== null) {
        const body = await readBoundedRequestBody(request, publicBodyLimit);
        if (!body) {
          response.status(413).end();
          return;
        }
        init.body = Uint8Array.from(body).buffer;
      } else {
        init.body = request as unknown as BodyInit;
        init.duplex = 'half';
      }
    }
    const upstream = await fetch(target, init);
    response.status(upstream.status);
    response.setHeader('Cache-Control', publicCacheControl(entryMode, upstream.headers.get('cache-control') ?? undefined));
    if (entryMode === 'public') {
      response.setHeader('Content-Type', 'application/json; charset=utf-8');
      response.setHeader('X-Content-Type-Options', 'nosniff');
    }
    for (const name of ['content-disposition', 'content-length', 'content-type', 'etag', 'last-modified', 'retry-after', 'vary']) {
      const value = upstream.headers.get(name);
      if (value) response.setHeader(name, value);
    }
    if (!upstream.body || method === 'HEAD') {
      response.end();
      return;
    }
    if (entryMode === 'public') {
      const maxBytes = 2 * 1024 * 1024;
      const declaredLength = Number(upstream.headers.get('content-length'));
      if (Number.isFinite(declaredLength) && declaredLength > maxBytes) {
        response.removeHeader('Content-Length');
        response.status(502).end();
        return;
      }
      const body = await readBoundedBody(upstream.body, maxBytes);
      if (!body) {
        response.removeHeader('Content-Length');
        response.status(502).end();
        return;
      }
      response.removeHeader('Content-Length');
      response.setHeader('Content-Length', String(body.length));
      response.end(body);
      return;
    }
    await pipeline(Readable.fromWeb(upstream.body as unknown as NodeReadableStream<Uint8Array>), response);
  } catch {
    if (!response.headersSent) response.status(503).end();
    else response.destroy();
  }
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
    const image = await fetchApi(url, {
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

async function serverIdentityHeaders(): Promise<Record<string, string>> {
  return cloudRunIdentityHeaders(apiServiceAudience);
}

async function fetchApi(input: string | URL, init: RequestInit): Promise<Response> {
  const headers = new Headers(init.headers);
  for (const [name, value] of Object.entries(await serverIdentityHeaders())) headers.set(name, value);
  return fetch(input, { ...init, headers });
}

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
      const result = await fetchApi(url, {
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

async function readBoundedBody(body: ReadableStream<Uint8Array>, maxBytes: number): Promise<Buffer | null> {
  const reader = body.getReader();
  const chunks: Buffer[] = [];
  let bytes = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) return Buffer.concat(chunks, bytes);
      bytes += value.byteLength;
      if (bytes > maxBytes) {
        await reader.cancel();
        return null;
      }
      chunks.push(Buffer.from(value));
    }
  } finally {
    reader.releaseLock();
  }
}
