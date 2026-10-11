import { HttpClient } from '@angular/common/http';
import { Injectable, inject, signal } from '@angular/core';
import { firstValueFrom, timeout } from 'rxjs';
import { client } from '../../generated/api/client.gen';

interface PublicConfig {
  apiBaseUrl: string;
}

@Injectable({ providedIn: 'root' })
export class ApiRuntimeConfig {
  private readonly http = inject(HttpClient);
  readonly apiBaseUrl = signal<string | null>(null);

  async load(): Promise<void> {
    const config = await firstValueFrom(
      this.http.get<PublicConfig>('/config.json').pipe(timeout(4000)),
    );
    const apiUrl = new URL(config.apiBaseUrl);
    if (
      (apiUrl.protocol !== 'http:' && apiUrl.protocol !== 'https:') ||
      apiUrl.username ||
      apiUrl.password
    ) {
      throw new Error('Invalid public API URL');
    }

    const normalizedUrl = apiUrl.toString().replace(/\/$/, '');
    this.apiBaseUrl.set(normalizedUrl);
    client.setConfig({ baseUrl: normalizedUrl });
  }

  readinessUrl(): string {
    const apiUrl = this.apiBaseUrl();
    if (!apiUrl) {
      throw new Error('Public API URL has not loaded');
    }
    return new URL('/health/ready', apiUrl).toString();
  }
}
