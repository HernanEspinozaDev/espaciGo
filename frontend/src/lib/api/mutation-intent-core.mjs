/**
 * @template {object} T
 * @param {T} value
 * @returns {Readonly<T>}
 */
function deepFreeze(value) {
  if (value !== null && typeof value === "object" && !Object.isFrozen(value)) {
    for (const nested of Object.values(value)) deepFreeze(nested);
    Object.freeze(value);
  }
  return value;
}

/**
 * @template {object} T
 * @param {T} body
 * @param {(value: unknown) => string} serialize
 * @returns {{ idempotencyKey: string; body: Readonly<T>; serializedBody: string }}
 */
export function createMutationIntentCore(body, serialize) {
  const serializedBody = serialize(body);
  return Object.freeze({
    idempotencyKey: crypto.randomUUID(),
    body: deepFreeze(JSON.parse(serializedBody)),
    serializedBody,
  });
}
