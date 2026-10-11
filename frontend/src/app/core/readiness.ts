import { HttpClient } from '@angular/common/http';
import { Injectable, inject, signal } from '@angular/core';
import { firstValueFrom, timeout } from 'rxjs';
import { ApiRuntimeConfig } from './api-runtime-config';

type ReadinessStatus = 'checking' | 'ready' | 'unavailable';

interface ReadinessResponse {
  status: string;
}

@Injectable({ providedIn: 'root' })
export class Readiness {
  private readonly http = inject(HttpClient);
  private readonly apiConfig = inject(ApiRuntimeConfig);
  readonly status = signal<ReadinessStatus>('checking');

  async refresh(): Promise<void> {
    this.status.set('checking');

    try {
      if (!this.apiConfig.apiBaseUrl()) {
        await this.apiConfig.load();
      }
      const response = await firstValueFrom(
        this.http.get<ReadinessResponse>(this.apiConfig.readinessUrl()).pipe(timeout(4000)),
      );
      this.status.set(response.status === 'ready' ? 'ready' : 'unavailable');
    } catch {
      this.status.set('unavailable');
    }
  }
}
