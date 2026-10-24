const TYPE_SEPARATOR = "!";
const TYPE_VALUES_SEPARATOR = "*";
const VALUE_SEPARATOR = ".";

export function parseVts(value: string | null): Record<string, string[]> {
  if (!value) return {};
  return Object.fromEntries(
    value
      .split(TYPE_SEPARATOR)
      .map(
        (segment) =>
          segment.split(TYPE_VALUES_SEPARATOR, 2) as [string, string?],
      )
      .map(([type, values]) =>
        type && values
          ? [type, values.split(VALUE_SEPARATOR).filter(Boolean)]
          : null,
      )
      .filter(
        (pair): pair is [string, string[]] =>
          pair !== null && pair[1].length > 0,
      ),
  );
}

export function encodeVts(record: Record<string, string[]>): string {
  return Object.entries(record)
    .filter(([, values]) => values.length > 0)
    .map(
      ([type, values]) =>
        `${type}${TYPE_VALUES_SEPARATOR}${values.join(VALUE_SEPARATOR)}`,
    )
    .join(TYPE_SEPARATOR);
}
