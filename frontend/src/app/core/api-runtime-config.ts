import { HttpClient } from '@angular/common/http';
import { isPlatformBrowser } from '@angular/common';
import { Injectable, inject, PLATFORM_ID, signal } from '@angular/core';
import { firstValueFrom, timeout } from 'rxjs';
import { client } from '../../generated/api/client.gen';
import { API_PUBLIC_BASE_URL, API_SERVER_BASE_URL, FRONTEND_PUBLIC_ORIGIN, configuredPublicApiBase, configuredPublicOrigin } from './public-runtime-config';

interface PublicConfig {
  apiBaseUrl: string;
  siteOrigin: string;
}

@Injectable({ providedIn: 'root' })
export class ApiRuntimeConfig {
  private readonly platformId = inject(PLATFORM_ID);
  private readonly http = inject(HttpClient);
  private readonly serverApiBase = inject(API_SERVER_BASE_URL, { optional: true });
  private readonly serverPublicApiBase = inject(API_PUBLIC_BASE_URL, { optional: true });
  private readonly serverSiteOrigin = inject(FRONTEND_PUBLIC_ORIGIN, { optional: true });
  readonly apiBaseUrl = signal<string | null>(this.serverApiBase ?? null);
  readonly publicApiBaseUrl = signal<string | null>(this.serverPublicApiBase ?? null);
  readonly publicSiteOrigin = signal<string | null>(this.serverSiteOrigin ?? null);

  async load(): Promise<void> {
    const config = await firstValueFrom(
      this.http.get<PublicConfig>('/config.json').pipe(timeout(4000)),
    );
    const normalizedUrl = configuredPublicApiBase(config.apiBaseUrl);
    const siteOrigin = configuredPublicOrigin(config.siteOrigin);
    if (!normalizedUrl || !siteOrigin) throw new Error('Invalid public runtime configuration');
    const apiUrl = isPlatformBrowser(this.platformId) ? normalizedUrl : (this.serverApiBase ?? normalizedUrl);
    if (!isPlatformBrowser(this.platformId) && apiUrl.startsWith('/')) {
      throw new Error('SSR requires an absolute internal API URL');
    }
    this.apiBaseUrl.set(apiUrl);
    this.publicApiBaseUrl.set(normalizedUrl);
    this.publicSiteOrigin.set(siteOrigin);
    client.setConfig({ baseUrl: apiUrl });
  }

  readinessUrl(): string {
    const apiUrl = this.apiBaseUrl();
    if (!apiUrl) {
      throw new Error('Public API URL has not loaded');
    }
    if (apiUrl.startsWith('/')) return '/health/ready';
    return new URL('/health/ready', apiUrl).toString();
  }
}
