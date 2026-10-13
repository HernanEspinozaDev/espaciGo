import { test } from 'node:test';
import assert from 'node:assert/strict';
import stylelint from 'stylelint';
import plugin from './design-token-rule.mjs';
const lint = async (code) =>
  (
    await stylelint.lint({
      code,
      customSyntax: 'postcss-scss',
      config: { plugins: [plugin], rules: { 'espacigo/design-tokens': true } },
    })
  ).results[0].warnings;
for (const declaration of [
  'padding: 13px',
  'gap: .37rem',
  'margin-inline: 2vw',
  'font-size: 1.03rem',
  'font-weight: 623',
  'font-family: Arial',
  'line-height: 1.43',
  'letter-spacing: .02em',
]) {
  test(`rejects unreviewed ${declaration}`, async () =>
    assert.ok((await lint(`.fixture { ${declaration}; }`)).length));
}
test('rejects raw viewport thresholds', async () => {
  assert.ok((await lint('@media (max-width: 731px) { .fixture { display: grid; } }')).length);
});
test('allows tokens, zero/auto and shared responsive mixins', async () => {
  assert.deepEqual(
    await lint(
      "@include bp.down('viewport-40rem') { .fixture { padding: 0 metrics.$space-1rem; gap: var(--space-4); font-size: metrics.$font-size-1rem; font: inherit; margin: auto; } }",
    ),
    [],
  );
});
test('a token beside an arbitrary literal does not conceal the literal', async () => {
  assert.ok((await lint('.fixture { padding: var(--space-4) 13px; }')).length);
});

test('rejects local variables, unknown tokens and arbitrary fallbacks', async () => {
  for (const value of ['$custom', 'metrics.$missing', 'var(--custom)', 'var(--space-4, 13px)', '1PX', '2dvh']) {
    assert.ok((await lint(`.fixture { padding: ${value}; }`)).length, value);
  }
});
