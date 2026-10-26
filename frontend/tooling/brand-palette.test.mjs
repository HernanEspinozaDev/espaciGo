import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import test from "node:test";

const frontend = fileURLToPath(new URL("../", import.meta.url));
const fromNext = createRequire(import.meta.resolve("next/package.json"));
const postcss = fromNext("postcss");
const source = (name) => readFileSync(`${frontend}src/app/${name}`, "utf8");
const brand = existsSync(`${frontend}src/app/brand-palette.css`)
  ? source("brand-palette.css")
  : "";
const semantic = new Set([
  "--background",
  "--foreground",
  "--card",
  "--card-foreground",
  "--popover",
  "--popover-foreground",
  "--primary",
  "--primary-foreground",
  "--secondary",
  "--secondary-foreground",
  "--muted",
  "--muted-foreground",
  "--accent",
  "--accent-foreground",
  "--destructive",
  "--border",
  "--input",
  "--ring",
]);
const isColor = (name) =>
  semantic.has(name) ||
  /^--(?:brand-|color-|sidebar-|chart-)/.test(name) ||
  [
    "background",
    "color",
    "background-color",
    "border-color",
    "outline-color",
  ].includes(name);

function nonColorSignature(css) {
  const root = postcss.parse(css);
  const signature = [];
  root.walk((node) => {
    if (node.type === "decl" && !isColor(node.prop)) {
      const parents = [];
      for (
        let parent = node.parent;
        parent && parent.type !== "root";
        parent = parent.parent
      ) {
        parents.unshift(parent.selector ?? `@${parent.name} ${parent.params}`);
      }
      signature.push([parents, node.prop, node.value]);
    }
    if (node.type === "atrule" && node.name !== "import") {
      signature.push([`@${node.name}`, node.params]);
    }
    if (
      node.type === "atrule" &&
      node.name === "import" &&
      node.params !== '"./brand-palette.css"'
    ) {
      signature.push(["@import", node.params]);
    }
  });
  return signature;
}

test("brand palette defines the requested primary and canvas for existing Next scopes", () => {
  assert.match(brand, /--brand-primary:\s*#a44731\s*;/i);
  assert.match(brand, /--brand-canvas:\s*#fbf7f2\s*;/i);
  for (const file of ["globals.css", "workspace-template.css"]) {
    assert.match(source(file), /--primary:\s*var\(--brand-primary\)/);
    assert.match(source(file), /--background:\s*var\(--brand-canvas\)/);
  }
});

test("existing CSS preserves all non-color declarations and directives from the main reference", () => {
  // Captured from the normalized non-color AST of main 8a87e38.
  // Frozen digests keep the guard usable in shallow CI/Docker without .git.
  const reference = {
    "globals.css":
      "7bbda892849d3d96266dfaf7b47faa515fc95ffc2023934960d22065c9df1fed",
    "workspace-template.css":
      "69a255fa204a8204cde51ef4b9dba7d9f41387b898c5a7c9900044d902f50611",
  };
  for (const [name, expected] of Object.entries(reference)) {
    const actual = createHash("sha256")
      .update(JSON.stringify(nonColorSignature(source(name))))
      .digest("hex");
    assert.equal(actual, expected, name);
  }
});

test("brand stylesheet adds no geometry, typography, layout, network imports or component selectors", () => {
  assert.ok(brand.length > 0, "brand stylesheet is required");
  const root = postcss.parse(brand);
  root.walkDecls((decl) =>
    assert.ok(isColor(decl.prop), `Non-color declaration: ${decl.prop}`),
  );
  root.walkAtRules((rule) =>
    assert.equal(rule.name, "theme", `Unexpected @${rule.name}`),
  );
  root.walkRules((rule) => assert.equal(rule.selector, ":root"));
});
