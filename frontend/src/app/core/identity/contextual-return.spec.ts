import { describe, expect, it } from 'vitest';
import {
  buildContextualReturn,
  canonicalCatalogReturn,
  parseIdentityReturn,
} from './contextual-return';

const spaceId = '550e8400-e29b-41d4-a716-446655440000';

describe('contextual identity return', () => {
  it('canonicalizes public filters and consumes the action into a read-only detail return', () => {
    const canonical = canonicalCatalogReturn(
      '/explorar?sort=relevance&q=%20Oficina%20%20centro%20&category_code=oficina&cursor=stale',
    );
    expect(canonical).toBeNull();
    const validListing = canonicalCatalogReturn(
      '/explorar?sort=relevance&q=%20Oficina%20%20centro%20&category_code=oficina&page_size=20',
    );
    expect(validListing).toBe('/explorar?q=Oficina+centro&category_code=oficina&sort=relevance');
    const context = buildContextualReturn(spaceId, validListing);
    expect(context).toBe(
      `/espacios/${spaceId}?action=reserve&returnTo=${encodeURIComponent(validListing!)}`,
    );
    const parsed = parseIdentityReturn([context!]);
    expect(parsed).toEqual({
      identityReturnUrl: context,
      detailUrl: `/espacios/${spaceId}?context=returned&returnTo=${encodeURIComponent(validListing!)}`,
      listingUrl: validListing,
    });
    expect(parseIdentityReturn([parsed && typeof parsed !== 'string' ? parsed.detailUrl : ''])).toBeNull();
  });

  it('keeps the legacy contract invitation return', () => {
    const contractId = '123e4567-e89b-42d3-a456-426614174000';
    expect(parseIdentityReturn([`/reservas?contrato=${contractId}`])).toBe(
      `/reservas?contrato=${contractId}`,
    );
  });

  it.each([
    'https://evil.example',
    '//evil.example/espacios/' + spaceId,
    '/\\\\evil.example/espacios/' + spaceId,
    `/espacios/${spaceId}?action=reserve&returnTo=https://evil.example`,
    `/espacios/${spaceId}?action=reserve&returnTo=%2F%2Fevil.example`,
    `/espacios/${spaceId}?action=reserve&action=reserve&returnTo=%2Fexplorar`,
    `/espacios/${spaceId}?action=other&returnTo=%2Fexplorar`,
    `/espacios/not-a-uuid?action=reserve&returnTo=%2Fexplorar`,
    `/espacios/${spaceId}?action=reserve&returnTo=%2Fexplorar%3Fq%3Done%26q%3Dtwo`,
    `/espacios/${spaceId}?action=reserve&returnTo=%2Fexplorar%3Fsort%3Dunit_price_asc`,
    `/espacios/${spaceId}?action=reserve&returnTo=%2Fexplorar%3Fcursor%3Dstale`,
    `/espacios/${spaceId}?action=reserve&returnTo=%2Fexplorar%23fragment`,
  ])('rejects unsafe or ambiguous return %s', (returnUrl) => {
    expect(parseIdentityReturn([returnUrl])).toBeNull();
  });

  it('rejects repeated return parameters, credentials, and unsupported catalog query keys', () => {
    expect(parseIdentityReturn(['/explorar', `/espacios/${spaceId}?action=reserve&returnTo=%2Fexplorar`])).toBeNull();
    expect(canonicalCatalogReturn('/explorar?utm_source=outside')).toBeNull();
    expect(canonicalCatalogReturn('/explorar?attribute=a:true&attribute=a:false&category_code=oficina')).toBeNull();
  });
});
