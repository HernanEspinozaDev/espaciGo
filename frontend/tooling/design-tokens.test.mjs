import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const globalCss = await readFile(
  new URL("../src/app/globals.css", import.meta.url),
  "utf8",
);
const workspaceCss = await readFile(
  new URL("../src/app/workspace-template.css", import.meta.url),
  "utf8",
);

test("the shared Tailwind setup retains marketplace-compatible theme tokens", () => {
  assert.match(globalCss, /@import "tailwindcss"/);
  assert.match(globalCss, /@theme inline/);
  assert.match(globalCss, /--color-background: var\(--background\)/);
  assert.match(globalCss, /--color-primary: var\(--primary\)/);
  assert.match(globalCss, /yns-image-shimmer/);
  assert.match(globalCss, /@import "\.\/workspace-template\.css"/);
});

test("SaaS Starter palette is scoped to its route-group shell", () => {
  assert.match(workspaceCss, /\.saas-application\s*\{/);
  assert.match(workspaceCss, /\.saas-application\.dark\s*\{/);
  assert.doesNotMatch(workspaceCss, /(^|\n)\s*:root\s*\{/);
  for (const token of [
    "--background",
    "--foreground",
    "--primary",
    "--sidebar-background",
    "--chart-1",
  ]) {
    assert.match(
      workspaceCss,
      new RegExp(`${token}:`),
      `missing adapted starter token ${token}`,
    );
  }
});
