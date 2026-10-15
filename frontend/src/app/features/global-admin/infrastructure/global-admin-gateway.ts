import { Injectable, inject } from '@angular/core';
import { GlobalAdminPort } from '../domain/ports/global-admin-port';
import { GlobalAdminApi } from './global-admin-api';

@Injectable({ providedIn: 'root' })
export class GlobalAdminGateway extends GlobalAdminPort {
  private readonly api = inject(GlobalAdminApi);

  override overview(token: string) {
    return this.api.overview(token);
  }

  override list(token: string, filters: Record<string, string | number | undefined>) {
    return this.api.list(token, filters);
  }

  override account(token: string, id: string) {
    return this.api.account(token, id);
  }
}
