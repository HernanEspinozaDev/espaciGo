import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import test from "node:test";
import ts from "typescript";

test("host runtime clears private state when the confirmed session identity or permissions change", async () => {
  const dir = await mkdtemp(path.join(os.tmpdir(), "host-session-binding-"));
  try {
    let snapshot = {
      status: "anonymous",
      accountId: null,
      roles: [],
      capabilities: [],
      restrictedMode: false,
    };
    let notify;
    let clears = 0;
    let returnClears = 0;
    globalThis.__hostSessionTest = {
      authFacade: {
        getSnapshot: () => snapshot,
        subscribe: (fn) => {
          notify = fn;
          return () => {};
        },
      },
      HostListingsFacade: class {
        clear() {
          clears++;
        }
      },
      OpenApiHostListingsAdapter: class {},
      ListingReturnsFacade: class {
        clear() {
          returnClears++;
        }
      },
      ListingReturnsOpenApiAdapter: class {},
    };
    const source = await readFile(
      new URL(
        "../src/features/host-listings/host-listings-runtime.ts",
        import.meta.url,
      ),
      "utf8",
    );
    const parsed = ts.createSourceFile(
      "runtime.ts",
      source,
      ts.ScriptTarget.Latest,
      true,
    );
    let body = source;
    for (const n of [...parsed.statements].reverse())
      if (ts.isImportDeclaration(n))
        body = body.slice(0, n.getFullStart()) + body.slice(n.end);
    const file = path.join(dir, "runtime.mjs");
    await writeFile(
      file,
      ts.transpileModule(
        "const { authFacade, HostListingsFacade, OpenApiHostListingsAdapter, ListingReturnsFacade, ListingReturnsOpenApiAdapter }=globalThis.__hostSessionTest;\n" +
          body,
        {
          compilerOptions: {
            target: ts.ScriptTarget.ES2022,
            module: ts.ModuleKind.ESNext,
          },
        },
      ).outputText,
    );
    await import(pathToFileURL(file));
    assert.equal(typeof notify, "function");
    snapshot = {
      ...snapshot,
      status: "authenticated",
      accountId: "synthetic-owner",
    };
    notify();
    assert.equal(clears, 1);
    notify();
    assert.equal(clears, 1);
    snapshot = { ...snapshot, capabilities: ["synthetic-capability"] };
    notify();
    assert.equal(clears, 2);
    snapshot = { ...snapshot, status: "anonymous", accountId: null };
    notify();
    assert.equal(clears, 3);
    assert.equal(returnClears, 3);
  } finally {
    delete globalThis.__hostSessionTest;
    await rm(dir, { recursive: true, force: true });
  }
});
