import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-cookie-scope-"),
);
async function compile(relative, name, replacements = []) {
  let source = await readFile(path.join(root, relative), "utf8");
  source = source.replaceAll('import "client-only";', "");
  for (const [before, after] of replacements)
    source = source.replaceAll(before, after);
  const destination = path.join(scratch, name);
  await writeFile(
    destination,
    ts.transpileModule(source, {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ESNext,
      },
    }).outputText,
  );
  return import(pathToFileURL(destination));
}
await compile("src/lib/api/errors.ts", "errors.mjs");
await writeFile(
  path.join(scratch, "blobs.mjs"),
  "export const privateBlobUrls = { clear() {} };",
);
await compile(
  "src/features/auth/application/auth-lifecycle.ts",
  "auth-lifecycle.mjs",
);
const { AuthFacade } = await compile(
  "src/features/auth/application/auth-facade.ts",
  "auth.mjs",
  [
    ['import "client-only";', ""],
    ["@/lib/api/errors", "./errors.mjs"],
    ["@/lib/api/private-blob-urls", "./blobs.mjs"],
    ["./auth-lifecycle", "./auth-lifecycle.mjs"],
  ],
);
const { HostListingsFacade } = await compile(
  "src/features/host-listings/application/host-listings-facade.ts",
  "host.mjs",
);
const { createHostListingsFeatureBridge } = await compile(
  "src/features/host-listings/application/host-listings-feature-bridge.ts",
  "bridge.mjs",
);
function fixture() {
  let session = {
    accountId: "550e8400-e29b-41d4-a716-446655440000",
    roles: [],
    capabilities: [],
    restrictedMode: false,
    expiresAt: new Date(Date.now() + 60_000).toISOString(),
  };
  const auth = new AuthFacade({
    login: async () => ({ ...session }),
    currentSession: async () => ({ ...session }),
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
  });
  return {
    auth,
    permissions(capabilities) {
      session = { ...session, capabilities };
    },
  };
}
const credentials = { email: "scope@example.invalid", password: "Synthetic1!" };

test("feature scope changes on a new cookie session and on permission changes", async () => {
  const { auth, permissions } = fixture();
  await auth.login(credentials);
  const first = auth.getAccessToken();
  assert.match(first, /^bff-session:/);
  await auth.logout();
  assert.equal(auth.getAccessToken(), null);
  await auth.login(credentials);
  const second = auth.getAccessToken();
  assert.notEqual(first, second);
  permissions(["space.read"]);
  await auth.checkSession();
  assert.notEqual(auth.getAccessToken(), second);
  await auth.logout();
});

test("a stale feature scope cannot dispatch against a newly restored cookie", async () => {
  const { auth } = fixture();
  await auth.login(credentials);
  const old = auth.getAccessToken();
  let requests = 0;
  const core = new HostListingsFacade(
    {
      listListings: async () => {
        requests++;
        return [];
      },
    },
    auth,
  );
  const host = createHostListingsFeatureBridge(core, auth);
  await auth.logout();
  await auth.login(credentials);
  await assert.rejects(host.loadListings(old), { name: "AbortError" });
  assert.equal(requests, 0);
  await host.loadListings(auth.getAccessToken());
  assert.equal(requests, 1);
  await auth.logout();
});

test("cookie-backed feature operations reject late results after logout", async () => {
  const { auth } = fixture();
  await auth.login(credentials);
  let finish;
  let signal;
  const pending = auth.runAuthenticatedOperation((scope, currentSignal) => {
    assert.equal(scope, auth.getAccessToken());
    signal = currentSignal;
    return new Promise((resolve) => {
      finish = resolve;
    });
  });
  await auth.logout();
  assert.equal(signal.aborted, true);
  finish("private result");
  await assert.rejects(pending, { name: "AbortError" });
});

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

for (const operation of ["publishListing", "archiveListing"]) {
  for (const explicit of [undefined, '"original-intent"']) {
    test(`${operation} feature bridge preserves ${explicit ? "explicit" : "selected"} CAS revision`, async () => {
      const { auth } = fixture();
      await auth.login(credentials);
      const calls = [];
      const id = "550e8400-e29b-41d4-a716-446655440000";
      const core = new HostListingsFacade({
        getListing: async () => ({ id, etag: '"selected-current"' }),
        getPricing: async () => null,
        getWeeklySchedule: async () => null,
        listPhotos: async () => [],
        [operation]: async (...args) => {
          calls.push(args);
          return { id, etag: '"next"' };
        },
      });
      const bridge = createHostListingsFeatureBridge(core, auth);
      await bridge.selectListing(auth.getAccessToken(), id);
      await bridge[operation](auth.getAccessToken(), id, explicit);
      assert.equal(calls.length, 1);
      assert.equal(calls[0][0], id);
      assert.equal(calls[0][1], explicit ?? '"selected-current"');
      assert.ok(calls[0][2] instanceof AbortSignal);
    });
  }
}
