import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { TermsResponse } from '../../core/contracts/models';
import { IdentityPort } from '../../core/identity/domain/ports/identity-port';
import { ConsentDocumentPage } from './consent-document-page';

const response: TermsResponse = {
  items: [
    {
      id: 'terms-current',
      code: 'terms-current',
      type: 'terminos',
      sha256: 'synthetic-sha',
      published_at: '2026-10-01T00:00:00Z',
      synthetic: true,
      document: {
        id: 'terms-current',
        code: 'terms-current',
        type: 'terminos',
        title: 'Términos de ensayo',
        scope: 'Cuenta local',
        status: 'vigente',
        sections: [
          { heading: 'Uso del servicio', body: 'Texto sintético de la primera sección.' },
          { heading: 'Datos de cuenta', body: 'Texto sintético de la segunda sección.' },
        ],
      },
    },
  ],
};

describe('ConsentDocumentPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('explains a missing document and lets the reader retry before accepting it', async () => {
    const api = { terms: vi.fn().mockRejectedValueOnce(new Error('synthetic unavailable')).mockResolvedValue(response) };
    TestBed.configureTestingModule({
      imports: [ConsentDocumentPage],
      providers: [
        provideRouter([]),
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: {
              data: { documentType: 'terminos' },
              queryParamMap: convertToParamMap({}),
            },
          },
        },
        { provide: IdentityPort, useValue: api },
      ],
    });
    const fixture = TestBed.createComponent(ConsentDocumentPage);
    fixture.detectChanges();
    await vi.waitFor(() => expect(api.terms).toHaveBeenCalledOnce());
    await vi.waitFor(() => expect(fixture.nativeElement.querySelector('[role="alert"] h1')?.textContent).toContain('Documento no disponible'));

    const root = fixture.nativeElement as HTMLElement;
    const retry = root.querySelector<HTMLButtonElement>('button');
    expect(retry?.textContent).toContain('Volver a intentar');
    retry?.click();
    fixture.detectChanges();
    await vi.waitFor(() => expect(api.terms).toHaveBeenCalledTimes(2));
    await vi.waitFor(() => expect(root.querySelector('#legal-title')?.textContent).toContain('Términos de ensayo'));
    expect(root.querySelector('[role="alert"]')).toBeNull();
    fixture.destroy();
  });

  it('provides an in-document index linked to the exact published sections', async () => {
    const api = { terms: vi.fn().mockResolvedValue(response) };
    TestBed.configureTestingModule({
      imports: [ConsentDocumentPage],
      providers: [
        provideRouter([]),
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: {
              data: { documentType: 'terminos' },
              queryParamMap: convertToParamMap({}),
            },
          },
        },
        { provide: IdentityPort, useValue: api },
      ],
    });
    const fixture = TestBed.createComponent(ConsentDocumentPage);
    fixture.detectChanges();
    await vi.waitFor(() => expect(api.terms).toHaveBeenCalledOnce());
    await vi.waitFor(() => expect(fixture.nativeElement.querySelector('#legal-title')).not.toBeNull());
    fixture.detectChanges();

    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('h1')?.textContent).toContain('Términos de ensayo');
    expect([...root.querySelectorAll('.legal-index a')].map((link) => [
      link.textContent?.trim(), link.getAttribute('href'),
    ])).toEqual([
      ['Uso del servicio', '#legal-section-0'],
      ['Datos de cuenta', '#legal-section-1'],
    ]);
    expect(root.querySelector('#legal-section-0 h2')?.textContent).toContain('Uso del servicio');
    expect(root.querySelector('#legal-section-1 h2')?.textContent).toContain('Datos de cuenta');
    expect(root.querySelector('time')?.getAttribute('datetime')).toBe('2026-10-01T00:00:00Z');
    fixture.destroy();
  });
});
