import assert from "node:assert/strict";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import * as prettier from "prettier";
import { parse } from "yaml";

const frontendRoot = fileURLToPath(new URL("../", import.meta.url));
const repositoryRoot = path.resolve(frontendRoot, "..");
const outputPath = path.join(
  repositoryRoot,
  "planning/frontend_next/evidence/matriz-operaciones-api-fase3.md",
);
const specPath = path.join(repositoryRoot, "planning/openapi.yaml");
const mainPath = path.join(repositoryRoot, "cmd/api/main.go");
const identityHandlerPath = path.join(
  repositoryRoot,
  "internal/identity/transport/http/handler.go",
);
const httpMethods = new Set([
  "get",
  "post",
  "put",
  "patch",
  "delete",
  "options",
  "head",
  "trace",
]);

const [specText, mainSource, identitySource] = await Promise.all([
  readFile(specPath, "utf8"),
  readFile(mainPath, "utf8"),
  readFile(identityHandlerPath, "utf8"),
]);
const spec = parse(specText);
assert.ok(spec?.paths && typeof spec.paths === "object");

function normalizePath(pathname) {
  return pathname
    .split("/")
    .map((segment) => (segment.startsWith("{") ? "{}" : segment))
    .join("/");
}

function lineAt(source, offset) {
  return source.slice(0, offset).split("\n").length;
}

function functionNameAt(source, offset) {
  const declarations = [
    ...source.matchAll(/^func\s+(?:\([^\n)]*\)\s*)?([A-Za-z_]\w*)\s*\(/gm),
  ];
  const enclosing = declarations.filter((match) => match.index < offset).at(-1);
  return enclosing?.[1] ?? "unknown";
}

function parseMounts(source) {
  const mounts = [];
  const pattern = /mux\.Handle(?:Func)?\(\s*"([^"]+)"/g;
  for (const match of source.matchAll(pattern)) {
    const raw = match[1];
    const route = /^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(\/.*)$/.exec(
      raw,
    );
    const method = route?.[1] ?? "*";
    const pathname = route?.[2] ?? raw;
    if (!pathname.startsWith("/api/v1")) continue;
    mounts.push({
      method,
      pathname,
      normalizedPath: normalizePath(pathname),
      line: lineAt(source, match.index),
      functionName: functionNameAt(source, match.index),
      prefix: pathname.endsWith("/"),
    });
  }
  return mounts;
}

const mounts = parseMounts(mainSource);
function sourceRange(startMarker, endMarker) {
  const start = mainSource.indexOf(startMarker);
  assert.notEqual(start, -1, `missing Go source marker: ${startMarker}`);
  const end = mainSource.indexOf(endMarker, start);
  assert.notEqual(end, -1, `missing Go source marker: ${endMarker}`);
  return {
    startLine: lineAt(mainSource, start),
    endLine: lineAt(mainSource, end),
  };
}

const authPrototypeRange = sourceRange(
  "if prototypeEnabled {",
  "\n\tserver := newHTTPServer",
);
const localTrialRange = sourceRange(
  'if os.Getenv("LOCAL_BOOKING_TRIAL") == "1" {',
  '\n\t\tmux.HandleFunc("GET /openapi.yaml"',
);

function isInRange(line, range) {
  return line >= range.startLine && line <= range.endLine;
}

function mountConditions(line) {
  const conditions = [];
  if (isInRange(line, authPrototypeRange)) {
    conditions.push("authPrototypeEnabled");
  }
  if (isInRange(line, localTrialRange)) {
    conditions.push("LOCAL_BOOKING_TRIAL=1");
  }
  return conditions;
}

function callsFromMain(functionName) {
  const pattern = new RegExp(`\\b${functionName}\\s*\\(`, "g");
  return [...mainSource.matchAll(pattern)]
    .filter((match) => {
      const before = mainSource.slice(
        Math.max(0, match.index - 12),
        match.index,
      );
      return !/func\s+$/.test(before);
    })
    .map((match) => lineAt(mainSource, match.index));
}

assert.ok(
  callsFromMain("run").length > 0,
  "main.go must invoke the run entrypoint",
);

function runtimeMount(mount) {
  // main() calls run(); helper registrations inherit the guards at their call sites.
  if (mount.functionName === "main" || mount.functionName === "run") {
    return {
      called: true,
      callLines: [mount.line],
      conditions: mountConditions(mount.line),
      detail: "",
    };
  }
  const callLines = callsFromMain(mount.functionName);
  if (callLines.length === 0) {
    return {
      called: false,
      callLines: [],
      conditions: [],
      detail: `helper ${mount.functionName} contiene la ruta en cmd/api/main.go:${mount.line}, pero no tiene llamada desde run`,
    };
  }
  const conditionSets = callLines.map((line) => mountConditions(line));
  const normalizedSets = new Set(
    conditionSets.map((conditions) => conditions.join(" + ")),
  );
  let conditions = [];
  let detail = `helper ${mount.functionName} contiene la ruta en cmd/api/main.go:${mount.line}; llamado desde cmd/api/main.go:${callLines.join(", ")}`;
  if (normalizedSets.size === 1) {
    conditions = conditionSets[0];
  } else if (conditionSets.some((items) => items.length === 0)) {
    detail +=
      "; existe al menos una llamada sin condición de montaje detectada";
  } else {
    conditions = [
      `condiciones distintas en llamadas de cmd/api/main.go:${callLines.join(", ")}`,
    ];
  }
  return { called: true, callLines, conditions, detail };
}

const runtimeMounts = mounts
  .map((mount) => ({ ...mount, ...runtimeMount(mount) }))
  .filter((mount) => mount.called);

const identityAllowlistStart = identitySource.indexOf(
  "paths := map[string]bool{",
);
assert.notEqual(identityAllowlistStart, -1);
const identityAllowlistEnd = identitySource.indexOf(
  "\n\t}",
  identityAllowlistStart,
);
assert.notEqual(identityAllowlistEnd, -1);
const identityAllowlistText = identitySource.slice(
  identityAllowlistStart,
  identityAllowlistEnd,
);
const identityPaths = new Set(
  [...identityAllowlistText.matchAll(/"(\/api\/v1\/[^\"]+)"\s*:\s*true/g)].map(
    (match) => match[1],
  ),
);
for (const match of identitySource.matchAll(
  /const\s+\w+\s*=\s*"(\/api\/v1\/[^\"]+)"/g,
)) {
  identityPaths.add(match[1]);
}

function identityMethods(pathname) {
  if (pathname.endsWith("/auth/session") || pathname.endsWith("/auth/terms")) {
    return ["GET"];
  }
  if (pathname.endsWith("/profile")) return ["GET", "PUT"];
  if (pathname.endsWith("/rights-requests")) return ["GET", "POST"];
  if (
    pathname.endsWith("/privacy/export") ||
    pathname.endsWith("/privacy/export/archive") ||
    pathname.endsWith("/privacy/suppression-requests") ||
    pathname.endsWith("/admin/credential-notices")
  ) {
    return ["GET"];
  }
  if (
    pathname.includes("/privacy/suppression-requests/") &&
    (pathname.endsWith("/review") || pathname.endsWith("/execute"))
  ) {
    return ["POST"];
  }
  if (
    pathname.startsWith("/api/v1/admin/credential-notices/") &&
    pathname.endsWith("/reopen")
  ) {
    return ["POST"];
  }
  if (identityPaths.has(pathname)) return ["POST"];
  return [];
}

function mountStatus(pathname, method) {
  const fullPath = `/api/v1${pathname}`;
  const normalized = normalizePath(fullPath);
  const exact = runtimeMounts.filter(
    (mount) => !mount.prefix && mount.normalizedPath === normalized,
  );
  const methodExact = exact.find((mount) => mount.method === method);
  if (methodExact) {
    return {
      kind: "directo",
      detail: `método y ruta exactos en cmd/api/main.go:${methodExact.line}${methodExact.detail ? `; ${methodExact.detail}` : ""}`,
      conditions: methodExact.conditions,
    };
  }
  const methodAgnostic = exact.find((mount) => mount.method === "*");
  if (methodAgnostic) {
    return {
      kind: "no verificado",
      detail: `ruta exacta en cmd/api/main.go:${methodAgnostic.line}, pero el método se delega al handler y esta operación no se verificó allí${methodAgnostic.detail ? `; ${methodAgnostic.detail}` : ""}`,
      conditions: methodAgnostic.conditions,
    };
  }

  const prefixes = runtimeMounts
    .filter(
      (mount) =>
        mount.prefix &&
        `/api/v1${pathname}`.startsWith(mount.pathname) &&
        mount.pathname !== "/api/v1/",
    )
    .sort((left, right) => right.pathname.length - left.pathname.length);
  if (prefixes.length > 0) {
    const selected = prefixes[0];
    return {
      kind: "familia/prefijo",
      detail: `familia ${selected.pathname} registrada en cmd/api/main.go:${selected.line}; path/método de esta operación no verificados en el handler${selected.detail ? `; ${selected.detail}` : ""}`,
      conditions: selected.conditions,
    };
  }

  const explicitIdentity = [...identityPaths].some(
    (candidate) => normalizePath(candidate) === normalized,
  );
  const dynamicIdentity =
    (fullPath.includes("/privacy/suppression-requests/") &&
      (fullPath.endsWith("/review") || fullPath.endsWith("/execute"))) ||
    (fullPath.startsWith("/api/v1/admin/credential-notices/") &&
      fullPath.endsWith("/reopen"));
  if (
    (explicitIdentity || dynamicIdentity) &&
    identityMethods(fullPath).includes(method)
  ) {
    const rootMount = runtimeMounts.find(
      (mount) => mount.pathname === "/api/v1/" && mount.prefix,
    );
    return {
      kind: "directo",
      detail: `path allowlist y método explícitos en internal/identity/transport/http/handler.go:110-154; mux /api/v1/ en cmd/api/main.go:${rootMount?.line ?? "?"}`,
      conditions: rootMount ? rootMount.conditions : [],
    };
  }
  const rootMount = runtimeMounts.find(
    (mount) => mount.pathname === "/api/v1/" && mount.prefix,
  );
  if (rootMount) {
    return {
      kind: "no verificado",
      detail: `solo coincide con el catch-all /api/v1/ de cmd/api/main.go:${rootMount.line}; el path/método no está confirmado por la allowlist del handler`,
      conditions: rootMount.conditions,
    };
  }
  return {
    kind: "sin montaje",
    detail: "sin registro coincidente en cmd/api/main.go",
    conditions: [],
  };
}

function operationAuth(operation) {
  if (Object.hasOwn(operation, "security")) {
    return operation.security.length === 0
      ? "Pública (security: [])"
      : operation.security
          .map((item) => Object.keys(item).join("+"))
          .join(" o ");
  }
  return (spec.security ?? []).length === 0
    ? "Sin esquema global"
    : (spec.security ?? [])
        .map((item) => Object.keys(item).join("+"))
        .join(" o ");
}

function declaredPolicy(operation, operationText) {
  const roles = operation["x-roles"] ?? operation["x-role"];
  const capabilities = operation["x-capabilities"] ?? operation["x-capability"];
  const capabilityEnum = spec.components?.schemas?.AdminCapability?.enum ?? [];
  const mentions = capabilityEnum.filter((capability) =>
    operationText.includes(capability),
  );
  const policy = [];
  if (roles) policy.push(`roles: ${JSON.stringify(roles)}`);
  if (capabilities) policy.push(`capacidades: ${JSON.stringify(capabilities)}`);
  if (!capabilities && mentions.length > 0) {
    policy.push(`capacidades citadas: ${mentions.join(", ")}`);
  }
  return policy.length > 0
    ? `${policy.join("; ")}; Go valida autorización`
    : "OpenAPI no declara rol/capacidad específica; Go es autoridad";
}

function futureUiPhase(pathname, tags) {
  if (pathname.startsWith("/public/")) return "Fase 4";
  if (
    pathname === "/auth/login" ||
    pathname === "/auth/session" ||
    pathname === "/auth/logout"
  ) {
    return "Fase 3: integración de sesión";
  }
  if (pathname.startsWith("/auth/")) return "Fase 5";
  if (
    pathname.startsWith("/profile") ||
    pathname.startsWith("/payout-account") ||
    pathname.startsWith("/rights-requests") ||
    pathname.startsWith("/privacy/") ||
    pathname.startsWith("/verifications")
  ) {
    return pathname.startsWith("/admin/") ? "Fase 9" : "Fase 6";
  }
  if (
    pathname.startsWith("/local/booking-trial/") ||
    pathname.startsWith("/admin/disputes") ||
    pathname.startsWith("/local/reputation/") ||
    pathname.includes("/reviews") ||
    pathname.includes("damage-claim") ||
    pathname.includes("/operations") ||
    pathname.includes("/contracts/")
  ) {
    return "Fase 8";
  }
  if (pathname.startsWith("/admin/")) return "Fase 9";
  if (pathname.startsWith("/spaces/")) return "Fase 7";
  if (pathname === "/spaces") return "Fase 7";
  const tag = tags[0] ?? "";
  if (/M0[1-3]/i.test(tag)) return /M01/i.test(tag) ? "Fase 5" : "Fase 6";
  if (/M0[4-5]/i.test(tag)) return "Fase 7";
  if (/M0[6-8]/i.test(tag)) return "Fase 8";
  if (/M09|global administration/i.test(tag)) return "Fase 9";
  return "Fase por confirmar según contrato/plan";
}

function cell(value) {
  return String(value ?? "—")
    .replaceAll("|", "\\|")
    .replaceAll("\n", " ")
    .replaceAll("\r", "");
}

const rows = [];
for (const [pathname, pathItem] of Object.entries(spec.paths)) {
  for (const [method, operation] of Object.entries(pathItem)) {
    if (!httpMethods.has(method.toLowerCase())) continue;
    const tags = operation.tags ?? pathItem.tags ?? [];
    const operationText = [
      operation.summary ?? "",
      operation.description ?? "",
      JSON.stringify(operation["x-capabilities"] ?? ""),
    ].join(" ");
    const mount = mountStatus(pathname, method.toUpperCase());
    rows.push({
      method: method.toUpperCase(),
      pathname,
      operationId: operation.operationId ?? "sin operationId",
      tag: tags.join(", ") || "sin tag",
      auth: operationAuth(operation),
      policy: declaredPolicy(operation, operationText),
      mountClass: mount.kind,
      mount: mount.detail,
      mountCondition:
        mount.conditions.length > 0
          ? `condicional: ${mount.conditions.join(" + ")}`
          : "sin condición de montaje detectada",
      phase: futureUiPhase(pathname, tags),
    });
  }
}
assert.ok(rows.length > 0, "OpenAPI inventory must not be empty");
rows.sort((left, right) =>
  `${left.pathname} ${left.method}`.localeCompare(
    `${right.pathname} ${right.method}`,
  ),
);

const mountClasses = [
  "directo",
  "familia/prefijo",
  "no verificado",
  "sin montaje",
];
const mountSummary = mountClasses.map(
  (kind) => `${kind}: ${rows.filter((row) => row.mountClass === kind).length}`,
);
const conditionalCount = rows.filter((row) =>
  row.mountCondition.startsWith("condicional:"),
).length;
const authPrototypeCount = rows.filter((row) =>
  row.mountCondition.includes("authPrototypeEnabled"),
).length;
const localTrialCount = rows.filter((row) =>
  row.mountCondition.includes("LOCAL_BOOKING_TRIAL=1"),
).length;
const roles =
  spec.components?.schemas?.LoginResponse?.properties?.roles?.items?.enum ?? [];
const capabilityValues = spec.components?.schemas?.AdminCapability?.enum ?? [];
const operationRows = rows
  .map(
    (row, index) =>
      `${index + 1} | ${cell(row.method)} | \`${cell(row.pathname)}\` | \`${cell(row.operationId)}\` | ${cell(row.tag)} | ${cell(row.auth)} | ${cell(row.policy)} | ${cell(row.mountClass)}: ${cell(row.mount)} | ${cell(row.mountCondition)} | ${cell(row.phase)} |`,
  )
  .join("\n");
const markdown = `# Matriz de operaciones OpenAPI y montaje Go — Fase 3

Generada desde \`planning/openapi.yaml\`, \`cmd/api/main.go\` y \`internal/identity/transport/http/handler.go\`. Total: ${rows.length} operaciones OpenAPI; la presencia en OpenAPI no se toma como prueba del montaje. Conteo por evidencia principal: ${mountSummary.join("; ")}. Montajes condicionales: ${conditionalCount} (solapados con la clase principal); \`authPrototypeEnabled\`: ${authPrototypeCount}; \`LOCAL_BOOKING_TRIAL=1\`: ${localTrialCount}. \`Directo\` indica ruta/método explícitos o allowlist+dispatch explícitos en el handler. \`Familia/prefijo\` acredita solo el registro del prefijo; el path/método concreto queda sin verificar. \`No verificado\` incluye rutas con método delegado o que solo coinciden con el catch-all. La columna de condición identifica ramas de montaje y no presume que estén activas. \`Sin montaje\` no coincide con registro alguno. No se realizaron llamadas mutantes desde este inventario.

Roles enumerados por el contrato: ${roles.map((role) => `\`${role}\``).join(", ") || "ninguno"}. Capacidades enumeradas por el contrato: ${capabilityValues.map((capability) => `\`${capability}\``).join(", ") || "ninguna"}. La columna de autorización refleja únicamente lo declarado en OpenAPI; si no hay rol/capacidad explícita, el handler/servicio Go es la autoridad y no se deduce autorización desde etiquetas o nombres de ruta. \"Fase UI futura\" es planificación, no funcionalidad implementada.

| # | Método | Path OpenAPI | operationId | Tag | Auth del contrato | Rol/capacidad declarados | Clase/montaje Go | Condición del montaje | Fase UI futura |\n|---:|---|---|---|---|---|---|---|---|---|\n|${operationRows.replaceAll("\n", "\n|")}
`;
const formattedMarkdown = await prettier.format(markdown, {
  parser: "markdown",
});

if (process.argv.includes("--check")) {
  const current = await readFile(outputPath, "utf8");
  assert.equal(
    current,
    formattedMarkdown,
    "API operation matrix is stale; run pnpm api:inventory",
  );
  console.log(`API inventory current: ${rows.length} operations`);
} else {
  await writeFile(outputPath, formattedMarkdown, "utf8");
  console.log(
    `Wrote API inventory: ${rows.length} operations; ${mountSummary.join("; ")}; montajes condicionales: ${conditionalCount}`,
  );
}
