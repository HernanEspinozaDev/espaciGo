export type PublicAttributePredicate =
  | { kind: 'boolean'; value: boolean }
  | { kind: 'range'; minimum?: string; maximum?: string }
  | { kind: 'enum'; value: string }
  | { kind: 'enum_list'; values: string[] };

const CODE = /^[a-z][a-z0-9_]{0,59}$/;
const OPTION = /^[a-z][a-z0-9_]{0,79}$/;
const NUMBER = /^(0|[1-9]\d*)(\.\d+)?$/;

/** Decode and canonicalize repeated public attribute query parameters. */
export function decodePublicAttributeFilters(
  entries: readonly string[],
): Record<string, PublicAttributePredicate> | null {
  if (entries.length > 32) return null;
  const filters: Record<string, PublicAttributePredicate> = {};

  for (const entry of entries) {
    const split = entry.indexOf(':');
    if (split < 1) return null;
    const code = entry.slice(0, split);
    if (!CODE.test(code)) return null;
    const expression = entry.slice(split + 1);
    const parts = expression.split(':');
    const prior = filters[code];

    if (parts.length === 1 && (parts[0] === 'true' || parts[0] === 'false')) {
      if (prior) return null;
      filters[code] = { kind: 'boolean', value: parts[0] === 'true' };
      continue;
    }

    if (parts.length === 2 && (parts[0] === 'gte' || parts[0] === 'lte') && NUMBER.test(parts[1])) {
      if (prior && prior.kind !== 'range') return null;
      const range = prior?.kind === 'range' ? { ...prior } : { kind: 'range' as const };
      const key = parts[0] === 'gte' ? 'minimum' : 'maximum';
      if (range[key] !== undefined) return null;
      range[key] = parts[1];
      filters[code] = range;
      continue;
    }

    if (parts.length === 2 && parts[0] === 'eq' && OPTION.test(parts[1])) {
      if (prior) return null;
      filters[code] = { kind: 'enum', value: parts[1] };
      continue;
    }

    if (parts.length === 2 && parts[0] === 'in') {
      const options = parts[1].split(',');
      if (options.some((option) => !OPTION.test(option))) return null;
      if (new Set(options).size !== options.length) return null;
      if (prior && prior.kind !== 'enum_list') return null;
      const merged = prior?.kind === 'enum_list' ? [...prior.values] : [];
      if (merged.length + options.length > 32 || options.some((option) => merged.includes(option))) return null;
      filters[code] = { kind: 'enum_list', values: [...merged, ...options] };
      continue;
    }

    return null;
  }

  if (Object.keys(filters).length > 16) return null;
  return filters;
}

/** Encode predicates into the canonical repeated-query representation. */
export function encodePublicAttributeFilters(
  filters: Record<string, PublicAttributePredicate>,
): string[] {
  return Object.entries(filters)
    .sort(([left], [right]) => left.localeCompare(right))
    .flatMap(([code, predicate]) => {
      if (predicate.kind === 'boolean') return [`${code}:${predicate.value}`];
      if (predicate.kind === 'range') {
        return [
          ...(predicate.minimum ? [`${code}:gte:${predicate.minimum}`] : []),
          ...(predicate.maximum ? [`${code}:lte:${predicate.maximum}`] : []),
        ];
      }
      if (predicate.kind === 'enum') return [`${code}:eq:${predicate.value}`];
      return [`${code}:in:${[...predicate.values].sort().join(',')}`];
    });
}
