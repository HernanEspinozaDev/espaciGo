import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import test from "node:test";
import ts from "typescript";

test("compiled host runtime facade purges private state on confirmed identity and permission changes", async () => {
  const dir = await mkdtemp(path.join(os.tmpdir(), "host-session-binding-"));
  try {
    let snapshot = {
      status: "authenticated",
      accountId: "owner-a",
      roles: ["host"],
      capabilities: ["listing:read"],
      restrictedMode: false,
      expiresAt: null,
    };
    const listeners = new Set();
    const authFacade = {
      getSnapshot: () => snapshot,
      getAccessToken: () =>
        `public-scope-marker:${snapshot.accountId}:${snapshot.capabilities.join(",")}:${snapshot.status}`,
      subscribe: (fn) => {
        listeners.add(fn);
        return () => listeners.delete(fn);
      },
      checkSession: async () => undefined,
    };
    let privateBlobClears = 0;
    let returnClears = 0;
    const port = {
      listListings: async () => [
        { id: "private-listing", title: "Synthetic private listing" },
      ],
      getListing: async (id) => ({ id, title: "Synthetic private listing" }),
      getPricing: async () => null,
      getWeeklySchedule: async () => null,
      listPhotos: async () => [],
    };
    const stripImports = (source) => {
      const parsed = ts.createSourceFile(
        "source.ts",
        source,
        ts.ScriptTarget.Latest,
        true,
      );
      for (const n of [...parsed.statements].reverse())
        if (ts.isImportDeclaration(n))
          source = source.slice(0, n.getFullStart()) + source.slice(n.end);
      return source;
    };
    const facadeSource = stripImports(
      await readFile(
        new URL(
          "../src/features/host-listings/application/host-listings-facade.ts",
          import.meta.url,
        ),
        "utf8",
      ),
    );
    const runtimeSource = stripImports(
      await readFile(
        new URL(
          "../src/features/host-listings/host-listings-runtime.ts",
          import.meta.url,
        ),
        "utf8",
      ),
    );
    globalThis.__hostSessionTest = {
      authFacade,
      port,
      privateBlobClear: () => privateBlobClears++,
      returnClear: () => returnClears++,
    };
    const facadeFile = path.join(dir, "facade.mjs");
    await writeFile(
      facadeFile,
      ts.transpileModule(
        facadeSource.replace(/^export\s+/gm, "") +
          "\nglobalThis.__hostSessionTest.RealHostListingsFacade = HostListingsFacade;",
        {
          compilerOptions: {
            target: ts.ScriptTarget.ES2022,
            module: ts.ModuleKind.ESNext,
          },
        },
      ).outputText,
    );
    await import(pathToFileURL(facadeFile));
    const runtimeBindings = `
      const authFacade = globalThis.__hostSessionTest.authFacade;
      const privateBlobUrls = { clear: () => globalThis.__hostSessionTest.privateBlobClear() };
      class HostListingsFacade { constructor(_adapter, auth, purge) { return new globalThis.__hostSessionTest.RealHostListingsFacade(globalThis.__hostSessionTest.port, auth, purge); } }
      class OpenApiHostListingsAdapter {}
      const createHostListingsFeatureBridge = () => ({});
      class ListingReturnsFacade { clear() { globalThis.__hostSessionTest.returnClear(); } }
      class ListingReturnsOpenApiAdapter {}
    `;
    const runtimeFile = path.join(dir, "runtime.mjs");
    await writeFile(
      runtimeFile,
      ts.transpileModule(runtimeBindings + runtimeSource, {
        compilerOptions: {
          target: ts.ScriptTarget.ES2022,
          module: ts.ModuleKind.ESNext,
        },
      }).outputText,
    );
    const { hostListingsFacade: facade } = await import(
      pathToFileURL(runtimeFile)
    );
    await facade.loadListings();
    await facade.selectListing("private-listing");
    assert.equal(facade.getSnapshot().selectedListing.id, "private-listing");
    assert.equal(
      listeners.size,
      2,
      "facade and return-scope runtime subscribe to auth",
    );
    const notifyAll = () => [...listeners].forEach((listener) => listener());
    const assertPurged = (next, expected) => {
      snapshot = next;
      notifyAll();
      assert.equal(facade.getSnapshot().selectedListing, null);
      assert.equal(facade.getSnapshot().listings.length, 0);
      assert.equal(privateBlobClears, expected);
      assert.equal(returnClears, expected);
    };
    assertPurged({ ...snapshot, accountId: "owner-b" }, 1);
    assertPurged(
      { ...snapshot, capabilities: ["listing:read", "listing:write"] },
      2,
    );
    assertPurged(
      {
        status: "anonymous",
        accountId: null,
        roles: [],
        capabilities: [],
        restrictedMode: false,
        expiresAt: null,
      },
      3,
    );
  } finally {
    delete globalThis.__hostSessionTest;
    await rm(dir, { recursive: true, force: true });
  }
});
