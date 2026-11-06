import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import test from "node:test";
import ts from "typescript";

async function withQueue(run) {
  const directory = await mkdtemp(path.join(os.tmpdir(), "gallery-queue-"));
  try {
    const source = await readFile(
      new URL(
        "../src/features/host-listings/application/gallery-upload-queue.ts",
        import.meta.url,
      ),
      "utf8",
    );
    const file = path.join(directory, "queue.mjs");
    await writeFile(
      file,
      ts.transpileModule(source, {
        compilerOptions: {
          target: ts.ScriptTarget.ES2022,
          module: ts.ModuleKind.ESNext,
        },
      }).outputText,
    );
    const { GalleryUploadQueue } = await import(pathToFileURL(file));
    await run(GalleryUploadQueue);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
}
function deferred() {
  let resolve, reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}
const task = (id) => ({ id, file: { name: `${id}.jpg` }, key: `key-${id}` });
const tick = () => new Promise((resolve) => setTimeout(resolve, 0));
function callbacks(overrides = {}) {
  return {
    upload: async (item) => ({ id: `photo-${item.id}` }),
    lookup: async () => ({ state: "not_found" }),
    confirm: async () => true,
    changed: () => {},
    ...overrides,
  };
}

test("a server-confirmed failed upload can be explicitly discarded without replay", async () => {
  await withQueue(async (Queue) => {
    let posts = 0;
    const queue = new Queue(
      callbacks({
        upload: async (item) => {
          posts++;
          if (item.id === "bad") throw Error("rejected");
          return { id: "good-photo" };
        },
        lookup: async () => ({ state: "failed" }),
      }),
    );
    queue.enqueue([task("bad")]);
    await tick();
    await tick();
    assert.equal(queue.getSnapshot()[0].discardAllowed, true);
    queue.discard("bad");
    assert.equal(queue.getSnapshot().length, 0);
    assert.equal(posts, 1);
    queue.enqueue([task("good")]);
    await tick();
    await tick();
    assert.equal(queue.getSnapshot()[0].state, "ready");
    assert.equal(posts, 2);
    queue.dispose();
  });
});

test("pending or not_found results cannot be discarded as confirmed failures", async () => {
  for (const state of ["pending", "not_found", "deleting"]) {
    await withQueue(async (Queue) => {
      let posts = 0;
      const queue = new Queue(
        callbacks({
          upload: async () => {
            posts++;
            throw Error("uncertain");
          },
          lookup: async () => ({ state }),
        }),
      );
      queue.enqueue([task("unknown")]);
      await tick();
      await tick();
      assert.equal(queue.getSnapshot()[0].discardAllowed, false);
      queue.discard("unknown");
      assert.equal(queue.getSnapshot().length, 1);
      assert.equal(posts, 1);
      queue.dispose();
    });
  }
});

test("rechecking a failed upload invalidates its former discard authority", async () => {
  await withQueue(async (Queue) => {
    const query = deferred();
    let reads = 0;
    const queue = new Queue(
      callbacks({
        upload: async () => {
          throw Error("uncertain");
        },
        lookup: () =>
          ++reads === 1 ? Promise.resolve({ state: "failed" }) : query.promise,
      }),
    );
    queue.enqueue([task("recheck")]);
    await tick();
    await tick();
    assert.equal(queue.getSnapshot()[0].discardAllowed, true);
    const checking = queue.check("recheck");
    queue.discard("recheck");
    assert.equal(queue.getSnapshot().length, 1);
    assert.equal(queue.getSnapshot()[0].discardAllowed, false);
    query.resolve({ state: "pending" });
    await checking;
    queue.discard("recheck");
    assert.equal(queue.getSnapshot().length, 1);
    queue.dispose();
  });
});

test("limits uploads to two and starts the third only after a slot is released", async () => {
  await withQueue(async (Queue) => {
    const calls = [];
    let active = 0;
    let maximum = 0;
    const queue = new Queue(
      callbacks({
        upload: (item) => {
          const request = deferred();
          calls.push({ id: item.id, request });
          active++;
          maximum = Math.max(maximum, active);
          return request.promise.finally(() => active--);
        },
      }),
    );
    queue.enqueue([task("a"), task("b"), task("c")]);
    await tick();
    assert.equal(calls.length, 2);
    calls[0].request.resolve({ id: "pa" });
    await tick();
    await tick();
    assert.equal(calls.length, 3);
    assert.equal(maximum, 2);
    calls[1].request.resolve({ id: "pb" });
    calls[2].request.resolve({ id: "pc" });
    await tick();
    queue.dispose();
  });
});

test("queued cancellation dispatches no upload", async () => {
  await withQueue(async (Queue) => {
    let posts = 0;
    const queue = new Queue(
      callbacks({
        upload: async () => {
          posts++;
          return { id: "p" };
        },
      }),
    );
    queue.enqueue([task("cancel-me")]);
    queue.cancel("cancel-me");
    await tick();
    assert.equal(posts, 0);
    assert.equal(queue.getSnapshot()[0].state, "cancelled");
    queue.dispose();
  });
});

test("failed lookup offers explicit retry with the same file and key", async () => {
  await withQueue(async (Queue) => {
    const original = task("same");
    const seen = [];
    let lookups = 0;
    const queue = new Queue(
      callbacks({
        upload: async (item) => {
          seen.push(item);
          if (seen.length === 1) throw Error("uncertain");
          return { id: "photo" };
        },
        lookup: async () => {
          lookups++;
          return { state: "not_found" };
        },
      }),
    );
    queue.enqueue([original]);
    await tick();
    await tick();
    assert.equal(queue.getSnapshot()[0].retryAllowed, true);
    assert.equal(lookups, 1);
    queue.retry("same");
    await tick();
    await tick();
    assert.equal(seen.length, 2);
    assert.equal(seen[1].file, original.file);
    assert.equal(seen[1].key, original.key);
    assert.equal(queue.getSnapshot()[0].state, "ready");
    queue.dispose();
  });
});

test("pending and deleting stay pending without replay; ready requires confirm", async () => {
  await withQueue(async (Queue) => {
    let posts = 0;
    let confirmed = 0;
    const queue = new Queue(
      callbacks({
        upload: async () => {
          posts++;
          return { id: "photo", state: "pending" };
        },
        lookup: async () => ({ state: "deleting" }),
        confirm: async () => {
          confirmed++;
          return true;
        },
      }),
    );
    queue.enqueue([task("pending")]);
    await tick();
    assert.equal(queue.getSnapshot()[0].state, "pending");
    await queue.check("pending");
    assert.equal(queue.getSnapshot()[0].state, "pending");
    assert.equal(posts, 1);
    assert.equal(confirmed, 0);
    queue.dispose();
    const ready = new Queue(
      callbacks({
        upload: async () => ({ id: "p" }),
        confirm: async () => false,
      }),
    );
    ready.enqueue([task("verify")]);
    await tick();
    assert.equal(ready.getSnapshot()[0].state, "pending");
    ready.dispose();
  });
});

test("in-flight cancel aborts then performs one lookup after upload settles", async () => {
  await withQueue(async (Queue) => {
    const request = deferred();
    let signal;
    let lookups = 0;
    const queue = new Queue(
      callbacks({
        upload: (_item, abortSignal) => {
          signal = abortSignal;
          return request.promise;
        },
        lookup: async () => {
          lookups++;
          return { state: "pending" };
        },
      }),
    );
    queue.enqueue([task("abort")]);
    await tick();
    queue.cancel("abort");
    assert.equal(signal.aborted, true);
    assert.equal(lookups, 0);
    request.reject(Error("aborted"));
    await tick();
    await tick();
    assert.equal(lookups, 1);
    assert.equal(queue.getSnapshot()[0].state, "pending");
    queue.dispose();
  });
});

test("dispose aborts and suppresses late readback, confirm, and change callbacks", async () => {
  await withQueue(async (Queue) => {
    const request = deferred();
    let lookups = 0;
    let changes = 0;
    let signal;
    const queue = new Queue(
      callbacks({
        upload: (_item, s) => {
          signal = s;
          return request.promise;
        },
        lookup: () => {
          lookups++;
          return new Promise(() => {});
        },
        changed: () => {
          changes++;
        },
      }),
    );
    queue.enqueue([task("dispose")]);
    await tick();
    const priorChanges = changes;
    queue.dispose();
    assert.equal(signal.aborted, true);
    request.reject(Error("late"));
    await tick();
    await tick();
    assert.equal(lookups, 0);
    assert.equal(changes, priorChanges);
    assert.deepEqual(queue.getSnapshot(), []);
  });
});

test("a stale manual lookup cannot replace a newer item state", async () => {
  await withQueue(async (Queue) => {
    const lookup = deferred();
    const queue = new Queue(
      callbacks({
        upload: async () => ({ id: "photo", state: "pending" }),
        lookup: () => lookup.promise,
      }),
    );
    queue.enqueue([task("late")]);
    await tick();
    const check = queue.check("late");
    queue.cancel("late");
    lookup.resolve({ state: "ready", photo_id: "photo" });
    await check;
    assert.equal(queue.getSnapshot()[0].state, "pending");
    queue.dispose();
  });
});

test("manual checks deduplicate, survive failure and reconcile a prior not_found", async () => {
  await withQueue(async (Queue) => {
    const lookup = deferred();
    let calls = 0;
    let result = lookup.promise;
    const queue = new Queue(
      callbacks({
        upload: async () => ({ id: "photo", state: "pending" }),
        lookup: () => {
          calls++;
          return result;
        },
      }),
    );
    queue.enqueue([task("manual")]);
    await tick();
    const first = queue.check("manual");
    const second = queue.check("manual");
    assert.equal(calls, 1);
    lookup.reject(new Error("temporary lookup failure"));
    await Promise.all([first, second]);
    assert.equal(queue.getSnapshot()[0].state, "pending");
    result = Promise.resolve({ state: "not_found" });
    await queue.check("manual");
    assert.equal(queue.getSnapshot()[0].retryAllowed, true);
    result = Promise.resolve({ state: "ready", photo_id: "photo" });
    await queue.check("manual");
    assert.equal(queue.getSnapshot()[0].state, "ready");
    queue.dispose();
  });
});
