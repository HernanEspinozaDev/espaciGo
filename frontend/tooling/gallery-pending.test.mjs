import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import ts from "typescript";

// Execute the actual component's handlers. Only React's rendering machinery and
// decorative components are test doubles; decisions and requests are real code.
async function gallery(run) {
  const dir = await mkdtemp(path.join(os.tmpdir(), "gallery-component-"));
  try {
    const source = await readFile(
      new URL(
        "../src/features/host-listings/ui/listing-gallery.tsx",
        import.meta.url,
      ),
      "utf8",
    );
    const parsed = ts.createSourceFile(
      "gallery.tsx",
      source,
      ts.ScriptTarget.Latest,
      true,
      ts.ScriptKind.TSX,
    );
    const names = [];
    let body = source;
    for (const node of [...parsed.statements].reverse()) {
      if (!ts.isImportDeclaration(node)) continue;
      if (
        node.importClause &&
        !node.importClause.isTypeOnly &&
        node.moduleSpecifier.text !== "react"
      ) {
        for (const entry of node.importClause.namedBindings?.elements ?? [])
          names.push(entry.name.text);
      }
      body = body.slice(0, node.getFullStart()) + body.slice(node.end);
    }
    const prelude =
      `const { useState, useMemo, useEffect, useRef } = globalThis.__galleryHooks; const React={createElement:(type,props,...children)=>({type,props:{...props,children}})}; const privateBlobUrls={};\n` +
      names
        .filter((n) => n !== "privateBlobUrls")
        .map((n) => `const ${n}=${JSON.stringify(n)};`)
        .join("\n");
    const values = [];
    let cursor = 0;
    let dispose;
    let changes = 0;
    globalThis.__galleryHooks = {
      useState(initial) {
        const i = cursor++;
        if (!(i in values)) values[i] = initial;
        return [
          values[i],
          (v) => {
            changes++;
            values[i] = typeof v === "function" ? v(values[i]) : v;
          },
        ];
      },
      useRef(initial) {
        const i = cursor++;
        return (values[i] ??= { current: initial });
      },
      useMemo: (fn) => fn(),
      useEffect(effect, deps) {
        if (deps.length === 2 && !dispose) dispose = effect();
      },
    };
    const file = path.join(dir, "component.mjs");
    await writeFile(
      file,
      ts.transpileModule(prelude + body, {
        compilerOptions: {
          target: ts.ScriptTarget.ES2022,
          module: ts.ModuleKind.ESNext,
          jsx: ts.JsxEmit.React,
        },
      }).outputText,
    );
    const { ListingGallery } = await import(pathToFileURL(file));
    const requests = [];
    let status = { state: "pending", photo_id: "next-photo" };
    let ready = false;
    const old = {
      id: "old-photo",
      state: "ready",
      position: 1,
      is_cover: true,
    };
    const props = {
      token: "synthetic-session",
      listingId: "synthetic-space",
      selectedListing: { etag: '"7"', cover_photo_id: old.id },
      photos: [old],
      loadPhotoVariant: async () => new Blob(),
      uploadPhoto: async (...args) => {
        requests.push(["upload", ...args]);
        return { id: "next-photo", state: "pending" };
      },
      replacePhoto: async (...args) => {
        requests.push(["replace", ...args]);
        return { id: "next-photo", state: "pending" };
      },
      getPhotoUploadStatus: async (...args) => {
        requests.push(["lookup", ...args]);
        if (status instanceof Error) throw status;
        return status;
      },
      loadPhotos: async () => {
        requests.push(["refresh"]);
        return ready ? [{ ...old, id: "next-photo" }] : [old];
      },
    };
    let tree;
    const render = () => {
      cursor = 0;
      tree = ListingGallery(props);
      return tree;
    };
    function nodes(node = tree) {
      if (Array.isArray(node)) return node.flatMap((n) => nodes(n));
      if (!node || typeof node !== "object") return [];
      return [node, ...nodes(node.props?.children)];
    }
    const text = (node) =>
      Array.isArray(node)
        ? node.map(text).join("")
        : node && typeof node === "object"
          ? text(node.props?.children)
          : typeof node === "string"
            ? node
            : "";
    const find = (label) =>
      nodes().find(
        (n) => n.props?.["aria-label"] === label || text(n) === label,
      );
    const settle = async () => {
      await new Promise((resolve) => setImmediate(resolve));
      render();
    };
    const click = async (label) => {
      const n = find(label);
      assert.ok(n, label);
      assert.ok(!n.props.disabled, `${label} enabled`);
      n.props.onClick();
      await settle();
    };
    const fileObject = new File(["synthetic"], "synthetic.png", {
      type: "image/png",
    });
    const replace = async () => {
      find("Reemplazar fotografía").props.onChange({
        currentTarget: { files: [fileObject], value: "" },
      });
      await settle();
    };
    render();
    await run({
      render,
      find,
      nodes,
      text: () => text(tree),
      click,
      settle,
      replace,
      requests,
      props,
      fileObject,
      unmount: () => dispose?.(),
      changes: () => changes,
      setStatus: (v) => {
        status = v;
      },
      setReady: () => {
        ready = true;
      },
    });
  } finally {
    delete globalThis.__galleryHooks;
    await rm(dir, { recursive: true, force: true });
  }
}

test("definitively rejected invalid bytes allow selecting a different file without an uncertain operation", async () => {
  await gallery(async (g) => {
    g.props.replacePhoto = async () => {
      throw Object.assign(new Error("synthetic rejection"), {
        status: 422,
        code: "validation_error",
      });
    };
    await g.replace();
    assert.equal(g.find("Consultar estado"), undefined);
    assert.equal(g.find("Reemplazar fotografía").props.disabled, false);
    assert.match(g.text(), /imagen fue rechazada/);
    assert.ok(
      g.nodes().some((n) => n.props?.["data-photo-id"] === "old-photo"),
    );
  });
});

test("unmounted gallery ignores a late successful replacement without lookup or refresh", async () => {
  await gallery(async (g) => {
    let finish;
    g.props.replacePhoto = () =>
      new Promise((resolve) => {
        finish = resolve;
      });
    await g.replace();
    g.unmount();
    const before = g.changes();
    finish({ id: "next-photo", state: "ready" });
    await new Promise((resolve) => setImmediate(resolve));
    assert.equal(g.changes(), before);
    assert.equal(g.requests.length, 0);
  });
});

test("pending replacement preserves the old photo and forbids replay until terminal lookup", async () => {
  await gallery(async (g) => {
    await g.replace();
    assert.ok(
      g.nodes().some((n) => n.props?.["data-photo-id"] === "old-photo"),
    );
    assert.doesNotMatch(g.text(), /Fotografía reemplazada\./);
    assert.equal(
      g.find("Reintentar con el mismo archivo").props.disabled,
      true,
    );
    assert.equal(g.find("Eliminar foto").props.disabled, true);
    assert.equal(g.find("Reemplazar fotografía").props.disabled, true);
    await g.click("Consultar estado");
    assert.equal(g.requests.filter((r) => r[0] === "replace").length, 1);
    const original = g.requests.find((r) => r[0] === "replace");
    assert.deepEqual(
      g.requests.find((r) => r[0] === "lookup"),
      ["lookup", "synthetic-session", "synthetic-space", original[5]],
    );
    g.setStatus({ state: "deleting", photo_id: "next-photo" });
    await g.click("Consultar estado");
    assert.equal(
      g.find("Reintentar con el mismo archivo").props.disabled,
      true,
    );
    g.setStatus({ state: "ready", photo_id: "next-photo" });
    await g.click("Consultar estado");
    assert.doesNotMatch(g.text(), /Fotografía reemplazada\./);
    g.setReady();
    await g.click("Consultar estado");
    assert.match(g.text(), /Fotografía reemplazada\./);
    assert.equal(g.find("Consultar estado"), undefined);
  });
});

for (const state of ["failed", "not_found"])
  test(`${state} permits only an explicit retry with original file, key, target and revision`, async () => {
    await gallery(async (g) => {
      await g.replace();
      const first = g.requests.find((r) => r[0] === "replace");
      g.setStatus({ state, photo_id: "next-photo" });
      await g.click("Consultar estado");
      assert.equal(g.requests.filter((r) => r[0] === "replace").length, 1);
      await g.click("Reintentar con el mismo archivo");
      assert.deepEqual(
        g.requests.filter((r) => r[0] === "replace"),
        [first, first],
      );
      assert.equal(first[4].image, g.fileObject);
    });
  });

test("gallery refresh queries unresolved intent and failed lookup cannot enable retry", async () => {
  await gallery(async (g) => {
    await g.replace();
    g.setStatus(new Error("synthetic lookup failure"));
    await g.click("Actualizar galería");
    assert.ok(g.requests.some((r) => r[0] === "lookup"));
    assert.ok(g.find("Consultar estado"));
    assert.equal(
      g.find("Reintentar con el mismo archivo").props.disabled,
      true,
    );
    assert.doesNotMatch(g.text(), /Fotografía reemplazada\./);
  });
});
