import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";
import test from "node:test";
import ts from "typescript";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");

async function invokePageRedirect(path, destination) {
  const source = await readFile(resolve(root, path), "utf8");
  const javascript = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      jsx: ts.JsxEmit.ReactJSX,
    },
  }).outputText;
  let redirectedTo;
  const compiledModule = { exports: {} };
  vm.runInNewContext(javascript, {
    module: compiledModule,
    exports: compiledModule.exports,
    require: (name) => {
      if (name === "next/navigation") {
        return {
          redirect(path) {
            redirectedTo = path;
            throw new Error("NEXT_REDIRECT");
          },
        };
      }
      return new Proxy(function stub() {}, {
        get: () =>
          new Proxy(function stub() {}, { get: () => function stub() {} }),
      });
    },
  });
  assert.throws(() => compiledModule.exports.default(), /NEXT_REDIRECT/);
  assert.equal(redirectedTo, destination);
}

test("legacy dashboard routes redirect to canonical Mi cuenta", async (t) => {
  for (const [path, destination] of [
    ["src/app/(workspace)/dashboard/page.tsx", "/mi-cuenta"],
    ["src/app/(workspace)/dashboard/general/page.tsx", "/mi-cuenta"],
    ["src/app/(workspace)/dashboard/security/page.tsx", "/mi-cuenta/seguridad"],
    ["src/app/(workspace)/dashboard/activity/page.tsx", "/mi-cuenta"],
  ]) {
    await t.test(path, () => invokePageRedirect(path, destination));
  }
});

test("return-target allowlist keeps legacy dashboard bookmarks", async () => {
  const source = await readFile(
    resolve(root, "src/features/auth/domain/return-target.ts"),
    "utf8",
  );
  for (const alias of [
    "/dashboard",
    "/dashboard/general",
    "/dashboard/security",
    "/dashboard/activity",
  ]) {
    assert.match(source, new RegExp(`\\"${alias}\\"`));
  }
});

test("verification return links to Mi cuenta using canonical wording", async () => {
  const source = await readFile(
    resolve(root, "src/app/verificaciones/retorno/page.tsx"),
    "utf8",
  );
  assert.match(source, /href="\/mi-cuenta"[\s\S]*?Volver a Mi cuenta/);
  assert.doesNotMatch(source, /href="\/dashboard"[\s\S]*?Volver al panel/);
});
