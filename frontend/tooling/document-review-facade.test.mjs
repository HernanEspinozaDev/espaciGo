import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import ts from "typescript";

async function harness() {
  const dir = await mkdtemp(path.join(os.tmpdir(), "review-facade-"));
  let accountId = "acct-A";
  const calls = [];
  const blobs = [];
  const originalDocument = globalThis.document;
  globalThis.document = {
    createElement: () => ({
      click: () => blobs.push("click"),
      remove: () => {},
    }),
    body: { append: () => {} },
  };
  class ApiError extends Error {
    constructor(status) {
      super("raw-secret-uuid");
      this.status = status;
    }
  }
  globalThis.__reviewTest = {
    ApiError,
    authFacade: {
      getSnapshot: () => ({ accountId }),
      runAuthenticatedOperation: (fn) =>
        fn("token", new AbortController().signal),
    },
    privateBlobUrls: {
      revoke: () => blobs.push("revoke"),
      create: () => {
        blobs.push("create");
        return { url: "blob:private" };
      },
    },
    calls,
  };
  const src = await readFile(
    new URL(
      "../src/features/document-review/application/review-facade.ts",
      import.meta.url,
    ),
    "utf8",
  );
  const parsed = ts.createSourceFile(
    "review.ts",
    src,
    ts.ScriptTarget.Latest,
    true,
  );
  let body = src;
  for (const n of [...parsed.statements].reverse())
    if (ts.isImportDeclaration(n))
      body = body.slice(0, n.getFullStart()) + body.slice(n.end);
  const file = path.join(dir, "facade.mjs");
  const prelude = `const {ApiError,authFacade,privateBlobUrls}=globalThis.__reviewTest; const revisionNumber=x=>Number(x);`;
  await writeFile(
    file,
    ts.transpileModule(prelude + body, {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ESNext,
      },
    }).outputText,
  );
  const mod = await import(pathToFileURL(file));
  const port = {
    list: (...a) => {
      calls.push(["list", ...a]);
      return Promise.resolve([]);
    },
    get: (...a) => {
      calls.push(["get", ...a]);
      return Promise.resolve({
        id: a[1],
        state: "submitted",
        revision: 1,
        files: [{ id: "f", state: "limpio" }],
      });
    },
    download: (...a) => {
      calls.push(["download", ...a]);
      return Promise.resolve(new Blob(["x"]));
    },
    review: (...a) => {
      calls.push(["review", ...a]);
      return Promise.resolve({});
    },
    revoke: (...a) => {
      calls.push(["revoke", ...a]);
      return Promise.resolve({});
    },
  };
  return {
    facade: new mod.ReviewFacade(port),
    port,
    calls,
    blobs,
    setAccount: (x) => (accountId = x),
    ApiError,
    close: async () => {
      globalThis.document = originalDocument;
      delete globalThis.__reviewTest;
      await rm(dir, { recursive: true, force: true });
    },
  };
}
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((a, b) => {
    resolve = a;
    reject = b;
  });
  return { promise, resolve, reject };
};

test("unmount clearing permits a new authoritative read without waiting for an obsolete write", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    f.setSession("acct-A");
    await f.select("case-A");
    const post = deferred();
    h.port.review = () => post.promise;
    const pending = f.decide({
      capability: "review_identity",
      decision: "approve",
    });
    f.clearPrivateData();
    await f.load();
    assert.equal(h.calls.filter((c) => c[0] === "list").length, 1);
    post.resolve({});
    await pending;
    assert.equal(f.getSnapshot().selected, null);
  } finally {
    await h.close();
  }
});

test("an old finally cannot release a newer account mutation", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    f.setSession("acct-A");
    await f.select("case-A");
    const old = deferred(),
      next = deferred();
    let writes = 0;
    h.port.review = () => (++writes === 1 ? old.promise : next.promise);
    const a = f.decide({ capability: "review_identity", decision: "approve" });
    h.setAccount("acct-B");
    f.setSession("acct-B");
    await f.select("case-B");
    const b = f.decide({ capability: "review_identity", decision: "approve" });
    old.resolve({});
    await a;
    assert.equal(f.getSnapshot().pending, true);
    await f.decide({ capability: "review_identity", decision: "approve" });
    assert.equal(writes, 2);
    h.port.get = () =>
      Promise.resolve({
        id: "case-B",
        state: "aprobada",
        revision: 2,
        files: [],
      });
    next.resolve({});
    await b;
    assert.equal(f.getSnapshot().pending, false);
    assert.equal(f.getSnapshot().selected.state, "aprobada");
  } finally {
    await h.close();
  }
});

test("mutation blocks competing reads and publishes only the fresh GET", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    f.setSession("acct-A");
    await f.select("case-A");
    const post = deferred(),
      read = deferred(),
      readStarted = deferred();
    h.port.review = () => post.promise;
    const pending = f.decide({
      capability: "review_identity",
      decision: "approve",
    });
    const readsBefore = h.calls.length;
    await f.load();
    await f.select("case-B");
    assert.equal(h.calls.length, readsBefore);
    assert.equal(f.getSnapshot().selected.revision, 1);
    h.port.get = () => {
      readStarted.resolve();
      return read.promise;
    };
    post.resolve({});
    await readStarted.promise;
    assert.equal(f.getSnapshot().pending, true);
    assert.equal(f.getSnapshot().selected.revision, 1);
    read.resolve({ id: "case-A", state: "aprobada", revision: 2, files: [] });
    await pending;
    assert.equal(f.getSnapshot().selected.revision, 2);
    assert.equal(f.getSnapshot().pending, false);
  } finally {
    await h.close();
  }
});

test("case change fences a late private blob even when the port ignores abort", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    f.setSession("acct-A");
    await f.select("case-A");
    const blob = deferred(),
      started = deferred();
    h.port.download = () => {
      started.resolve();
      return blob.promise;
    };
    const download = f.download("f");
    await started.promise;
    await f.select("case-B");
    blob.resolve(new Blob(["private"]));
    await download;
    assert.equal(h.blobs.includes("create"), false);
    assert.equal(f.getSnapshot().selected.id, "case-B");
  } finally {
    await h.close();
  }
});

for (const mode of ["write-error", "get-404", "get-500", "stale"]) {
  test(`unconfirmed mutation ${mode} purges assignments and requires explicit refresh`, async () => {
    const h = await harness();
    try {
      const f = h.facade;
      f.setSession("acct-A");
      await f.select("case-A");
      h.port.list = () =>
        Promise.resolve([
          {
            id: "case-A",
            state: "submitted",
            revision: 1,
            files: [],
          },
        ]);
      await f.load();
      await f.select("case-A");
      const originalGet = h.port.get;
      let posts = 0;
      if (mode === "write-error") {
        h.port.review = () => {
          posts++;
          return Promise.reject(new h.ApiError(500));
        };
      } else {
        h.port.review = () => {
          posts++;
          return Promise.resolve({});
        };
        h.port.get = () => {
          if (mode === "stale")
            return Promise.resolve({
              id: "case-A",
              state: "submitted",
              revision: 1,
              files: [],
            });
          if (mode === "get-404") return Promise.reject(new h.ApiError(404));
          if (mode === "get-500") return Promise.reject(new h.ApiError(500));
          return Promise.resolve({
            id: "case-A",
            state: "submitted",
            revision: 2,
            files: [],
          });
        };
      }
      await f.decide({ capability: "review_identity", decision: "approve" });
      assert.deepEqual(f.getSnapshot().cases, []);
      assert.equal(f.getSnapshot().selected, null);
      assert.match(f.getSnapshot().error, /Actualiza/);
      const writes = h.calls.filter(
        (c) => c[0] === "review" || c[0] === "revoke",
      ).length;
      const selectedReads = h.calls.filter(
        (c) => c[0] === "get" || c[0] === "download",
      ).length;
      await f.decide({ capability: "review_identity", decision: "approve" });
      await f.revoke({ purpose: "identity", reason_code: "solicitud_titular" });
      await f.select("case-A");
      await f.download("file-A");
      assert.equal(
        h.calls.filter((c) => c[0] === "review" || c[0] === "revoke").length,
        writes,
      );
      assert.equal(
        h.calls.filter((c) => c[0] === "get" || c[0] === "download").length,
        selectedReads,
        "purged state blocks all selected-case operations",
      );
      assert.equal(f.getSnapshot().cases.length, 0);
      h.port.list = () => Promise.reject(new h.ApiError(500));
      await f.load();
      await f.select("case-A");
      assert.equal(
        f.getSnapshot().selected,
        null,
        "failed refresh must not reopen selected actions",
      );
      assert.match(f.getSnapshot().error, /no se confirmó/);
      h.port.list = () =>
        Promise.resolve([
          { id: "case-A", state: "aprobada", revision: 2, files: [] },
        ]);
      h.port.get = originalGet;
      await f.load();
      assert.equal(
        f.getSnapshot().cases.length,
        1,
        "explicit refresh restores authoritative assignments",
      );
      assert.equal(f.getSnapshot().error, null);
      await f.select("case-A");
      assert.equal(f.getSnapshot().selected.id, "case-A");
      assert.equal(
        posts,
        1,
        "a potentially committed decision must never be replayed automatically",
      );
    } finally {
      await h.close();
    }
  });
}

test("a download cannot open after a decision starts on the same case", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    f.setSession("acct-A");
    await f.select("case-A");
    const blob = deferred(),
      started = deferred(),
      write = deferred();
    h.port.download = () => {
      started.resolve();
      return blob.promise;
    };
    const download = f.download("f");
    await started.promise;
    h.port.review = () => write.promise;
    const mutation = f.decide({
      capability: "review_identity",
      decision: "approve",
    });
    blob.resolve(new Blob(["private"]));
    await download;
    assert.equal(
      h.blobs.includes("create"),
      false,
      "stale download opened during a decision",
    );
    write.resolve({});
    await mutation;
  } finally {
    await h.close();
  }
});

test("authoritative empty assignment refresh purges the selected case", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    f.setSession("acct-A");
    await f.select("case-A");
    assert.equal(f.getSnapshot().selected.id, "case-A");
    await f.load();
    assert.equal(f.getSnapshot().selected, null);
    assert.equal(f.getSnapshot().cases.length, 0);
  } finally {
    await h.close();
  }
});

test("old-account write never rereads its case with the next account", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    f.setSession("acct-A");
    await f.select("case-A");
    const write = deferred();
    h.port.review = () => write.promise;
    const pending = f.decide({
      capability: "review_identity",
      decision: "approve",
    });
    h.setAccount("acct-B");
    f.setSession("acct-B");
    await f.select("case-B");
    const callsBefore = h.calls.length;
    write.resolve({});
    await pending;
    assert.equal(
      h.calls.length,
      callsBefore,
      "old write dispatched a GET under the new account",
    );
    assert.equal(f.getSnapshot().selected.id, "case-B");
  } finally {
    await h.close();
  }
});

test("SSR snapshot identity is stable and setSession fences permission changes", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    assert.equal(f.getServerSnapshot(), f.getServerSnapshot());
    f.setSession("acct-A", "perm1");
    await f.load();
    assert.equal(f.getSnapshot().cases.length, 0);
    f.setSession("acct-A", "perm2");
    assert.equal(f.getSnapshot().cases.length, 0);
  } finally {
    await h.close();
  }
});
test("403 clears private data without exposing raw errors or restoring it", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    f.setSession("acct-A");
    h.port.get = () => Promise.reject(new h.ApiError(403));
    await f.select("case");
    assert.equal(f.getSnapshot().selected, null);
    assert.match(f.getSnapshot().error, /No tienes acceso/);
    assert.doesNotMatch(f.getSnapshot().error, /raw-secret-uuid/);
  } finally {
    await h.close();
  }
});
test("ordinary assignment refresh failure invalidates old row actions until authoritative recovery", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    f.setSession("acct-A");
    h.port.list = async () => [
      { id: "case-A", state: "submitted", revision: 1, files: [] },
    ];
    await f.load();
    await f.select("case-A");
    h.port.list = async () => {
      throw new h.ApiError(500);
    };
    await f.load();
    assert.deepEqual(f.getSnapshot().cases, []);
    assert.equal(f.getSnapshot().selected, null);
    const calls = h.calls.length;
    await f.select("case-A");
    await f.decide({ capability: "review_identity", decision: "approve" });
    assert.equal(h.calls.length, calls);
    h.port.list = async () => [
      { id: "case-A", state: "submitted", revision: 2, files: [] },
    ];
    await f.load();
    await f.select("case-A");
    assert.equal(f.getSnapshot().selected.id, "case-A");
    assert.equal(f.getSnapshot().error, null);
  } finally {
    await h.close();
  }
});
test("late selection A cannot replace newer selection B", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    f.setSession("acct-A");
    const a = deferred(),
      b = deferred();
    h.port.get = (_t, id) => (id === "A" ? a.promise : b.promise);
    const pa = f.select("A"),
      pb = f.select("B");
    b.resolve({ id: "B", state: "submitted", revision: 2, files: [] });
    await pb;
    a.resolve({ id: "A", state: "submitted", revision: 1, files: [] });
    await pa;
    assert.equal(f.getSnapshot().selected.id, "B");
  } finally {
    await h.close();
  }
});
test("logout fences a port that ignores abort", async () => {
  const h = await harness();
  try {
    const f = h.facade;
    f.setSession("acct-A");
    const d = deferred();
    h.port.get = () => d.promise;
    const p = f.select("A");
    h.setAccount(null);
    f.setSession(null);
    d.resolve({ id: "A", state: "submitted", revision: 1, files: [] });
    await p;
    assert.equal(f.getSnapshot().selected, null);
    assert.equal(f.getSnapshot().accountId, null);
  } finally {
    await h.close();
  }
});
