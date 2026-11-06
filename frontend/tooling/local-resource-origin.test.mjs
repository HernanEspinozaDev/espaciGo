import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

const source = await readFile(
  new URL("../tests/helpers/local-resource-origin.ts", import.meta.url),
  "utf8",
);
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  },
}).outputText;
const loaded = { exports: {} };
new Function("module", "exports", compiled)(loaded, loaded.exports);
const { isAllowedLocalResource } = loaded.exports;

test("accepts only same-origin local resources and same-origin blob URLs", () => {
  const base = "http://127.0.0.1:4218";
  assert.equal(
    isAllowedLocalResource("http://127.0.0.1:4218/path", base),
    true,
  );
  assert.equal(
    isAllowedLocalResource("blob:http://127.0.0.1:4218/id", base),
    true,
  );
  assert.equal(isAllowedLocalResource("blob:null/id", base), false);
  assert.equal(
    isAllowedLocalResource("blob:https://remote.example/id", base),
    false,
  );
  assert.equal(
    isAllowedLocalResource("https://remote.example/file", base),
    false,
  );
});
