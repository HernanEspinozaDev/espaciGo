import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import openapiTS, { astToString } from "openapi-typescript";
import { format } from "prettier";
import ts from "typescript";
import { parse } from "yaml";

const frontendRoot = fileURLToPath(new URL("../", import.meta.url));
const repositoryRoot = path.resolve(frontendRoot, "..");
const schemaPath = path.join(repositoryRoot, "planning/openapi.yaml");
const defaultOutput = path.join(
  frontendRoot,
  "src/lib/api/generated/openapi.d.ts",
);
const outputPath = process.argv.includes("--output")
  ? path.resolve(process.argv[process.argv.indexOf("--output") + 1])
  : defaultOutput;

if (!outputPath || outputPath === path.resolve(".")) {
  throw new Error("Invalid OpenAPI output path");
}

const source = await readFile(schemaPath, "utf8");
const schema = parse(source);
const ast = await openapiTS(schema, {
  cwd: repositoryRoot,
  readWriteMarkers: true,
  transform(schemaObject) {
    const isInteger64 =
      schemaObject.format === "int64" &&
      (schemaObject.type === "integer" ||
        (Array.isArray(schemaObject.type) &&
          schemaObject.type.includes("integer")));
    if (!isInteger64) return undefined;

    const members = [
      ts.factory.createKeywordTypeNode(ts.SyntaxKind.NumberKeyword),
      ts.factory.createKeywordTypeNode(ts.SyntaxKind.StringKeyword),
    ];
    if (
      schemaObject.nullable === true ||
      (Array.isArray(schemaObject.type) && schemaObject.type.includes("null"))
    ) {
      members.push(ts.factory.createLiteralTypeNode(ts.factory.createNull()));
    }
    return ts.factory.createUnionTypeNode(members);
  },
});

await mkdir(path.dirname(outputPath), { recursive: true });
await writeFile(
  outputPath,
  await format(astToString(ast), { filepath: outputPath }),
  "utf8",
);
