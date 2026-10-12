import type { BackendOperation } from '../../../../core/contracts/operations';

export type Attempt = { key: string; body: unknown };
export class AdminApiError extends Error {
  constructor(
    message: string,
    readonly status = 0,
  ) {
    super(message);
    this.name = 'AdminApiError';
  }
}

export abstract class AdminPort {
  abstract readonly permissionDeniedRevision: () => number;
  abstract readonly capabilitiesRevision: () => number;
  abstract read<T>(
    token: string,
    path: BackendOperation,
    query?: Record<string, string | number | undefined>,
  ): Promise<T>;
  abstract execute<T>(
    token: string,
    accountId: string,
    path: BackendOperation,
    body?: unknown,
    idempotencyScope?: string,
  ): Promise<T>;
  abstract remove(token: string, path: BackendOperation): Promise<void>;
  abstract downloadEvidence(token: string, path: BackendOperation): Promise<Blob>;
  abstract handleForbidden(token: string): void;
}
