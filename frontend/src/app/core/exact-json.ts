const exactClpKeys = new Set([
  'base_price_clp',
  'base_price',
  'base_subtotal',
  'estimated_total_clp',
  'min_total_clp',
  'max_total_clp',
  'unit_price_clp',
  'subtotal_clp',
  'guarantee_expected_clp',
]);

/** Quote CLP literals before JSON.parse so int64 amounts never pass through Number. */
export function parseExactClpJson<T>(source: string): T {
  let output = '';
  let index = 0;
  while (index < source.length) {
    if (source[index] !== '"') {
      output += source[index++];
      continue;
    }
    const stringStart = index++;
    while (index < source.length) {
      if (source[index] === '\\') index += 2;
      else if (source[index++] === '"') break;
    }
    const rawString = source.slice(stringStart, index);
    output += rawString;
    let cursor = index;
    while (/\s/.test(source[cursor] ?? '') && cursor < source.length) cursor++;
    if (source[cursor] !== ':') continue;
    const key = rawString.slice(1, -1);
    if (!exactClpKeys.has(key)) continue;
    cursor++;
    while (/\s/.test(source[cursor] ?? '') && cursor < source.length) cursor++;
    const number = /^-?\d+/.exec(source.slice(cursor))?.[0];
    if (!number || /[.eE]/.test(source[cursor + number.length] ?? '')) {
      throw new Error(`Importe CLP entero inválido: ${key}`);
    }
    output += source.slice(index, cursor);
    index = cursor;
    output += `"${number}"`;
    index += number.length;
  }
  return JSON.parse(output) as T;
}

/** Serialize numeric-looking CLP strings as JSON integer tokens without Number conversion. */
export function stringifyExactClpJson(value: unknown): string {
  return JSON.stringify(value).replace(
    /("(?:base_price_clp|base_price|base_subtotal)"\s*:\s*)"(\d+)"/g,
    '$1$2',
  );
}
