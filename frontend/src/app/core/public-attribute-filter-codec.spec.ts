import { describe, expect, it } from 'vitest';
import { decodePublicAttributeFilters, encodePublicAttributeFilters } from './public-attribute-filter-codec';

describe('public attribute URL codec', () => {
  it('normalizes CSV and repeated enum-list forms to one canonical OR expression', () => {
    const decoded = decodePublicAttributeFilters([
      'climatizacion:in:calefaccion,aire_acondicionado',
      'climatizacion:in:refrigeracion',
      'altura_util_m:gte:3.50',
      'altura_util_m:lte:5',
    ]);
    expect(decoded).toEqual({
      climatizacion: { kind: 'enum_list', values: ['calefaccion', 'aire_acondicionado', 'refrigeracion'] },
      altura_util_m: { kind: 'range', minimum: '3.50', maximum: '5' },
    });
    expect(encodePublicAttributeFilters(decoded!)).toEqual([
      'altura_util_m:gte:3.50',
      'altura_util_m:lte:5',
      'climatizacion:in:aire_acondicionado,calefaccion,refrigeracion',
    ]);
  });

  it.each([
    ['climatizacion:in:aire_acondicionado,aire_acondicionado'],
    ['climatizacion:in:aire_acondicionado', 'climatizacion:in:aire_acondicionado'],
    ['climatizacion:in:aire_acondicionado', 'climatizacion:eq:aire_acondicionado'],
    ['altura_util_m:gte:3', 'altura_util_m:gte:4'],
    ['altura_util_m:gte:3.5x'],
  ])('rejects invalid or ambiguous attributes %j', (...entries) => {
    expect(decodePublicAttributeFilters(entries)).toBeNull();
  });
});
