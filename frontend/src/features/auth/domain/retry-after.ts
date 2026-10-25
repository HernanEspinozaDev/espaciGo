export function retryAfterSeconds(value: string | null): number | null {
  if (!value || !/^\d{1,7}$/.test(value.trim())) return null;
  const seconds = Number(value.trim());
  return Number.isSafeInteger(seconds) && seconds > 0 ? seconds : null;
}

export function remainingRetrySeconds(
  deadline: number,
  now = Date.now(),
): number {
  return Math.max(0, Math.ceil((deadline - now) / 1000));
}
