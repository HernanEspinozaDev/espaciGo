import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { format } from "prettier";
import { parse } from "yaml";

const frontendRoot = fileURLToPath(new URL("../", import.meta.url));
const repositoryRoot = path.resolve(frontendRoot, "..");
const schemaPath = path.join(repositoryRoot, "planning/openapi.yaml");
const defaultOutput = path.join(
  frontendRoot,
  "src/lib/api/generated/gateway-operations.ts",
);
const outputIndex = process.argv.indexOf("--output");
const outputPath =
  outputIndex === -1
    ? defaultOutput
    : path.resolve(process.argv[outputIndex + 1] ?? "");
const spec = parse(await readFile(schemaPath, "utf8"));
const methods = new Set(["get", "post", "put", "patch", "delete", "head"]);

function resolveComponent(reference, componentName) {
  if (!reference?.$ref) return reference;
  const match = reference.$ref.match(
    new RegExp(`^#/components/${componentName}/([^/]+)$`),
  );
  if (!match)
    throw new Error(
      `Unsupported ${componentName} reference: ${reference.$ref}`,
    );
  const name = match[1].replaceAll("~1", "/").replaceAll("~0", "~");
  const resolved = spec.components?.[componentName]?.[name];
  if (!resolved)
    throw new Error(`Unknown ${componentName} reference: ${reference.$ref}`);
  return resolved;
}

function resolveParameter(parameter) {
  return resolveComponent(parameter, "parameters");
}

function contentTypes(content) {
  return Object.keys(content ?? {}).sort();
}

const operations = [];
for (const [pathname, pathItem] of Object.entries(spec.paths ?? {}).sort(
  ([left], [right]) => left.localeCompare(right),
)) {
  for (const [method, operation] of Object.entries(pathItem ?? {})
    .filter(([key]) => methods.has(key.toLowerCase()))
    .sort(([left], [right]) => left.localeCompare(right))) {
    const parameters = [
      ...(pathItem.parameters ?? []),
      ...(operation.parameters ?? []),
    ].map(resolveParameter);
    const query = parameters
      .filter((parameter) => parameter.in === "query")
      .filter((parameter) => Boolean(parameter.name))
      .map((parameter) => {
        const style = parameter.style ?? "form";
        const explode = parameter.explode ?? style === "form";
        return {
          name: parameter.name,
          allowRepeated: parameter.schema?.type === "array" && explode,
        };
      })
      .sort((left, right) => left.name.localeCompare(right.name));
    const headers = parameters
      .filter((parameter) => parameter.in === "header")
      .map((parameter) => parameter.name?.toLowerCase())
      .filter(Boolean)
      .sort();
    const security = operation.security ?? spec.security ?? [];
    const requestBody = resolveComponent(
      operation.requestBody,
      "requestBodies",
    );
    const requestMediaTypes = contentTypes(requestBody?.content);
    const responseMediaTypes = [
      ...new Set(
        Object.values(operation.responses ?? {}).flatMap((response) => {
          const resolved = resolveComponent(response, "responses");
          return contentTypes(resolved?.content);
        }),
      ),
    ].sort();
    operations.push({
      method: method.toUpperCase(),
      path: pathname,
      operationId: operation.operationId ?? "",
      query,
      headers,
      bearer: security.some((requirement) => "bearerAuth" in requirement),
      requestMediaTypes,
      responseMediaTypes,
    });
  }
}

const body = `// Generated from planning/openapi.yaml by pnpm api:generate. Do not edit.\nexport const gatewayOperations = ${JSON.stringify(operations, null, 2)} as const;\n`;
await mkdir(path.dirname(outputPath), { recursive: true });
await writeFile(
  outputPath,
  await format(body, { filepath: outputPath }),
  "utf8",
);
console.log(`Wrote gateway allowlist: ${operations.length} operations`);
