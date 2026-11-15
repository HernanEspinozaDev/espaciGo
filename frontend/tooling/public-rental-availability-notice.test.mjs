import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const noticeUrl = new URL(
  "../src/features/public-catalog/domain/availability-notice.ts",
  import.meta.url,
);
const source = await readFile(noticeUrl, "utf8");
const ts = await import("typescript");
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  },
}).outputText;
const noticeModule = { exports: {} };
new Function("module", "exports", compiled)(noticeModule, noticeModule.exports);

test("public availability copy describes a non-binding point-in-time query", () => {
  assert.equal(
    noticeModule.exports.publicAvailabilityNotice,
    "Consulta los intervalos que informa el servidor para la fecha seleccionada. La respuesta refleja el estado observado al momento de la consulta; no congela ni reserva el horario.",
  );
});
