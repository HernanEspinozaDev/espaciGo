import { decodePublicAttributeFilters, encodePublicAttributeFilters } from '../public-attribute-filter-codec';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const CATALOG_KEYS = new Set([
  'q',
  'category_code',
  'rate_unit',
  'min_unit_price_clp',
  'max_unit_price_clp',
  'sort',
  'page_size',
  'view',
  'desktop_columns',
  'mobile_columns',
  'region_code',
  'comuna_code',
  'min_area_m2',
  'start_at',
  'end_at',
  'min_total_clp',
  'max_total_clp',
  'attribute',
]);
const CATEGORIES = new Set([
  'oficina',
  'sala_multiproposito',
  'bodega',
  'estacionamiento',
  'local_flexible',
  'stand',
  'quincho',
  'parcela_eventos',
]);
const MAX_CLP = 9223372036854775807n;

export type ContextualReturn = {
  identityReturnUrl: string;
  detailUrl: string;
  listingUrl: string;
};

/** Accept only the legacy contract invitation or an explicitly supported space action. */
export function parseIdentityReturn(values: readonly string[]): ContextualReturn | string | null {
  if (values.length !== 1) return null;
  const value = values[0];
  const invitation = safeInvitation(value);
  if (invitation) return invitation;
  if (!value || value.length > 3000 || !value.startsWith('/') || value.startsWith('//')) return null;

  try {
    const url = new URL(value, 'https://espacigo.invalid');
    if (
      url.origin !== 'https://espacigo.invalid' ||
      url.username ||
      url.password ||
      url.hash ||
      url.pathname !== `/espacios/${url.pathname.split('/')[2]}`
    ) {
      return null;
    }
    const spaceId = url.pathname.slice('/espacios/'.length);
    if (!UUID.test(spaceId) || url.searchParams.size !== 2) return null;
    const action = url.searchParams.getAll('action');
    const listings = url.searchParams.getAll('returnTo');
    if (action.length !== 1 || action[0] !== 'reserve' || listings.length !== 1) return null;
    const listingUrl = canonicalCatalogReturn(listings[0]);
    if (!listingUrl) return null;
    return {
      identityReturnUrl: `/espacios/${spaceId}?action=reserve&returnTo=${encodeURIComponent(listingUrl)}`,
      // The action intent is consumed by this redirect. The detail keeps only a
      // display marker and a safe catalog return; it never resumes a write.
      detailUrl: `/espacios/${spaceId}?context=returned&returnTo=${encodeURIComponent(listingUrl)}`,
      listingUrl,
    };
  } catch {
    return null;
  }
}

export function buildContextualReturn(spaceId: string, listing: string | null): string | null {
  if (!UUID.test(spaceId)) return null;
  const listingUrl = canonicalCatalogReturn(listing ?? '/explorar');
  return listingUrl
    ? `/espacios/${spaceId}?action=reserve&returnTo=${encodeURIComponent(listingUrl)}`
    : null;
}

export function canonicalCatalogReturn(value: string): string | null {
  if (!value || value.length > 2000 || !value.startsWith('/') || value.startsWith('//')) return null;
  try {
    const url = new URL(value, 'https://espacigo.invalid');
    if (
      url.origin !== 'https://espacigo.invalid' ||
      url.username ||
      url.password ||
      url.pathname !== '/explorar' ||
      url.hash
    ) {
      return null;
    }
    const values = new Map<string, string[]>();
    for (const [key, entry] of url.searchParams) {
      if (!CATALOG_KEYS.has(key)) return null;
      const entries = values.get(key) ?? [];
      entries.push(entry);
      values.set(key, entries);
    }
    for (const [key, entries] of values) {
      if (key !== 'attribute' && entries.length !== 1) return null;
    }

    const get = (key: string) => values.get(key)?.[0] ?? '';
    const query = new URLSearchParams();
    const q = get('q').normalize('NFC').trim().replace(/\s+/gu, ' ');
    if (q.length > 120 || /\p{Cc}/u.test(q)) return null;
    if (q) query.set('q', q);

    const category = get('category_code');
    if (category && !CATEGORIES.has(category)) return null;
    if (category) query.set('category_code', category);

    const unit = get('rate_unit');
    if (unit && !['hora', 'dia', 'mes'].includes(unit)) return null;
    if (unit) query.set('rate_unit', unit);

    for (const key of ['min_unit_price_clp', 'max_unit_price_clp']) {
      const value = get(key);
      if (value && !validClp(value)) return null;
      if (value) query.set(key, canonicalInteger(value));
    }
    const minUnit = get('min_unit_price_clp');
    const maxUnit = get('max_unit_price_clp');
    if (minUnit && maxUnit && BigInt(minUnit) > BigInt(maxUnit)) return null;

    const sort = get('sort');
    if (sort && !['default', 'title_asc', 'title_desc', 'unit_price_asc', 'unit_price_desc', 'estimated_total_asc', 'estimated_total_desc', 'relevance'].includes(sort)) return null;
    if (sort === 'unit_price_asc' || sort === 'unit_price_desc') {
      if (!unit) return null;
      query.set('sort', sort);
    } else if (sort === 'title_asc' || sort === 'title_desc') {
      query.set('sort', sort);
    } else if (sort.startsWith('estimated_total_')) {
      query.set('sort', sort);
    } else if (sort === 'relevance') {
      if (!q) return null;
      query.set('sort', sort);
    }

    const pageSize = get('page_size');
    if (pageSize) {
      if (!/^[1-9]\d*$/.test(pageSize) || Number(pageSize) > 48) return null;
      if (Number(pageSize) !== 24) query.set('page_size', String(Number(pageSize)));
    }

    const view = get('view');
    if (view && !['list', 'grid'].includes(view)) return null;
    // Legacy presentation preferences are validated above, then dropped. Layout is fixed by viewport.
    const desktopColumns = get('desktop_columns');
    if (desktopColumns && !['2', '3', '4'].includes(desktopColumns)) return null;

    const mobileColumns = get('mobile_columns');
    if (mobileColumns && !['1', '2'].includes(mobileColumns)) return null;


    for (const [key, pattern] of [
      ['region_code', /^\d{1,2}$/],
      ['comuna_code', /^\d{1,5}$/],
      ['min_area_m2', /^(0|[1-9]\d*)(\.\d{1,2})?$/],
    ] as const) {
      const value = get(key);
      if (value && !pattern.test(value)) return null;
      if (value) query.set(key, value);
    }

    const start = get('start_at');
    const end = get('end_at');
    if (!!start !== !!end) return null;
    if (start && (!validDate(start) || !validDate(end) || Date.parse(end) <= Date.parse(start))) return null;
    if (start) {
      query.set('start_at', start);
      query.set('end_at', end);
    }
    for (const key of ['min_total_clp', 'max_total_clp']) {
      const value = get(key);
      if (value && (!start || !validClp(value))) return null;
      if (value) query.set(key, canonicalInteger(value));
    }
    const minTotal = get('min_total_clp');
    const maxTotal = get('max_total_clp');
    if (minTotal && maxTotal && BigInt(minTotal) > BigInt(maxTotal)) return null;
    if (sort.startsWith('estimated_total_') && !start) return null;

    const attributes = values.get('attribute') ?? [];
    if (attributes.length && !category) return null;
    const decodedAttributes = decodePublicAttributeFilters(attributes);
    if (decodedAttributes === null) return null;
    for (const attribute of encodePublicAttributeFilters(decodedAttributes)) query.append('attribute', attribute);

    const encoded = query.toString();
    return encoded ? `/explorar?${encoded}` : '/explorar';
  } catch {
    return null;
  }
}

function safeInvitation(value: string): string | null {
  if (!value.startsWith('/reservas?')) return null;
  try {
    const url = new URL(value, 'https://espacigo.invalid');
    const ids = url.searchParams.getAll('contrato');
    return url.origin === 'https://espacigo.invalid' &&
      url.pathname === '/reservas' &&
      !url.hash &&
      url.searchParams.size === 1 &&
      ids.length === 1 &&
      UUID.test(ids[0])
      ? `/reservas?contrato=${encodeURIComponent(ids[0])}`
      : null;
  } catch {
    return null;
  }
}

function validClp(value: string): boolean {
  if (!/^(0|[1-9]\d*)$/.test(value)) return false;
  return BigInt(value) <= MAX_CLP;
}

function canonicalInteger(value: string): string {
  return BigInt(value).toString();
}

function validDate(value: string): boolean {
  return /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(value) &&
    Number.isFinite(Date.parse(value));
}
