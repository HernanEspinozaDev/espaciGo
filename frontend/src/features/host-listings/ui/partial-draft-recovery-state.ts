let pending = false;

export function markPartialDraftRecoveryPending(): void {
  pending = true;
}

export function hasPendingPartialDraftRecovery(): boolean {
  return pending;
}

export function clearPartialDraftRecoveryPending(): void {
  pending = false;
}
