import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const css = await readFile(
  new URL("../src/app/globals.css", import.meta.url),
  "utf8",
);
const tokens = new Map(
  Array.from(
    css.matchAll(/(--color-[a-z-]+):\s*(#[0-9a-f]{6})/gi),
    ([, name, value]) => [name, value],
  ),
);

function luminance(hex) {
  const channels = hex
    .slice(1)
    .match(/.{2}/g)
    .map((value) => Number.parseInt(value, 16) / 255);
  const linear = channels.map((value) =>
    value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4,
  );
  return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
}

function contrast(foreground, background) {
  const values = [luminance(foreground), luminance(background)].sort(
    (a, b) => b - a,
  );
  return (values[0] + 0.05) / (values[1] + 0.05);
}

test("shared design tokens meet WCAG AA for declared text/surface pairs", () => {
  const pairs = [
    ["--color-ink", "--color-canvas"],
    ["--color-ink", "--color-surface"],
    ["--color-ink", "--color-surface-soft"],
    ["--color-ink-muted", "--color-canvas"],
    ["--color-ink-muted", "--color-surface"],
    ["--color-brand", "--color-canvas"],
    ["--color-brand-deep", "--color-surface"],
    ["--color-success", "--color-success-surface"],
    ["--color-warning", "--color-warning-surface"],
    ["--color-error", "--color-error-surface"],
    ["--color-info", "--color-info-surface"],
    ["--color-focus", "--color-surface"],
    ["--color-on-deep", "--color-deep"],
    ["--color-deep-muted", "--color-deep"],
    ["#ffffff", "--color-brand"],
  ];

  for (const [foregroundName, backgroundName] of pairs) {
    const foreground = foregroundName.startsWith("#")
      ? foregroundName
      : tokens.get(foregroundName);
    const background = backgroundName.startsWith("#")
      ? backgroundName
      : tokens.get(backgroundName);
    assert.ok(foreground, `missing foreground token ${foregroundName}`);
    assert.ok(background, `missing background token ${backgroundName}`);
    assert.ok(
      contrast(foreground, background) >= 4.5,
      `${foregroundName} on ${backgroundName} has ${contrast(foreground, background).toFixed(2)}:1 contrast`,
    );
  }
});

test("spacing, radius, elevation, typography, and density tokens are defined", () => {
  for (const name of [
    "--space-1",
    "--space-4",
    "--space-8",
    "--space-public-section",
    "--space-admin-gap",
    "--radius-sm",
    "--radius-lg",
    "--shadow-soft",
    "--shadow-lift",
    "--font-sans",
    "--font-editorial",
  ]) {
    assert.match(css, new RegExp(`${name}:`), `missing design token ${name}`);
  }
});
