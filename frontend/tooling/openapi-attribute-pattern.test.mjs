import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { load } from 'js-yaml';

test('parsed OpenAPI attribute pattern accepts public operators and precise decimals', () => {
  const document = load(readFileSync(new URL('../../planning/openapi.yaml', import.meta.url), 'utf8'));
  const parameter = document.paths['/public/spaces'].get.parameters.find(({ name }) => name === 'attribute');
  const pattern = parameter?.schema?.items?.pattern;
  assert.ok(pattern, 'attribute array item schema declares a pattern');
  const expression = new RegExp(pattern);

  for (const value of [
    'wifi:true', 'wifi:false', 'altura_util_m:gte:3.50', 'altura_util_m:lte:5',
    'puestos_trabajo:gte:3', 'puestos_trabajo:lte:8', 'tipo_uso_oficina:eq:privada',
    'climatizacion:in:aire_acondicionado,calefaccion',
  ]) assert.match(value, expression);

  for (const value of [
    'altura_util_m:gte:3.x', 'puestos_trabajo:lte:-1', 'tipo_uso_oficina:eq:Privada',
    'climatizacion:in:aire_acondicionado,', 'wifi:maybe', 'wifi:gt:1',
  ]) assert.doesNotMatch(value, expression);
});
