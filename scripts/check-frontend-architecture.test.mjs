import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { contractRoots, inspectSource } from './check-frontend-architecture.mjs';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const roots = contractRoots(fs.readFileSync(path.join(root, 'planning/openapi.yaml'), 'utf8'));
const inspect = (source, layer = 'application') => inspectSource(path.join(root, `frontend/src/app/features/example/${layer}/fixture.ts`), source, roots);
for (const namespace of roots) {
  test(`rejects application route /${namespace} from the real OpenAPI contract`, () => {
    assert.ok(inspect(`const endpoint = '/${namespace}/example';`).some(error => error.includes('REST path')));
  });
}
test('finds new endpoint namespaces without editing the rule', () => {
  const future = contractRoots('paths:\n  \"/new-resource/{id}\":\n    get: {}\n');
  assert.ok(inspectSource(path.join(root, 'frontend/src/app/features/example/application/fixture.ts'), "const endpoint = '/new-resource/1';", future).length);
});
test('rejects templates, absolute endpoint URLs and versioned bases', () => {
  for (const source of [
    'const endpoint = `/spaces/${id}`;',
    'const endpoint = `${origin}/verifications/${id}`;',
    "const endpoint = 'https://example.test/auth/login';",
    "const endpoint = '/api/v1/auth/login';",
    "const endpoint = '/auth';",
    "const endpoint = '//example.test/spaces/1';",
    "const endpoint = '/' + 'spaces/' + id;",
  ]) assert.ok(inspect(source).length, source);
});
test('allows UI routes and infrastructure endpoint mapping', () => {
  assert.deepEqual(inspect("const route = '/mi-cuenta/verificaciones';"), []);
  assert.deepEqual(inspect("const endpoint = '/auth/login';", 'infrastructure'), []);
});
test('rejects an application transport import even without a URL literal', () => {
  assert.ok(inspect("import { HttpClient } from '@angular/common/http';").length);
});

test('rejects a dynamic global HTTP call in application', () => { assert.ok(inspect('fetch(endpoint);').length); });

test('rejects inline component CSS that would bypass Stylelint', () => { assert.ok(inspect('@Component({styles: [`.fixture { padding: 13px; }`]}) class Example {}', 'presentation').length); });
