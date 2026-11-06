import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const root = path.resolve(import.meta.dirname, "..");
const source = await readFile(
  path.join(root, "src/features/host-listings/ui/listing-editor.tsx"),
  "utf8",
);
const parsed = ts.createSourceFile(
  "editor.tsx",
  source,
  ts.ScriptTarget.Latest,
  true,
  ts.ScriptKind.TSX,
);
const mapper = parsed.statements.find(
  (node) =>
    ts.isFunctionDeclaration(node) &&
    node.name?.text === "mapDraftServerFieldErrors",
);
assert.ok(mapper, "editor field-error mapper exists");
class ApiError extends Error {
  constructor(fields) {
    super("Private upstream message");
    this.status = 422;
    this.code = "validation_error";
    this.fields = fields;
  }
}
const code = ts.transpileModule(mapper.getText(parsed), {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.None,
  },
}).outputText;
const map = new Function(
  "ApiError",
  `${code}; return mapDraftServerFieldErrors;`,
)(ApiError);
const schema = { attributes: [{ code: "capacidad_interior" }] };

test("unknown field/code pairs cannot create field-specific UI errors", () => {
  assert.deepEqual(
    map(
      new ApiError([
        {
          field: "private_owner_id",
          code: "required",
          message: "private value",
        },
        {
          field: "title",
          code: "unknown_private_code",
          message: "private value",
        },
        {
          field: "attributes.capacidad_interior",
          code: "unknown_private_code",
          message: "private value",
        },
        {
          field: "attributes.unknown",
          code: "required",
          message: "private value",
        },
      ]),
      schema,
    ),
    {},
  );
});

test("recognized field/code pairs remain mapped in a mixed response", () => {
  assert.deepEqual(
    map(
      new ApiError([
        { field: "title", code: "max_length" },
        { field: "attributes.capacidad_interior", code: "required" },
        { field: "private_owner_id", code: "required" },
      ]),
      schema,
    ),
    {
      title: "El título no puede superar 70 caracteres.",
      "attributes.capacidad_interior": "Completa este atributo.",
    },
  );
});
