export interface PasswordPolicyResult {
  readonly valid: boolean;
  readonly codePoints: number;
  readonly utf8Bytes: number;
  readonly hasUppercase: boolean;
  readonly hasDigit: boolean;
  readonly hasPunctuation: boolean;
}

export function inspectPassword(value: string): PasswordPolicyResult {
  const codePoints = Array.from(value).length;
  const utf8Bytes = new TextEncoder().encode(value).byteLength;
  const hasUppercase = /\p{Lu}/u.test(value);
  const hasDigit = /\p{Nd}/u.test(value);
  const hasPunctuation = /[\p{P}\p{S}]/u.test(value);
  return Object.freeze({
    valid:
      codePoints >= 8 &&
      utf8Bytes <= 72 &&
      hasUppercase &&
      hasDigit &&
      hasPunctuation,
    codePoints,
    utf8Bytes,
    hasUppercase,
    hasDigit,
    hasPunctuation,
  });
}
