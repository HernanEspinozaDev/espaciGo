# Local pdfcpu security fork

- Upstream module: `github.com/pdfcpu/pdfcpu` v0.16.3, pinned by the root `go.mod` before this fork.
- License: Apache-2.0; upstream `LICENSE.txt` is retained.
- Scope: the package tree required by EspaciGo's current dependency graph, with non-test source and embedded resources. Upstream tests and `testdata` are omitted from this partial snapshot; add focused regression tests for every local change.
- Purpose: apply a request-scoped, aggregate decoded-byte budget and cancellation checks at the actual filter materialization boundary used by `api.ReadContext` and subsequent validation. A raw-PDF token estimate is not a substitute for this enforcement.
- Local image-limit extension: enforce configured `MaxImagePixels` for image XObjects, page inline images, and inline images inside Form XObjects during inspection; pdfcpu's generic validation path does not cover all of these content paths by itself.
- Maintenance: compare changes against the pinned upstream v0.16.3 source, keep unrelated files unchanged, document any upstream rebase/version change here, and run the root Go tests plus focused fork-package tests before accepting updates.
