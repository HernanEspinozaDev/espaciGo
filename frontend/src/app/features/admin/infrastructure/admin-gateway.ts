import { Injectable, inject } from '@angular/core';
import type { BackendOperation } from '../../../core/contracts/operations';
import { operationPath } from '../../../core/infrastructure/operation-path';
import { AdminPort } from '../domain/ports/admin-port';
import { AdminApi } from './admin-api';

/** Maps application resources/commands to HTTP while retaining the transport's idempotency policy. */
@Injectable({ providedIn: 'root' })
export class AdminGateway extends AdminPort {
  private readonly transport = inject(AdminApi);
  readonly permissionDeniedRevision = this.transport.permissionDeniedRevision;
  readonly capabilitiesRevision = this.transport.capabilitiesRevision;
  read<T>(
    token: string,
    operation: BackendOperation,
    query?: Record<string, string | number | undefined>,
  ): Promise<T> {
    return this.transport.get<T>(token, operationPath(operation), query);
  }
  execute<T>(
    token: string,
    accountId: string,
    operation: BackendOperation,
    body?: unknown,
    scope?: string,
  ): Promise<T> {
    return this.transport.post<T>(token, accountId, operationPath(operation), body, scope);
  }
  remove(token: string, operation: BackendOperation): Promise<void> {
    return this.transport.delete(token, operationPath(operation));
  }
  downloadEvidence(token: string, operation: BackendOperation): Promise<Blob> {
    return this.transport.downloadEvidence(token, operationPath(operation));
  }
  handleForbidden(token: string): void {
    this.transport.handleForbidden(token);
  }
}
