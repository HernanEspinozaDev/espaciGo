import type { BackendOperation } from '../../../../core/contracts/operations';

export class FinanceApiError extends Error {
  constructor(
    message: string,
    readonly status = 0,
  ) {
    super(message);
    this.name = 'FinanceApiError';
  }
}

export abstract class FinancePort {
  abstract download(token: string, path: BackendOperation): Promise<Blob>;
  abstract read<T>(
    token: string,
    operation: BackendOperation,
    query?: Record<string, string | number | undefined>,
  ): Promise<T>;
  abstract execute<T>(
    token: string,
    operation: BackendOperation,
    body?: unknown,
    attemptKey?: string,
  ): Promise<T>;
  abstract clearAttempts(): void;
}
