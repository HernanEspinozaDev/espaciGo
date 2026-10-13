import { Injectable, inject } from '@angular/core';
import type { BackendOperation } from '../../../core/contracts/operations';
import { operationPath, operationQuery } from '../../../core/infrastructure/operation-path';
import { FinancePort } from '../domain/ports/finance-port';
import { FinanceApi } from './finance-api';

@Injectable({ providedIn: 'root' })
export class FinanceGateway extends FinancePort {
  private readonly transport = inject(FinanceApi);
  read<T>(
    token: string,
    operation: BackendOperation,
    query?: Record<string, string | number | undefined>,
  ): Promise<T> {
    return this.transport.call<T>(token, operationPath(operation) + operationQuery(query), 'GET');
  }
  execute<T>(token: string, operation: BackendOperation, body?: unknown, key?: string): Promise<T> {
    return this.transport.call<T>(token, operationPath(operation), 'POST', body, key);
  }
  download(token: string, operation: BackendOperation): Promise<Blob> {
    return this.transport.download(token, operationPath(operation));
  }
  clearAttempts(): void {
    this.transport.clearAttempts();
  }
}
