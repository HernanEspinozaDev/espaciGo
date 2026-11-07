export function normalizeRut(value: string): string {
  const compact = value.replace(/[^0-9kK]/g, "").toUpperCase();
  if (compact.length < 2) return compact;
  return `${compact.slice(0, -1)}-${compact.slice(-1)}`;
}

export function formatRut(value: string): string {
  const normalized = normalizeRut(value);
  const [digits, verifier] = normalized.split("-");
  if (!digits || !verifier) return normalized;
  return `${digits.replace(/\B(?=(\d{3})+(?!\d))/g, ".")}-${verifier}`;
}

export function isValidRut(value: string): boolean {
  const normalized = normalizeRut(value);
  const match = /^(\d{1,8})-([0-9K])$/.exec(normalized);
  if (!match) return false;
  const digits = match[1];
  let sum = 0;
  let multiplier = 2;
  for (let index = digits.length - 1; index >= 0; index -= 1) {
    sum += Number(digits[index]) * multiplier;
    multiplier = multiplier === 7 ? 2 : multiplier + 1;
  }
  const remainder = 11 - (sum % 11);
  const expected = remainder === 11 ? "0" : remainder === 10 ? "K" : String(remainder);
  return match[2] === expected;
}
