let pendingScope: string | null = null;

export function markPartialDraftRecoveryPending(scope: string): void {
  pendingScope = scope;
}

export function hasPendingPartialDraftRecovery(scope: string | null): boolean {
  return Boolean(scope && pendingScope === scope);
}

export function clearPartialDraftRecoveryPending(scope?: string): void {
  if (scope === undefined || pendingScope === scope) {
    pendingScope = null;
  }
}
