import { HttpBackend, HttpClient, HttpEvent, HttpRequest, HttpResponse, provideHttpClient, withInterceptors } from '@angular/common/http';
import { Component, signal } from '@angular/core';
import { BootstrapContext, bootstrapApplication } from '@angular/platform-browser';
import { renderApplication } from '@angular/platform-server';
import { ApplicationConfig, inject } from '@angular/core';
import { Observable, of, throwError } from 'rxjs';
import { afterEach, describe, expect, it } from 'vitest';
import { API_SERVER_BASE_URL } from './public-runtime-config';
import { SERVER_API_IDENTITY_HEADERS, serverApiIdentityInterceptor } from './server-api-identity';

const serviceToken = 'synthetic-iam-token-that-must-never-appear-in-rendered-html-'.padEnd(110, 'x');

@Component({ standalone: true, selector: 'app-root', template: '<h1>{{ title() }}</h1>' })
class SsrPublicReadProbe {
  readonly title = signal('loading');

  constructor() {
    inject(HttpClient).get<{ title: string }>('https://api.example.test/api/v1/public/spaces').subscribe({
      next: (value) => this.title.set(value.title),
      error: () => this.title.set('SSR upstream rejected request'),
    });
  }
}

class IamRequiredFakeUpstream implements HttpBackend {
  seen: HttpRequest<unknown> | null = null;

  handle(request: HttpRequest<unknown>): Observable<HttpEvent<unknown>> {
    this.seen = request;
    if (request.headers.get('X-Serverless-Authorization') !== `Bearer ${serviceToken}`) {
      return throwError(() => new Error('upstream requires Cloud Run IAM identity'));
    }
    return of(new HttpResponse({ status: 200, body: { title: 'SSR public catalog' } }));
  }
}

describe('Angular SSR IAM protected public read', () => {
  afterEach(() => document.body.replaceChildren());

  it('renders a fake IAM-protected upstream response without serializing the service token', async () => {
    const upstream = new IamRequiredFakeUpstream();
    const config: ApplicationConfig = {
      providers: [
        provideHttpClient(withInterceptors([serverApiIdentityInterceptor])),
        { provide: HttpBackend, useValue: upstream },
        { provide: API_SERVER_BASE_URL, useValue: 'https://api.example.test/api/v1' },
        { provide: SERVER_API_IDENTITY_HEADERS, useValue: async () => ({ 'X-Serverless-Authorization': `Bearer ${serviceToken}` }) },
      ],
    };
    const html = await renderApplication((context: BootstrapContext) => bootstrapApplication(SsrPublicReadProbe, config, context), {
      document: '<app-root></app-root>',
      url: 'http://localhost/explorar',
      allowedHosts: ['localhost'],
    });
    expect(upstream.seen?.headers.get('X-Serverless-Authorization')).toBe(`Bearer ${serviceToken}`);
    expect(upstream.seen?.transferCache).toBe(false);
    expect(html).toContain('SSR public catalog');
    expect(html).not.toContain(serviceToken);
  });
});
