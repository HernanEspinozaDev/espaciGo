import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const source = await readFile(new URL('../src/styles/_tokens.scss', import.meta.url), 'utf8');
const tokens = Object.fromEntries(
  [...source.matchAll(/(--[\w-]+):\s*(#[\da-fA-F]{3}(?:[\da-fA-F]{3})?)\s*;/g)].map(([, name, value]) => [
    name,
    value.length === 4 ? `#${[...value.slice(1)].map((channel) => channel + channel).join('')}` : value,
  ]),
);

function linearize(channel) {
  const normalized = channel / 255;
  return normalized <= 0.04045 ? normalized / 12.92 : ((normalized + 0.055) / 1.055) ** 2.4;
}

function luminance(hex) {
  const channels = hex.slice(1).match(/../g).map((part) => Number.parseInt(part, 16));
  return 0.2126 * linearize(channels[0]) + 0.7152 * linearize(channels[1]) + 0.0722 * linearize(channels[2]);
}

function contrast(first, second) {
  const values = [luminance(first), luminance(second)].sort((a, b) => b - a);
  return (values[0] + 0.05) / (values[1] + 0.05);
}

const textPairs = [
  ['--color-text', '--color-canvas'],
  ['--color-text', '--color-surface'],
  ['--color-text-muted', '--color-canvas'],
  ['--color-text-muted', '--color-surface'],
  ['--color-text-muted', '--color-surface-subtle'],
  ['--brand-primary', '--color-canvas'],
  ['--brand-primary', '--color-surface'],
  ['--status-success-text', '--status-success-background'],
  ['--status-warning-text', '--status-warning-background'],
  ['--status-error-text', '--status-error-background'],
  ['--status-info-text', '--status-info-background'],
  ['--status-neutral-text', '--status-neutral-background'],
];

for (const [foreground, background] of textPairs) {
  test(`${foreground} has WCAG AA text contrast on ${background}`, () => {
    assert.ok(tokens[foreground], `missing ${foreground}`);
    assert.ok(tokens[background], `missing ${background}`);
    assert.ok(contrast(tokens[foreground], tokens[background]) >= 4.5,
      `${contrast(tokens[foreground], tokens[background]).toFixed(2)}:1 is below 4.5:1`);
  });
}

test('focus indicator has non-text contrast on the main surfaces', () => {
  for (const surface of ['--color-canvas', '--color-surface']) {
    assert.ok(contrast(tokens['--color-focus'], tokens[surface]) >= 3,
      `${surface} does not provide 3:1 contrast for the focus indicator`);
  }
});

test('disabled control border has non-text contrast on its background', () => {
  assert.ok(contrast(tokens['--color-border-control'], tokens['--color-surface-subtle']) >= 3,
    'disabled control border does not provide 3:1 contrast');
});
