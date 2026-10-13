import { provideHttpClient } from '@angular/common/http';
import { PLATFORM_ID, RESPONSE_INIT } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { Meta } from '@angular/platform-browser';
import { TestBed } from '@angular/core/testing';
import { PublicPageSeo } from './public-page-seo';
import { FRONTEND_PUBLIC_ORIGIN } from './public-runtime-config';

describe('PublicPageSeo', () => {
  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        { provide: PLATFORM_ID, useValue: 'server' },
        { provide: FRONTEND_PUBLIC_ORIGIN, useValue: 'https://spaces.example.test' },
        { provide: RESPONSE_INIT, useValue: {} },
      ],
    });
  });

  it('uses the configured site origin, removes filter queries from canonical, and noindexes filters', () => {
    const seo = TestBed.inject(PublicPageSeo);
    const meta = TestBed.inject(Meta);
    const document = TestBed.inject(DOCUMENT);
    seo.catalog(true);

    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href'))
      .toBe('https://spaces.example.test/explorar');
    expect(meta.getTag('name="robots"')?.content).toBe('noindex, follow');
    expect(meta.getTag('property="og:url"')?.content).toBe('https://spaces.example.test/explorar');
  });

  it('keeps detail metadata public and context URLs out of its canonical', () => {
    const seo = TestBed.inject(PublicPageSeo);
    const meta = TestBed.inject(Meta);
    const document = TestBed.inject(DOCUMENT);
    seo.detail({
      id: '5e7d9f2a-631a-4c82-9d26-791225d19412',
      title: 'Sala para eventos',
      description: 'Descripción pública\n  controlada.',
      imageId: '1e7d9f2a-631a-4c82-9d26-791225d19412',
      contextual: true,
    });

    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href'))
      .toBe('https://spaces.example.test/espacios/5e7d9f2a-631a-4c82-9d26-791225d19412');
    expect(meta.getTag('name="robots"')?.content).toBe('noindex, follow');
    expect(meta.getTag('property="og:image"')?.content)
      .toBe('https://spaces.example.test/public/spaces/5e7d9f2a-631a-4c82-9d26-791225d19412/images/1e7d9f2a-631a-4c82-9d26-791225d19412');
    expect(meta.getTag('name="description"')?.content).toBe('Descripción pública controlada.');
  });

  it('returns a real 404 for removed public detail and a 503 for service failure', () => {
    const seo = TestBed.inject(PublicPageSeo);
    const response = TestBed.inject(RESPONSE_INIT);
    seo.unavailable();
    expect(response?.status).toBe(404);
    seo.unavailableService();
    expect(response?.status).toBe(503);
  });
});
