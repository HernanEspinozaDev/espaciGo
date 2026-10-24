export function serializeExactJson(value: unknown): string {
  const ancestors = new WeakSet<object>();

  function assertJsonValue(candidate: unknown): void {
    if (
      candidate === null ||
      typeof candidate === "string" ||
      typeof candidate === "boolean"
    ) {
      return;
    }
    if (typeof candidate === "number") {
      if (!Number.isFinite(candidate))
        throw new TypeError("JSON number is not finite");
      return;
    }
    if (typeof candidate !== "object") {
      throw new TypeError("Value is not representable as exact JSON");
    }
    if (ancestors.has(candidate)) throw new TypeError("Cyclic JSON value");

    ancestors.add(candidate);
    if (Array.isArray(candidate)) {
      for (let index = 0; index < candidate.length; index += 1) {
        if (!Object.hasOwn(candidate, index)) {
          throw new TypeError("Sparse arrays are not valid request JSON");
        }
        assertJsonValue(candidate[index]);
      }
    } else {
      const prototype = Object.getPrototypeOf(candidate);
      if (prototype !== Object.prototype && prototype !== null) {
        throw new TypeError("Request JSON must use plain objects");
      }
      if (Object.getOwnPropertySymbols(candidate).length > 0) {
        throw new TypeError("Symbol keys are not valid request JSON");
      }
      for (const key of Object.keys(candidate)) {
        const descriptor = Object.getOwnPropertyDescriptor(candidate, key);
        if (!descriptor || !("value" in descriptor)) {
          throw new TypeError("Request JSON cannot contain accessors");
        }
        assertJsonValue(descriptor.value);
      }
    }
    ancestors.delete(candidate);
  }

  assertJsonValue(value);
  const serialized = JSON.stringify(value);
  if (typeof serialized !== "string") {
    throw new TypeError("Value is not representable as JSON text");
  }
  return serialized;
}
