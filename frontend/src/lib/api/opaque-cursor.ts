function stableJson(value: unknown, ancestors = new WeakSet<object>()): string {
  if (value === null || typeof value !== "object") {
    if (typeof value === "number" && !Number.isFinite(value)) {
      throw new TypeError("Filters must contain finite numbers");
    }
    const serialized = JSON.stringify(value);
    if (typeof serialized !== "string") {
      throw new TypeError("Filters must be JSON values");
    }
    return serialized;
  }
  if (ancestors.has(value)) throw new TypeError("Filters cannot be cyclic");
  ancestors.add(value);
  if (Array.isArray(value)) {
    const items: string[] = [];
    for (let index = 0; index < value.length; index += 1) {
      if (!Object.hasOwn(value, index)) {
        throw new TypeError("Filter arrays cannot be sparse");
      }
      items.push(stableJson(value[index], ancestors));
    }
    ancestors.delete(value);
    return `[${items.join(",")}]`;
  }
  const prototype = Object.getPrototypeOf(value);
  if (prototype !== Object.prototype && prototype !== null) {
    ancestors.delete(value);
    throw new TypeError("Filters must use plain objects");
  }
  if (Object.getOwnPropertySymbols(value).length > 0) {
    ancestors.delete(value);
    throw new TypeError("Filters cannot use symbol keys");
  }
  const record = value as Record<string, unknown>;
  const keys = Object.keys(record).sort();
  const fields = keys.map((key) => {
    const descriptor = Object.getOwnPropertyDescriptor(record, key);
    if (!descriptor || !("value" in descriptor)) {
      throw new TypeError("Filters cannot use accessors");
    }
    return `${JSON.stringify(key)}:${stableJson(descriptor.value, ancestors)}`;
  });
  ancestors.delete(value);
  return `{${fields.join(",")}}`;
}

export class OpaqueCursorPager {
  #filters: string | null = null;
  #cursor: string | null = null;

  current(filters: unknown): string | null {
    this.#resetIfChanged(filters);
    return this.#cursor;
  }

  accept(filters: unknown, nextCursor: string | null): void {
    this.#resetIfChanged(filters);
    this.#cursor = nextCursor;
  }

  reset(): void {
    this.#filters = null;
    this.#cursor = null;
  }

  #resetIfChanged(filters: unknown): void {
    const signature = stableJson(filters);
    if (signature !== this.#filters) {
      this.#filters = signature;
      this.#cursor = null;
    }
  }
}
